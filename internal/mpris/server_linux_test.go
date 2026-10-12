//go:build linux

package mpris

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	testBusName     = "org.mpris.MediaPlayer2.pinetest"
	testObjectPath  = "/org/mpris/MediaPlayer2"
	testRootIface   = "org.mpris.MediaPlayer2"
	testPlayerIface = "org.mpris.MediaPlayer2.Player"
	testPropsIface  = "org.freedesktop.DBus.Properties"
)

// startSessionBus starts a private D-Bus session bus and returns its address.
// Without dbus-daemon the test skips. With PINE_REQUIRE_DBUS=1 it fails, so
// that a release cannot skip this test.
func startSessionBus(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("dbus-daemon")
	if err != nil {
		if os.Getenv("PINE_REQUIRE_DBUS") == "1" {
			t.Fatalf("dbus-daemon is required: %v", err)
		}
		t.Skipf("dbus-daemon is not available: %v", err)
	}
	cmd := exec.Command(path, "--session", "--nofork", "--print-address")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start dbus-daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	address, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read the address of dbus-daemon: %v", err)
	}
	return strings.TrimSpace(address)
}

// busCall calls a D-Bus method. It fails the test if the server does not
// answer.
func busCall(t *testing.T, obj dbus.BusObject, method string, args ...any) *dbus.Call {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	call := obj.CallWithContext(ctx, method, 0, args...)
	if ctx.Err() != nil {
		t.Fatalf("%s: no answer from the server", method)
	}
	return call
}

func errorName(err error) string {
	if dbusErr, ok := err.(dbus.Error); ok {
		return dbusErr.Name
	}
	return ""
}

