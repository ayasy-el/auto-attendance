package ethol

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"auto-attendance/internal/config"
)

func testConfig(host string) config.Config {
	return config.Config{Host: host, CASHost: host, Username: "user", Password: "pass", HTTP: config.HTTPConfig{Timeout: "5s", UserAgent: "test"}}
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
		case "/api/auth/config":
			fmt.Fprint(w, `{"tahun_aktif":2026,"semester_aktif":1}`)
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
		case "/api/auth/config":
			fmt.Fprint(w, `{"tahun_aktif":2026,"semester_aktif":1}`)
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
		case "/api/auth/config":
			fmt.Fprint(w, `{"tahun_aktif":2026,"semester_aktif":1}`)
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

func TestConcurrentReauthSingleflight(t *testing.T) {
	var logins atomic.Int32
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
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "token-new", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "refresh-new", Path: "/"})
			w.WriteHeader(http.StatusFound)
		case "/api/auth/validasi-token":
			fmt.Fprint(w, `{"nomor":1,"nama":"Tester"}`)
		case "/api/auth/config":
			fmt.Fprint(w, `{"tahun_aktif":2026,"semester_aktif":1}`)
		case "/api/auth/refresh":
			refreshCalls.Add(1)
			time.Sleep(30 * time.Millisecond) // Simulate network delay
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "token-renewed", Path: "/"})
			w.WriteHeader(http.StatusOK)
		case "/api/kuliah":
			token := r.Header.Get("Authorization")
			if token == "Bearer token-init" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"sukses":false,"pesan":"Anda belum melakukan Login"}`)
				return
			}
			if token == "Bearer token-renewed" {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `[{"nomor":1,"jenisSchema":4,"matakuliah":{"nama":"Jaringan"}}]`)
				return
			}
			http.Error(w, "invalid token", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c, _ := New(testConfig(server.URL))
	c.setToken("token-init")
	c.setRefreshToken("refresh-init")

	// Trigger 5 concurrent requests with expired token
	const concurrency = 5
	errCh := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			_, err := c.Courses(context.Background())
			errCh <- err
		}()
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent request failed: %v", err)
		}
	}

	// Singleflight mutex must ensure refresh is called only ONCE despite 5 concurrent 401s
	if calls := refreshCalls.Load(); calls != 1 {
		t.Fatalf("expected exactly 1 refresh call, got %d", calls)
	}
	if l := logins.Load(); l != 0 {
		t.Fatalf("expected 0 login calls, got %d", l)
	}
	if c.Token() != "token-renewed" {
		t.Fatalf("expected token-renewed, got %q", c.Token())
	}
}

func TestInsecureSkipVerifyTLS(t *testing.T) {
	// Start an HTTPS test server with self-signed certificate
	tsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"nomor":1,"nama":"Tester"}`)
	}))
	defer tsServer.Close()

	// 1. Without InsecureSkipVerify: request to untrusted self-signed TLS server must fail
	strictCfg := config.Config{
		Host:    tsServer.URL,
		CASHost: tsServer.URL,
		HTTP: config.HTTPConfig{
			Timeout:            "5s",
			InsecureSkipVerify: false,
		},
	}
	strictClient, err := New(strictCfg)
	if err != nil {
		t.Fatalf("new strict client: %v", err)
	}
	_, _, err = strictClient.request(context.Background(), http.MethodGet, tsServer.URL, nil, "")
	if err == nil {
		t.Fatal("expected TLS verification error for untrusted self-signed certificate, got nil")
	}

	// 2. With InsecureSkipVerify: request to untrusted self-signed TLS server must succeed
	insecureCfg := config.Config{
		Host:    tsServer.URL,
		CASHost: tsServer.URL,
		HTTP: config.HTTPConfig{
			Timeout:            "5s",
			InsecureSkipVerify: true,
		},
	}
	insecureClient, err := New(insecureCfg)
	if err != nil {
		t.Fatalf("new insecure client: %v", err)
	}
	resp, _, err := insecureClient.request(context.Background(), http.MethodGet, tsServer.URL, nil, "")
	if err != nil {
		t.Fatalf("expected request with InsecureSkipVerify to succeed, got: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
