package ethol

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAttendCourseReturnsNoActivePresence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) }))
	defer server.Close()
	c, _ := New(testConfig(server.URL))
	_, err := c.AttendCourse(context.Background(), Course{Number: 10, Schema: 4, Origin: 10}, Student{Number: 1})
	if err != ErrNoActivePresence {
		t.Fatalf("err=%v, want ErrNoActivePresence", err)
	}
}

func TestAttendCourseApprovesContractThenRetries(t *testing.T) {
	var submitCalls atomic.Int32
	var contractCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/presensi/aktif-kuliah":
			fmt.Fprint(w, `[{"key":"active-key","open":1}]`)
		case "/api/presensi/mahasiswa":
			if submitCalls.Add(1) == 1 {
				fmt.Fprint(w, `{"sukses":false,"pesan":"Setujui kontrak perkuliahan terlebih dahulu sebelum melakukan presensi."}`)
			} else {
				fmt.Fprint(w, `{"sukses":true,"pesan":"Presensi berhasil"}`)
			}
		case "/api/kontrak/setuju":
			contractCalls.Add(1)
			fmt.Fprint(w, `{"sukses":true}`)
		case "/api/presensi/riwayat":
			fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, _ := New(testConfig(server.URL))
	c.setToken("test-token")
	key, err := c.AttendCourse(context.Background(), Course{Number: 10, Schema: 4, Origin: 11}, Student{Number: 99})
	if err != nil || key != "active-key" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	if submitCalls.Load() != 2 || contractCalls.Load() != 1 {
		t.Fatalf("submit=%d contract=%d", submitCalls.Load(), contractCalls.Load())
	}
}

func TestAttendCourseReturnsAlreadyAttended(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/presensi/aktif-kuliah":
			fmt.Fprint(w, `[{"key":"active-key","open":1}]`)
		case "/api/presensi/riwayat":
			fmt.Fprint(w, `[]`)
		case "/api/presensi/mahasiswa":
			fmt.Fprint(w, `{"sukses":false,"pesan":"Anda sudah melakukan di sesi kuliah ini"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, _ := New(testConfig(server.URL))
	c.setToken("test-token")
	_, err := c.AttendCourse(context.Background(), Course{Number: 10, Schema: 4, Origin: 11}, Student{Number: 99})
	if err != ErrAlreadyAttended {
		t.Fatalf("err=%v, want ErrAlreadyAttended", err)
	}
}

func TestAttendCourseSkipsSubmitWhenActiveKeyIsInHistory(t *testing.T) {
	var submitCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/presensi/aktif-kuliah":
			fmt.Fprint(w, `[{"key":"active-key","open":1}]`)
		case "/api/presensi/riwayat":
			fmt.Fprint(w, `[{"key":"active-key"}]`)
		case "/api/presensi/mahasiswa":
			submitCalls.Add(1)
			fmt.Fprint(w, `{"sukses":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, _ := New(testConfig(server.URL))
	c.setToken("test-token")
	_, err := c.AttendCourse(context.Background(), Course{Number: 10, Schema: 4, Origin: 11}, Student{Number: 99})
	if err != ErrAlreadyAttended {
		t.Fatalf("err=%v, want ErrAlreadyAttended", err)
	}
	if submitCalls.Load() != 0 {
		t.Fatalf("submit calls=%d, want 0", submitCalls.Load())
	}
}