// TestServerOnSessionBus checks the server as an MPRIS client sees it.
func TestServerOnSessionBus(t *testing.T) {
	address := startSessionBus(t)
	// The server must not connect to the session bus of the desktop.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", address)

	state := &mockAccessor{
		playing:  true,
		hasItem:  true,
		title:    "The Hobbit",
		authors:  []string{"J.R.R. Tolkien"},
		itemID:   "item-123",
		position: 42.5,
		duration: 3600,
		volume:   75,
		speed:    1.5,
		queueLen: 2,
	}
	bridge := NewBridge(nil, "pinetest")
	// With a skip of 10 seconds, Next and Previous seek and return an error.
	bridge.Bind(accessorFn(state), 10)
	server := bridge.Server()
	listenErr := make(chan error, 1)
	go func() { listenErr <- server.Listen() }()
	t.Cleanup(func() { _ = server.Stop() })

	client, err := dbus.Connect(address)
	if err != nil {
		t.Fatalf("connect to the test bus: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	deadline := time.Now().Add(5 * time.Second)
	for {
		var owned bool
		if err := busCall(t, client.BusObject(), "org.freedesktop.DBus.NameHasOwner", testBusName).Store(&owned); err != nil {
			t.Fatalf("NameHasOwner error: %v", err)
		}
		if owned {
			break
		}
		select {
		case err := <-listenErr:
			t.Fatalf("Listen() returned before the name was owned: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the server did not take the name %s", testBusName)
		}
		time.Sleep(10 * time.Millisecond)
	}
	obj := client.Object(testBusName, testObjectPath)

	getAll := func(t *testing.T, iface string) map[string]dbus.Variant {
		t.Helper()
		var props map[string]dbus.Variant
		if err := busCall(t, obj, testPropsIface+".GetAll", iface).Store(&props); err != nil {
			t.Fatalf("GetAll(%s) error: %v", iface, err)
		}
		return props
	}
	checkValues := func(t *testing.T, got map[string]dbus.Variant, want map[string]any) {
		t.Helper()
		for name, wantValue := range want {
			variant, ok := got[name]
			if !ok {
				t.Errorf("%s is absent", name)
				continue
			}
			if !reflect.DeepEqual(variant.Value(), wantValue) {
				t.Errorf("%s = %#v (%s); want %#v", name, variant.Value(), variant.Signature(), wantValue)
			}
		}
	}
	wantMetadata := map[string]any{
		"mpris:trackid": dbus.ObjectPath("/org/pinetest/track/item_123"),
		"mpris:length":  int64(3_600_000_000),
		"xesam:title":   "The Hobbit",
		"xesam:artist":  []string{"J.R.R. Tolkien"},
	}
	wantRoot := map[string]any{
		"CanQuit":             true,
		"CanRaise":            false,
		"HasTrackList":        false,
		"Identity":            "pinetest",
		"DesktopEntry":        "pinetest",
		"SupportedUriSchemes": []string{},
		"SupportedMimeTypes":  []string{},
	}
	wantPlayer := map[string]any{
		"PlaybackStatus": "Playing",
		"Rate":           1.5,
		"Volume":         0.75,
		"Position":       int64(42_500_000),
		"MinimumRate":    0.5,
		"MaximumRate":    4.0,
		"CanGoNext":      true,
		"CanGoPrevious":  true,
		"CanPlay":        true,
		"CanPause":       true,
		"CanSeek":        true,
		"CanControl":     true,
	}

	t.Run("properties", func(t *testing.T) {
		root := getAll(t, testRootIface)
		checkValues(t, root, wantRoot)
		if len(root) != len(wantRoot) {
			t.Errorf("root interface has %d properties; want %d", len(root), len(wantRoot))
		}

		player := getAll(t, testPlayerIface)
		checkValues(t, player, wantPlayer)
		// Metadata is the one property that is not in wantPlayer.
		if len(player) != len(wantPlayer)+1 {
			t.Errorf("player interface has %d properties; want %d", len(player), len(wantPlayer)+1)
		}
		metadata, ok := player["Metadata"].Value().(map[string]dbus.Variant)
		if !ok {
			t.Fatalf("Metadata has signature %s; want a{sv}", player["Metadata"].Signature())
		}
		checkValues(t, metadata, wantMetadata)

		var position dbus.Variant
		if err := busCall(t, obj, testPropsIface+".Get", testPlayerIface, "Position").Store(&position); err != nil {
			t.Fatalf("Get(Position) error: %v", err)
		}
		if position.Value() != int64(42_500_000) {
			t.Errorf("Get(Position) = %#v; want int64(42500000)", position.Value())
		}
	})

	t.Run("property errors", func(t *testing.T) {
		err := busCall(t, obj, testPropsIface+".Get", testPlayerIface, "NoSuchProperty").Err
		if got, want := errorName(err), "org.freedesktop.DBus.Properties.Error.PropertyNotFound"; got != want {
			t.Errorf("Get(NoSuchProperty) error = %q (%v); want %q", got, err, want)
		}
		err = busCall(t, obj, testPropsIface+".GetAll", "org.example.NoSuchInterface").Err
		if got, want := errorName(err), "org.freedesktop.DBus.Properties.Error.InterfaceNotFound"; got != want {
			t.Errorf("GetAll(NoSuchInterface) error = %q (%v); want %q", got, err, want)
		}
		err = busCall(t, obj, testPropsIface+".Set", testPlayerIface, "Position", dbus.MakeVariant(int64(1))).Err
		if err == nil {
			t.Error("Set(Position) returned no error; Position is read-only")
		}
	})

	t.Run("introspection", func(t *testing.T) {
		node, err := introspect.Call(obj)
		if err != nil {
			t.Fatalf("Introspect error: %v", err)
		}
		interfaces := make(map[string]introspect.Interface)
		for _, iface := range node.Interfaces {
			interfaces[iface.Name] = iface
		}
		if _, ok := interfaces[testPropsIface]; !ok {
			t.Errorf("introspection data does not contain %s", testPropsIface)
		}
		// The type of each property in the introspection data must be the
		// type that the server sends.
		for _, name := range []string{testRootIface, testPlayerIface} {
			declared := make(map[string]string)
			for _, property := range interfaces[name].Properties {
				declared[property.Name] = property.Type
			}
			for property, variant := range getAll(t, name) {
				if got, want := declared[property], variant.Signature().String(); got != want {
					t.Errorf("%s.%s: introspection type = %q; the server sends %q", name, property, got, want)
				}
			}
		}
		methods := make(map[string]bool)
		for _, method := range interfaces[testPlayerIface].Methods {
			methods[method.Name] = true
		}
		for _, name := range []string{"Next", "Previous", "Pause", "PlayPause", "Stop", "Play", "Seek", "SetPosition", "OpenUri"} {
			if !methods[name] {
				t.Errorf("introspection data does not contain the method %s", name)
			}
		}
		if signals := interfaces[testPlayerIface].Signals; len(signals) != 1 || signals[0].Name != "Seeked" {
			t.Errorf("signals of the player interface = %v; want Seeked", signals)
		}
	})

	// wantMessage checks the message that a call must send to the app.
	wantMessage := func(t *testing.T, name string, want any) {
		t.Helper()
		select {
		case got := <-bridge.msgCh:
			if got != want {
				t.Errorf("%s sent %#v; want %#v", name, got, want)
			}
		default:
			t.Errorf("%s sent no message; want %#v", name, want)
		}
	}

	t.Run("methods", func(t *testing.T) {
		for _, tt := range []struct {
			method    string
			args      []any
			want      any
			wantError bool
		}{
			{method: "Play", want: PlayMsg{}},
			{method: "Pause", want: PauseMsg{}},
			{method: "PlayPause", want: PlayPauseMsg{}},
			{method: "Stop", want: PauseMsg{}},
			{method: "Seek", args: []any{int64(5_000_000)}, want: SeekRelativeMsg{OffsetSeconds: 5}},
			{
				method: "SetPosition",
				args:   []any{dbus.ObjectPath("/org/pinetest/track/item_123"), int64(30_000_000)},
				want:   SeekAbsoluteMsg{PositionSeconds: 30},
			},
			// The error stops the desktop wait for a track change.
			{method: "Next", want: SeekRelativeMsg{OffsetSeconds: 10}, wantError: true},
			{method: "Previous", want: SeekRelativeMsg{OffsetSeconds: -10}, wantError: true},
		} {
			err := busCall(t, obj, testPlayerIface+"."+tt.method, tt.args...).Err
			if tt.wantError {
				if got, want := errorName(err), "org.freedesktop.DBus.Error.Failed"; got != want {
					t.Errorf("%s error = %q (%v); want %q", tt.method, got, err, want)
				}
			} else if err != nil {
				t.Errorf("%s error: %v", tt.method, err)
			}
			wantMessage(t, tt.method, tt.want)
		}
		for _, method := range []string{testPlayerIface + ".OpenUri", testRootIface + ".Raise", testRootIface + ".Quit"} {
			var args []any
			if strings.HasSuffix(method, ".OpenUri") {
				args = []any{"file:///book.m4b"}
			}
			if err := busCall(t, obj, method, args...).Err; err != nil {
				t.Errorf("%s error: %v", method, err)
			}
		}
	})

	t.Run("set properties", func(t *testing.T) {
		if err := busCall(t, obj, testPropsIface+".Set", testPlayerIface, "Volume", dbus.MakeVariant(0.5)).Err; err != nil {
			t.Errorf("Set(Volume) error: %v", err)
		}
		wantMessage(t, "Set(Volume)", SetVolumeMsg{Volume: 50})
		if err := busCall(t, obj, testPropsIface+".Set", testPlayerIface, "Rate", dbus.MakeVariant(2.0)).Err; err != nil {
			t.Errorf("Set(Rate) error: %v", err)
		}
		wantMessage(t, "Set(Rate)", SetRateMsg{Rate: 2})

		// A value of a wrong type must give an error. It must not reach the app.
		err := busCall(t, obj, testPropsIface+".Set", testPlayerIface, "Volume", dbus.MakeVariant("loud")).Err
		if got, want := errorName(err), "org.freedesktop.DBus.Properties.Error.InvalidArg"; got != want {
			t.Errorf("Set(Volume) with a string: error = %q (%v); want %q", got, err, want)
		}
		select {
		case got := <-bridge.msgCh:
			t.Errorf("Set(Volume) with a string sent %#v; want no message", got)
		default:
		}
	})

	t.Run("signals", func(t *testing.T) {
		if err := client.AddMatchSignal(dbus.WithMatchObjectPath(testObjectPath)); err != nil {
			t.Fatalf("AddMatchSignal error: %v", err)
		}
		signals := make(chan *dbus.Signal, 16)
		client.Signal(signals)
		// waitSignal returns the body of the next signal with the given name.
		waitSignal := func(t *testing.T, name string) []any {
			t.Helper()
			timeout := time.After(5 * time.Second)
			for {
				select {
				case signal := <-signals:
					if signal.Name == name {
						return signal.Body
					}
				case <-timeout:
					t.Fatalf("no %s signal", name)
				}
			}
		}
		// waitChanged returns the properties of the next PropertiesChanged
		// signal of the player interface.
		waitChanged := func(t *testing.T) map[string]dbus.Variant {
			t.Helper()
			body := waitSignal(t, testPropsIface+".PropertiesChanged")
			if len(body) != 3 || body[0] != testPlayerIface {
				t.Fatalf("PropertiesChanged body = %v; want the player interface and two more values", body)
			}
			changed, ok := body[1].(map[string]dbus.Variant)
			if !ok {
				t.Fatalf("PropertiesChanged body[1] = %#v; want a{sv}", body[1])
			}
			return changed
		}

		server.PlaybackChanged()
		changed := waitChanged(t)
		checkValues(t, changed, map[string]any{"PlaybackStatus": "Playing", "Rate": 1.5, "MinimumRate": 0.5, "MaximumRate": 4.0})
		metadata, ok := changed["Metadata"].Value().(map[string]dbus.Variant)
		if !ok {
			t.Fatalf("PlaybackChanged sent Metadata with signature %s; want a{sv}", changed["Metadata"].Signature())
		}
		checkValues(t, metadata, wantMetadata)

		for _, tt := range []struct {
			name string
			emit func()
			want map[string]any
		}{
			{name: "StatusChanged", emit: server.StatusChanged, want: map[string]any{"PlaybackStatus": "Playing"}},
			{name: "VolumeChanged", emit: server.VolumeChanged, want: map[string]any{"Volume": 0.75}},
		} {
			tt.emit()
			changed := waitChanged(t)
			checkValues(t, changed, tt.want)
			if len(changed) != len(tt.want) {
				t.Errorf("%s sent %d properties; want %d", tt.name, len(changed), len(tt.want))
			}
		}

		server.MetadataChanged()
		changed = waitChanged(t)
		metadata, ok = changed["Metadata"].Value().(map[string]dbus.Variant)
		if !ok || len(changed) != 1 {
			t.Fatalf("MetadataChanged sent %v; want only Metadata", changed)
		}
		checkValues(t, metadata, wantMetadata)

		server.Seeked(42.5)
		body := waitSignal(t, testPlayerIface+".Seeked")
		if len(body) != 1 || body[0] != int64(42_500_000) {
			t.Errorf("Seeked body = %#v; want int64(42500000)", body)
		}
	})

	if err := server.Stop(); err != nil {
		t.Logf("Stop() error: %v", err)
	}
	select {
	case <-listenErr:
	case <-time.After(5 * time.Second):
		t.Error("Listen() did not return after Stop()")
	}
}
