package mpris

import (
	"github.com/quarckster/go-mpris-server/pkg/events"
	"github.com/quarckster/go-mpris-server/pkg/server"
	"github.com/quarckster/go-mpris-server/pkg/types"
)

// Server wraps the MPRIS D-Bus server and event handler.
type Server struct {
	srv    *server.Server
	events *events.EventHandler
}

// NewServer creates an MPRIS server with the given root and player adapters.
func NewServer(root types.OrgMprisMediaPlayer2Adapter, player types.OrgMprisMediaPlayer2PlayerAdapter) *Server {
	srv := server.NewServer("pine", root, player)
	return &Server{
		srv:    srv,
		events: events.NewEventHandler(srv),
	}
}

// Listen starts the D-Bus server. Blocks until Stop is called.
func (s *Server) Listen() error {
	return s.srv.Listen()
}

// Stop releases the D-Bus name and closes the connection.
func (s *Server) Stop() error {
	return s.srv.Stop()
}

// EventHandler returns the event handler for emitting property changes.
func (s *Server) EventHandler() *events.EventHandler {
	return s.events
}
