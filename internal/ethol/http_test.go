package ethol

import (
	"errors"
	"net/http"
	"testing"
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
