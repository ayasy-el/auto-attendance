package ethol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"auto-attendance/internal/config"
)

func TestIsNotLoggedIn(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"exact", `{"sukses":false,"pesan":"Anda belum melakukan Login"}`, true},
		{"case insensitive", `{"sukses":false,"pesan":"anda belum melakukan login"}`, true},
		{"success", `{"sukses":true,"pesan":"ok"}`, false},
		{"other failure", `{"sukses":false,"pesan":"presensi ditolak"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotLoggedIn([]byte(tc.body)); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	if !errors.Is(decode(&http.Response{StatusCode: 200}, []byte(`{"sukses":false,"pesan":"Anda belum melakukan Login"}`), nil, nil), errNotLoggedIn) {
		t.Fatal("expected not-logged-in sentinel")
	}
	if !errors.Is(decode(&http.Response{StatusCode: http.StatusUnauthorized}, []byte("unauthorized"), nil, nil), errUnauthorized) {
		t.Fatal("expected unauthorized sentinel")
	}
	if err := decode(&http.Response{StatusCode: 500}, []byte("oops"), nil, nil); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestIsAuthPath(t *testing.T) {
	cases := map[string]bool{
		"/auth/refresh":            true,
		"/api/auth/validasi-token": true,
		"api/auth/refresh":         true,
		"/api/auth/cas-callback":   true,
		"/api/auth/config":         false,
		"/api/auth":                false,
		"/api/kuliah":              false,
		"/api/authorize":           false,
	}
	for path, want := range cases {
		if got := isAuthPath(path); got != want {
			t.Errorf("isAuthPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestRequestWithOptionsMaxResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Repeat("A", 200))
	}))
	defer server.Close()

	cfg := config.Config{
		Host: server.URL,
		HTTP: config.HTTPConfig{
			Timeout:          "5s",
			MaxResponseBytes: 50,
		},
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, body, err := client.request(context.Background(), http.MethodGet, server.URL, nil, "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(body) != 50 {
		t.Fatalf("expected body capped at 50 bytes, got %d bytes", len(body))
	}
}
