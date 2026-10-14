package ethol

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestActivePeriodAutoDetectAndCache(t *testing.T) {
	var configCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/config":
			configCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"tahun_aktif": 2026, "semester_aktif": 1}`)
		case "/api/kuliah":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"nomor":1,"jenisSchema":4,"kuliah_asal":1,"matakuliah":{"nama":"Jaringan"}}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	y, s, err := client.ActivePeriod(ctx)
	if err != nil {
		t.Fatalf("ActivePeriod error: %v", err)
	}
	if y != 2026 || s != 1 {
		t.Fatalf("got y=%d s=%d, want 2026 and 1", y, s)
	}
	if configCalls.Load() != 1 {
		t.Fatalf("expected 1 call to /api/auth/config, got %d", configCalls.Load())
	}

	// Second call should use cache
	y2, s2, err := client.ActivePeriod(ctx)
	if err != nil || y2 != 2026 || s2 != 1 {
		t.Fatalf("second ActivePeriod failed: %v", err)
	}
	if configCalls.Load() != 1 {
		t.Fatalf("expected cached call not to hit server, got %d calls", configCalls.Load())
	}

	// ResetActivePeriod clears cache
	client.ResetActivePeriod()
	_, _, err = client.ActivePeriod(ctx)
	if err != nil {
		t.Fatalf("ActivePeriod after reset: %v", err)
	}
	if configCalls.Load() != 2 {
		t.Fatalf("expected 2 calls after reset, got %d", configCalls.Load())
	}

	// Courses uses auto-detected period
	courses, err := client.Courses(ctx)
	if err != nil {
		t.Fatalf("Courses: %v", err)
	}
	if len(courses) != 1 || courses[0].Subject.Name != "Jaringan" {
		t.Fatalf("unexpected courses: %#v", courses)
	}
}
