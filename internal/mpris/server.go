package mpris

import (
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// The object path and the interface names of MPRIS 2.2.
const (
	objectPath              = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	rootInterface           = "org.mpris.MediaPlayer2"
	playerInterface         = "org.mpris.MediaPlayer2.Player"
	propertiesInterface     = "org.freedesktop.DBus.Properties"
	introspectableInterface = "org.freedesktop.DBus.Introspectable"
)

// property is one D-Bus property. The server reads the value from the app on
// each request.
type property struct {
	name      string
	signature string
	get       func() (any, error)
	// set is nil for a read-only property. The writable MPRIS properties that
	// the server has are of the type double.
	set func(float64) error
}

func readOnly[T any](name, signature string, get func() (T, error)) property {
	return property{name: name, signature: signature, get: func() (any, error) { return get() }}
}

func readWrite(name string, get func() (float64, error), set func(float64) error) property {
	p := readOnly(name, "d", get)
	p.set = set
	return p
}

// Server is an MPRIS media player on the D-Bus session bus. It has the
// org.mpris.MediaPlayer2 and org.mpris.MediaPlayer2.Player interfaces.
type Server struct {
	name       string
	root       RootAdapter
	player     PlayerAdapter
	properties map[string][]property

	mu      sync.Mutex
	conn    *dbus.Conn
	stopped bool
}

// NewServer creates an MPRIS server for the app name with the given root and player adapters.
func NewServer(name string, root RootAdapter, player PlayerAdapter) *Server {
	status := func() (string, error) {
		status, err := player.PlaybackStatus()
		return string(status), err
	}
	metadata := func() (map[string]dbus.Variant, error) {
		metadata, err := player.Metadata()
		return metadata.variants(), err
	}
	return &Server{
		name:   name,
		root:   root,
		player: player,
		properties: map[string][]property{
			rootInterface: {
				readOnly("CanQuit", "b", root.CanQuit),
				readOnly("CanRaise", "b", root.CanRaise),
				readOnly("HasTrackList", "b", root.HasTrackList),
				readOnly("Identity", "s", root.Identity),
				readOnly("DesktopEntry", "s", root.DesktopEntry),
				readOnly("SupportedUriSchemes", "as", root.SupportedUriSchemes),
				readOnly("SupportedMimeTypes", "as", root.SupportedMimeTypes),
			},
			playerInterface: {
				readOnly("PlaybackStatus", "s", status),
				readWrite("Rate", player.Rate, player.SetRate),
				readOnly("Metadata", "a{sv}", metadata),
				readWrite("Volume", player.Volume, player.SetVolume),
				readOnly("Position", "x", player.Position),
				readOnly("MinimumRate", "d", player.MinimumRate),
				readOnly("MaximumRate", "d", player.MaximumRate),
				readOnly("CanGoNext", "b", player.CanGoNext),
				readOnly("CanGoPrevious", "b", player.CanGoPrevious),
				readOnly("CanPlay", "b", player.CanPlay),
				readOnly("CanPause", "b", player.CanPause),
				readOnly("CanSeek", "b", player.CanSeek),
				readOnly("CanControl", "b", player.CanControl),
			},
		},
	}
}

// variants returns the metadata as an MPRIS metadata map. An empty field is
// not in the map.
func (m Metadata) variants() map[string]dbus.Variant {
	values := map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(m.TrackID)}
	if m.Length > 0 {
		values["mpris:length"] = dbus.MakeVariant(m.Length.Microseconds())
	}
	if m.Title != "" {
		values["xesam:title"] = dbus.MakeVariant(m.Title)
	}
	if len(m.Artist) > 0 {
		values["xesam:artist"] = dbus.MakeVariant(m.Artist)
	}
	return values
}

// Listen connects to the session bus and answers MPRIS requests. It blocks
// until Stop is called or the connection closes.
func (s *Server) Listen() error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	if err := s.export(conn); err != nil {
		_ = conn.Close()
		return err
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return conn.Close()
	}
	s.conn = conn
	s.mu.Unlock()
	<-conn.Context().Done()
	return nil
}

// Stop closes the connection to the session bus. The bus then releases the name.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.conn == nil {
		return nil
	}
	conn := s.conn
	s.conn = nil
	return conn.Close()
}

// export puts the MPRIS object on the connection and then takes the bus name.
func (s *Server) export(conn *dbus.Conn) error {
	player := s.player
	call := func(method func() error) func() *dbus.Error {
		return func() *dbus.Error { return failed(method()) }
	}
	for iface, methods := range map[string]map[string]any{
		rootInterface: {
			"Raise": call(s.root.Raise),
			"Quit":  call(s.root.Quit),
		},
		playerInterface: {
			"Next":      call(player.Next),
			"Previous":  call(player.Previous),
			"Pause":     call(player.Pause),
			"PlayPause": call(player.PlayPause),
			"Stop":      call(player.Stop),
			"Play":      call(player.Play),
			"Seek": func(offset int64) *dbus.Error {
				return failed(player.Seek(microseconds(offset)))
			},
			"SetPosition": func(trackID dbus.ObjectPath, position int64) *dbus.Error {
				return failed(player.SetPosition(string(trackID), microseconds(position)))
			},
			"OpenUri": func(uri string) *dbus.Error { return failed(player.OpenUri(uri)) },
		},
		propertiesInterface: {
			"Get":    s.get,
			"GetAll": s.getAll,
			"Set":    s.set,
		},
		introspectableInterface: {
			"Introspect": introspect.NewIntrospectable(s.node()).Introspect,
		},
	} {
		if err := conn.ExportMethodTable(methods, objectPath, iface); err != nil {
			return err
		}
	}

	busName := "org.mpris.MediaPlayer2." + s.name
	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("the D-Bus name %s is not available", busName)
	}
	return nil
}

