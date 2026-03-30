package abs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoAddsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
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
		w.Write([]byte(`{}`))
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
		json.NewEncoder(w).Encode(resp)
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
