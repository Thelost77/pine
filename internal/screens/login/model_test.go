package login

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
)

func newTestModel() Model {
	return New(ui.DefaultStyles())
}

// typeString sends each rune in s as a KeyRunes message.
func typeString(m Model, s string) Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// tabTo advances focus to the given field index using tab keys.
func tabTo(m Model, field int) Model {
	for m.Focused() != field {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	return m
}

// ---------- Focus navigation ----------

func TestInitialFocus(t *testing.T) {
	m := newTestModel()
	if m.Focused() != fieldServer {
		t.Errorf("expected initial focus on server field (0), got %d", m.Focused())
	}
}

func TestTabCyclesFocus(t *testing.T) {
	m := newTestModel()

	// Tab: server → username
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focused() != fieldUsername {
		t.Errorf("after 1 tab: expected focus %d, got %d", fieldUsername, m.Focused())
	}

	// Tab: username → password
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focused() != fieldPassword {
		t.Errorf("after 2 tabs: expected focus %d, got %d", fieldPassword, m.Focused())
	}

	// Tab: password → server (wraps)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focused() != fieldServer {
		t.Errorf("after 3 tabs: expected focus %d (wrap), got %d", fieldServer, m.Focused())
	}
}

func TestShiftTabCyclesBackward(t *testing.T) {
	m := newTestModel()

	// Shift+Tab: server → password (wraps backward)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.Focused() != fieldPassword {
		t.Errorf("after shift+tab: expected focus %d, got %d", fieldPassword, m.Focused())
	}
}

func TestFullTabCycleRoundTrip(t *testing.T) {
	m := newTestModel()

	// Forward: server → username → password → server
	for i := 0; i < numFields; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	if m.Focused() != fieldServer {
		t.Errorf("full tab cycle should return to server, got %d", m.Focused())
	}

	// Backward: server → password → username → server
	for i := 0; i < numFields; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	}
	if m.Focused() != fieldServer {
		t.Errorf("full shift+tab cycle should return to server, got %d", m.Focused())
	}
}

// ---------- Text input ----------

func TestTypingIntoServerField(t *testing.T) {
	m := newTestModel()
	m = typeString(m, "https://abs.example.com")

	if got := m.ServerURL(); got != "https://abs.example.com" {
		t.Errorf("server field value = %q, want %q", got, "https://abs.example.com")
	}
}

func TestTypingIntoUsernameField(t *testing.T) {
	m := newTestModel()
	m = tabTo(m, fieldUsername)
	m = typeString(m, "alice")

	if got := m.Username(); got != "alice" {
		t.Errorf("username field value = %q, want %q", got, "alice")
	}
}

func TestTypingIntoPasswordField(t *testing.T) {
	m := newTestModel()
	m = tabTo(m, fieldPassword)
	m = typeString(m, "secret123")

	if got := m.Password(); got != "secret123" {
		t.Errorf("password field value = %q, want %q", got, "secret123")
	}
}

func TestTypingIntoAllFieldsPreservesValues(t *testing.T) {
	m := newTestModel()

	// Type server URL
	m = typeString(m, "https://my.server.com")

	// Tab to username and type
	m = tabTo(m, fieldUsername)
	m = typeString(m, "bob")

	// Tab to password and type
	m = tabTo(m, fieldPassword)
	m = typeString(m, "pass456")

	// Verify all fields retained their values
	if got := m.ServerURL(); got != "https://my.server.com" {
		t.Errorf("server = %q, want %q", got, "https://my.server.com")
	}
	if got := m.Username(); got != "bob" {
		t.Errorf("username = %q, want %q", got, "bob")
	}
	if got := m.Password(); got != "pass456" {
		t.Errorf("password = %q, want %q", got, "pass456")
	}
}

// ---------- Enter key behavior ----------

func TestEnterOnPasswordFieldTriggersLogin(t *testing.T) {
	m := newTestModel()
	m = tabTo(m, fieldPassword)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Loading() {
		t.Error("expected loading=true after enter on password")
	}
	if cmd == nil {
		t.Error("expected non-nil command (loginCmd) after enter on password")
	}
}

func TestEnterOnNonPasswordAdvancesFocus(t *testing.T) {
	m := newTestModel()

	// Enter on server field should move to username
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Focused() != fieldUsername {
		t.Errorf("enter on server should advance to username, got %d", m.Focused())
	}

	// Enter on username field should move to password
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Focused() != fieldPassword {
		t.Errorf("enter on username should advance to password, got %d", m.Focused())
	}
}

// ---------- Login success/failure messages ----------

func TestLoginSuccessMsgClearsLoadingAndError(t *testing.T) {
	m := newTestModel()
	m.loading = true
	m.err = fmt.Errorf("previous error")

	m, _ = m.Update(LoginSuccessMsg{Token: "tok123", ServerURL: "http://example.com", Username: "alice"})

	if m.Loading() {
		t.Error("expected loading=false after LoginSuccessMsg")
	}
	if m.Error() != nil {
		t.Errorf("expected nil error after LoginSuccessMsg, got %v", m.Error())
	}
}

func TestLoginFailedMsgSetsError(t *testing.T) {
	m := newTestModel()
	m.loading = true

	m, _ = m.Update(LoginFailedMsg{Err: fmt.Errorf("bad creds")})

	if m.Loading() {
		t.Error("expected loading=false after LoginFailedMsg")
	}
	if m.Error() == nil || m.Error().Error() != "bad creds" {
		t.Errorf("expected error 'bad creds', got %v", m.Error())
	}
}

// ---------- Mock server login flows ----------

func TestLoginSuccessWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if creds.Username != "alice" || creds.Password != "correct" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		resp := abs.LoginResponse{User: abs.LoginUser{
			ID: "usr1", Username: "alice", Token: "jwt-token-abc",
		}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	m := newTestModel()

	// Fill in the form with mock server URL
	m = typeString(m, srv.URL)
	m = tabTo(m, fieldUsername)
	m = typeString(m, "alice")
	m = tabTo(m, fieldPassword)
	m = typeString(m, "correct")

	// Press enter to trigger login
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Loading() {
		t.Fatal("expected loading=true after enter")
	}
	if cmd == nil {
		t.Fatal("expected login command")
	}

	// Execute the command to get the message
	msg := cmd()

	successMsg, ok := msg.(LoginSuccessMsg)
	if !ok {
		t.Fatalf("expected LoginSuccessMsg, got %T: %v", msg, msg)
	}
	if successMsg.Token != "jwt-token-abc" {
		t.Errorf("token = %q, want %q", successMsg.Token, "jwt-token-abc")
	}
	if successMsg.ServerURL != srv.URL {
		t.Errorf("server URL = %q, want %q", successMsg.ServerURL, srv.URL)
	}

	// Feed the success message back into the model
	m, _ = m.Update(successMsg)
	if m.Loading() {
		t.Error("expected loading=false after success")
	}
	if m.Error() != nil {
		t.Errorf("expected no error after success, got %v", m.Error())
	}
}

func TestLoginFailureWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
	}))
	defer srv.Close()

	m := newTestModel()

	// Fill in the form
	m = typeString(m, srv.URL)
	m = tabTo(m, fieldUsername)
	m = typeString(m, "alice")
	m = tabTo(m, fieldPassword)
	m = typeString(m, "wrong")

	// Press enter
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected login command")
	}

	// Execute the command — should fail
	msg := cmd()

	failMsg, ok := msg.(LoginFailedMsg)
	if !ok {
		t.Fatalf("expected LoginFailedMsg, got %T: %v", msg, msg)
	}
	if failMsg.Err == nil {
		t.Fatal("expected non-nil error in LoginFailedMsg")
	}

	// Feed the failure message back into the model
	m, _ = m.Update(failMsg)
	if m.Loading() {
		t.Error("expected loading=false after failure")
	}
	if m.Error() == nil {
		t.Error("expected error to be set after failure")
	}
}

func TestLoginFailureWithUnreachableServer(t *testing.T) {
	m := newTestModel()

	// Use an invalid server URL
	m = typeString(m, "http://127.0.0.1:1")
	m = tabTo(m, fieldUsername)
	m = typeString(m, "user")
	m = tabTo(m, fieldPassword)
	m = typeString(m, "pass")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected login command")
	}

	msg := cmd()
	if _, ok := msg.(LoginFailedMsg); !ok {
		t.Fatalf("expected LoginFailedMsg for unreachable server, got %T", msg)
	}
}

// ---------- View rendering ----------

func TestViewRendersThreeInputFields(t *testing.T) {
	m := newTestModel()
	v := m.View()

	for _, label := range []string{"Server URL", "Username", "Password"} {
		if !strings.Contains(v, label) {
			t.Errorf("expected view to contain label %q", label)
		}
	}
}

func TestViewShowsErrorMessage(t *testing.T) {
	m := newTestModel()
	m.err = fmt.Errorf("connection refused")
	v := m.View()

	if !strings.Contains(v, "connection refused") {
		t.Error("expected view to show error message")
	}
}

func TestViewShowsLoadingState(t *testing.T) {
	m := newTestModel()
	m.loading = true
	v := m.View()

	if !strings.Contains(v, "logging in") {
		t.Error("expected view to show loading indicator")
	}
}

func TestPasswordInputIsMasked(t *testing.T) {
	m := newTestModel()
	m = tabTo(m, fieldPassword)
	m = typeString(m, "abc")

	v := m.View()
	if !strings.Contains(v, "•") {
		t.Error("expected password field to show mask character •")
	}
}

func TestViewShowsHelpText(t *testing.T) {
	m := newTestModel()
	v := m.View()

	if !strings.Contains(v, "tab") {
		t.Error("expected view to contain help text with 'tab'")
	}
	if !strings.Contains(v, "enter") {
		t.Error("expected view to contain help text with 'enter'")
	}
}

func TestViewShowsTitle(t *testing.T) {
	m := newTestModel()
	v := m.View()

	if !strings.Contains(v, "pine") {
		t.Error("expected view to contain title 'abs-cli'")
	}
}

func TestViewShowsTypedServerURL(t *testing.T) {
	m := newTestModel()
	m = typeString(m, "https://my.abs.server")
	v := m.View()

	if !strings.Contains(v, "https://my.abs.server") {
		t.Error("expected view to display typed server URL")
	}
}

func TestViewShowsTypedUsername(t *testing.T) {
	m := newTestModel()
	m = tabTo(m, fieldUsername)
	m = typeString(m, "testuser")
	v := m.View()

	if !strings.Contains(v, "testuser") {
		t.Error("expected view to display typed username")
	}
}

func TestViewUsesPlaceWhenSizeSet(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	v := m.View()

	// When size is set, view should still contain all labels
	for _, label := range []string{"Server URL", "Username", "Password"} {
		if !strings.Contains(v, label) {
			t.Errorf("expected sized view to contain label %q", label)
		}
	}
}

// ---------- SetSize ----------

func TestSetSize(t *testing.T) {
	m := newTestModel()
	m.SetSize(120, 40)

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
}