func microseconds(value int64) time.Duration {
	return time.Duration(value) * time.Microsecond
}

func failed(err error) *dbus.Error {
	if err == nil {
		return nil
	}
	return dbus.MakeFailedError(err)
}

func (s *Server) find(iface, name string) (property, *dbus.Error) {
	properties, ok := s.properties[iface]
	if !ok {
		return property{}, prop.ErrIfaceNotFound
	}
	for _, p := range properties {
		if p.name == name {
			return p, nil
		}
	}
	return property{}, prop.ErrPropNotFound
}

func (s *Server) get(iface, name string) (dbus.Variant, *dbus.Error) {
	p, dbusErr := s.find(iface, name)
	if dbusErr != nil {
		return dbus.Variant{}, dbusErr
	}
	value, err := p.get()
	if err != nil {
		return dbus.Variant{}, dbus.MakeFailedError(err)
	}
	return dbus.MakeVariant(value), nil
}

func (s *Server) getAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	properties, ok := s.properties[iface]
	if !ok {
		return nil, prop.ErrIfaceNotFound
	}
	values := make(map[string]dbus.Variant, len(properties))
	for _, p := range properties {
		value, err := p.get()
		if err != nil {
			return nil, dbus.MakeFailedError(err)
		}
		values[p.name] = dbus.MakeVariant(value)
	}
	return values, nil
}

func (s *Server) set(iface, name string, value dbus.Variant) *dbus.Error {
	p, dbusErr := s.find(iface, name)
	if dbusErr != nil {
		return dbusErr
	}
	if p.set == nil {
		return prop.ErrReadOnly
	}
	number, ok := value.Value().(float64)
	if !ok {
		return prop.ErrInvalidArg
	}
	return failed(p.set(number))
}

// node returns the introspection data of the MPRIS object.
func (s *Server) node() *introspect.Node {
	in := func(name, signature string) introspect.Arg {
		return introspect.Arg{Name: name, Type: signature, Direction: "in"}
	}
	properties := func(iface string) []introspect.Property {
		var result []introspect.Property
		for _, p := range s.properties[iface] {
			access := "read"
			if p.set != nil {
				access = "readwrite"
			}
			result = append(result, introspect.Property{Name: p.name, Type: p.signature, Access: access})
		}
		return result
	}
	return &introspect.Node{
		Name: string(objectPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:       rootInterface,
				Methods:    []introspect.Method{{Name: "Raise"}, {Name: "Quit"}},
				Properties: properties(rootInterface),
			},
			{
				Name: playerInterface,
				Methods: []introspect.Method{
					{Name: "Next"},
					{Name: "Previous"},
					{Name: "Pause"},
					{Name: "PlayPause"},
					{Name: "Stop"},
					{Name: "Play"},
					{Name: "Seek", Args: []introspect.Arg{in("Offset", "x")}},
					{Name: "SetPosition", Args: []introspect.Arg{in("TrackId", "o"), in("Position", "x")}},
					{Name: "OpenUri", Args: []introspect.Arg{in("Uri", "s")}},
				},
				Signals: []introspect.Signal{
					{Name: "Seeked", Args: []introspect.Arg{{Name: "Position", Type: "x"}}},
				},
				Properties: properties(playerInterface),
			},
		},
	}
}

func (s *Server) connection() *dbus.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
}

// changed sends the current values of the given player properties to the
// MPRIS clients.
func (s *Server) changed(names ...string) {
	conn := s.connection()
	if conn == nil {
		return
	}
	values := make(map[string]dbus.Variant, len(names))
	for _, name := range names {
		if value, err := s.get(playerInterface, name); err == nil {
			values[name] = value
		}
	}
	_ = conn.Emit(objectPath, propertiesInterface+".PropertiesChanged", playerInterface, values, []string{})
}

// PlaybackChanged tells MPRIS clients that the playback state, the rate, or
// the track changed.
func (s *Server) PlaybackChanged() {
	s.changed("PlaybackStatus", "Rate", "Metadata", "MinimumRate", "MaximumRate")
}

// StatusChanged tells MPRIS clients that the playback status changed.
func (s *Server) StatusChanged() { s.changed("PlaybackStatus") }

// MetadataChanged tells MPRIS clients that the track metadata changed.
func (s *Server) MetadataChanged() { s.changed("Metadata") }

// VolumeChanged tells MPRIS clients that the volume changed.
func (s *Server) VolumeChanged() { s.changed("Volume") }

// Seeked tells MPRIS clients the playback position, in seconds.
func (s *Server) Seeked(positionSeconds float64) {
	conn := s.connection()
	if conn == nil {
		return
	}
	_ = conn.Emit(objectPath, playerInterface+".Seeked", int64(positionSeconds*1_000_000))
}
