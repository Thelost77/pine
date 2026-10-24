package abs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoAddsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token-123")
	_, err := c.do(context.Background(), http.MethodGet, "/api/test", nil)
	if err != nil {
		t.Fatalf("do() returned error: %v", err)
	}

	want := "Bearer test-token-123"
	if gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
}

func TestDoSetsJSONContentType(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	body := map[string]string{"key": "value"}
	_, err := c.do(context.Background(), http.MethodPost, "/api/test", body)
	if err != nil {
		t.Fatalf("do() returned error: %v", err)
	}

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
}

func TestDoReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.do(context.Background(), http.MethodGet, "/api/fail", nil)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestDoRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxResponseSize+1)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.do(context.Background(), http.MethodGet, "/api/big", nil)
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention exceeds, got: %v", err)
	}
}

func TestLoginDeserializesToken(t *testing.T) {
	resp := LoginResponse{
		User: LoginUser{
			ID:       "user-1",
			Username: "admin",
			Token:    "jwt-token-abc",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			t.Errorf("expected path /login, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Username != "admin" || body.Password != "secret" {
			t.Errorf("unexpected credentials: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	token, err := c.Login(context.Background(), "admin", "secret")
	if err != nil {
		t.Fatalf("Login() returned error: %v", err)
	}
	if token != "jwt-token-abc" {
		t.Errorf("token = %q, want %q", token, "jwt-token-abc")
	}
	if c.token != "jwt-token-abc" {
		t.Errorf("client token not updated: got %q", c.token)
	}
}

func TestNewClientTrimsTheServerAddress(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"user": {"token": "tok"}}`))
	}))
	defer srv.Close()

	c := NewClient(" "+srv.URL+"/ ", "")
	if _, err := c.Login(context.Background(), "admin", "secret"); err != nil {
		t.Fatalf("Login() returned error: %v", err)
	}
	if gotPath != "/login" {
		t.Errorf("request path = %q, want /login", gotPath)
	}
	if c.BaseURL() != srv.URL {
		t.Errorf("BaseURL() = %q, want %q", c.BaseURL(), srv.URL)
	}
}

func TestDoKeepsAShortBodyInTheStatusError(t *testing.T) {
	page := strings.Repeat("<p>Bad Gateway</p>", 10000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.do(context.Background(), http.MethodGet, "/api/items/a", nil)
	if !IsHTTPStatus(err, http.StatusBadGateway) {
		t.Fatalf("do() error = %v, want the status 502", err)
	}
	if len(err.Error()) > 600 {
		t.Errorf("len(err.Error()) = %d, want at most 600", len(err.Error()))
	}
}
