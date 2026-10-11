package login

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestLoginModel_UpdateAndLoginCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := jellyfin.AuthResponse{
			User:        jellyfin.User{ID: "usr-1", Name: "bob"},
			AccessToken: "tok-1",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	m := New(ui.DefaultStyles())
	if m.Focused() != fieldServer {
		t.Errorf("expected focus on fieldServer, got %d", m.Focused())
	}

	// Type server URL
	m.inputs[fieldServer].SetValue(server.URL)
	m.inputs[fieldUsername].SetValue("bob")
	m.inputs[fieldPassword].SetValue("pass")

	// Tab to password
	m.focused = fieldPassword

	// Trigger enter on password field
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on submit")
	}

	msg := cmd()
	success, ok := msg.(LoginSuccessMsg)
	if !ok {
		t.Fatalf("expected LoginSuccessMsg, got %T: %+v", msg, msg)
	}
	if success.Token != "tok-1" || success.UserID != "usr-1" {
		t.Errorf("unexpected success msg: %+v", success)
	}

	// Test View rendering
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}
