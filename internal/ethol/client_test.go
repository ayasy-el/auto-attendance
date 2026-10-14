package ethol

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"auto-attendance/internal/config"
)

func testConfig(host string) config.Config {
	return config.Config{Host: host, CASHost: host, Username: "user", Password: "pass", Tahun: 2026, Semester: 1, HTTP: config.HTTPConfig{Timeout: "5s", UserAgent: "test"}}
}

func TestLoginUsesTemporaryCASCookieAndAPIUsesToken(t *testing.T) {
	var apiCookie bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cas/login":
			if r.Method == http.MethodGet {
				http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "cas"})
				fmt.Fprint(w, `<input name="lt" value="lt-test">`)
				return
			}
			if r.Method == http.MethodPost {
				if _, err := r.Cookie("JSESSIONID"); err != nil {
					t.Errorf("CAS session cookie missing: %v", err)
				}
				w.Header().Set("Location", "/api/auth/cas-callback?ticket=t")
				w.WriteHeader(http.StatusFound)
				return
			}
		case "/api/auth/cas-callback":
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "token-test", Path: "/"})
			w.WriteHeader(http.StatusFound)
			return
		case "/api/auth/validasi-token":
			if r.Header.Get("Authorization") != "Bearer token-test" {
				t.Errorf("token header = %q", r.Header.Get("Authorization"))
			}
			fmt.Fprint(w, `{"nomor":123,"nama":"Tester"}`)
			return
		case "/api/kuliah":
			apiCookie = r.Header.Get("Cookie") != ""
			if r.Header.Get("Authorization") != "Bearer token-test" {
				t.Errorf("API token missing")
			}
			fmt.Fprint(w, `[]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	c, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	student, err := c.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if student.Number != 123 || c.Token() != "token-test" {
		t.Fatalf("student=%+v token=%q", student, c.Token())
	}
	if _, err := c.Courses(context.Background()); err != nil {
		t.Fatal(err)
	}
	if apiCookie {
		t.Fatal("API request must not send cookies")
	}
}

func TestRequestReloginsOnceWhenSessionExpires(t *testing.T) {
	var logins atomic.Int32
	var courseCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cas/login":
			if r.Method == http.MethodGet {
				fmt.Fprint(w, `<input name="lt" value="lt">`)
				return
			}
			logins.Add(1)
			w.Header().Set("Location", "/api/auth/cas-callback?ticket=t")
			w.WriteHeader(http.StatusFound)
			return
		case "/api/auth/cas-callback":
			http.SetCookie(w, &http.Cookie{Name: "token", Value: fmt.Sprintf("token-%d", logins.Load()), Path: "/"})
			w.WriteHeader(http.StatusFound)
			return
		case "/api/auth/validasi-token":
			fmt.Fprint(w, `{"nomor":1,"nama":"Tester"}`)
			return
		case "/api/kuliah":
			if courseCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"sukses":false,"pesan":"Anda belum melakukan Login"}`)
				return
			}
			fmt.Fprint(w, `[]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	c, _ := New(testConfig(server.URL))
	if _, err := c.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Courses(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 2 {
		t.Fatalf("login count=%d, want 2", logins.Load())
	}
	if !strings.HasPrefix(c.Token(), "token-2") {
		t.Fatalf("token=%q", c.Token())
	}
}

func TestRequestRefreshesTokenBeforeRelogin(t *testing.T) {
	var logins atomic.Int32
	var courseCalls atomic.Int32
	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cas/login":
			if r.Method == http.MethodGet {
				fmt.Fprint(w, `<input name="lt" value="lt">`)
				return
			}
			logins.Add(1)
			w.Header().Set("Location", "/api/auth/cas-callback?ticket=t")
			w.WriteHeader(http.StatusFound)
		case "/api/auth/cas-callback":
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "token-1", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "refresh-1", Path: "/"})
			w.WriteHeader(http.StatusFound)
		case "/api/auth/validasi-token":
			fmt.Fprint(w, `{"nomor":1,"nama":"Tester"}`)
		case "/api/auth/refresh":
			refreshCalls.Add(1)
			cookie, err := r.Cookie("refresh_token")
			if err != nil || cookie.Value != "refresh-1" {
				t.Fatalf("refresh cookie = %v, want refresh-1", err)
			}
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "token-refreshed", Path: "/"})
			w.WriteHeader(http.StatusOK)
		case "/api/kuliah":
			if courseCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"sukses":false,"pesan":"Anda belum melakukan Login"}`)
				return
			}
			if r.Header.Get("Authorization") != "Bearer token-refreshed" {
				t.Errorf("refreshed token = %q", r.Header.Get("Authorization"))
			}
			fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c, _ := New(testConfig(server.URL))
	if _, err := c.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Courses(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 1 {
		t.Fatalf("login count=%d, want 1", logins.Load())
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("refresh count=%d, want 1", refreshCalls.Load())
	}
	if c.Token() != "token-refreshed" {
		t.Fatalf("token=%q, want token-refreshed", c.Token())
	}
}
