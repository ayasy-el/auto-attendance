package ethol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto-attendance/internal/config"
)

type Client struct {
	cfg     config.Config
	http    *http.Client
	tokenMu sync.RWMutex
	token   string
}
type Student struct {
	Number int    `json:"nomor"`
	Name   string `json:"nama"`
}
type Course struct {
	Number  int `json:"nomor"`
	Origin  int `json:"kuliah_asal"`
	Schema  int `json:"jenisSchema"`
	Subject struct {
		Name string `json:"nama"`
	} `json:"matakuliah"`
}
type ClassTime struct {
	Course int    `json:"kuliah"`
	Day    int    `json:"nomor_hari"`
	Start  string `json:"jam_awal"`
	End    string `json:"jam_akhir"`
}
type Notification struct {
	ID   string `json:"idNotifikasi"`
	Data string `json:"dataTerkait"`
}
type activePresence struct {
	Key  string `json:"key"`
	Open int    `json:"open"`
}

func New(cfg config.Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	timeout, _ := time.ParseDuration(cfg.HTTP.Timeout)
	return &Client{cfg: cfg, http: &http.Client{Jar: jar, Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) Login(ctx context.Context) (Student, error) {
	service := strings.TrimRight(c.cfg.Host, "/") + "/api/auth/cas-callback"
	resp, body, err := c.request(ctx, http.MethodGet, strings.TrimRight(c.cfg.CASHost, "/")+"/cas/login?service="+url.QueryEscape(service), nil, "")
	if err != nil || resp.StatusCode != http.StatusOK {
		return Student{}, fmt.Errorf("ambil halaman CAS: %w", responseError(resp, err))
	}
	lt := regexp.MustCompile(`name=["']lt["'][^>]*value=["']([^"']+)`).FindStringSubmatch(string(body))
	if len(lt) != 2 {
		return Student{}, errors.New("CAS_LT tidak ditemukan")
	}
	form := url.Values{"username": {c.cfg.Username}, "password": {c.cfg.Password}, "lt": {html.UnescapeString(lt[1])}, "_eventId": {"submit"}, "submit": {"LOGIN"}}
	resp, _, err = c.request(ctx, http.MethodPost, strings.TrimRight(c.cfg.CASHost, "/")+"/cas/login?service="+url.QueryEscape(service), strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil || resp.StatusCode != http.StatusFound {
		return Student{}, fmt.Errorf("login CAS: %w", responseError(resp, err))
	}
	ticketURL := resp.Header.Get("Location")
	if ticketURL == "" {
		return Student{}, errors.New("CAS tidak mengembalikan ticket")
	}
	if u, err := url.Parse(ticketURL); err == nil && !u.IsAbs() {
		base, _ := url.Parse(c.cfg.CASHost)
		ticketURL = base.ResolveReference(u).String()
	}
	resp, _, err = c.request(ctx, http.MethodGet, ticketURL, nil, "")
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return Student{}, fmt.Errorf("callback CAS: %w", responseError(resp, err))
	}
	if token := tokenFromCookies(resp.Cookies()); token != "" {
		c.setToken(token)
	}
	if c.Token() == "" {
		return Student{}, errors.New("token CAS tidak ditemukan")
	}
	var student Student
	if err := c.getJSON(ctx, "/api/auth/validasi-token", nil, &student); err != nil {
		return Student{}, fmt.Errorf("validasi token: %w", err)
	}
	return student, nil
}

// Token returns the token captured during the one-time CAS login.
func (c *Client) Token() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.token
}

func (c *Client) setToken(token string) {
	c.tokenMu.Lock()
	c.token = token
	c.tokenMu.Unlock()
}

func tokenFromCookies(cookies []*http.Cookie) string {
	for _, cookie := range cookies {
		if cookie.Name == "token" {
			return cookie.Value
		}
	}
	return ""
}

func (c *Client) Courses(ctx context.Context) ([]Course, error) {
	var v []Course
	err := c.getJSON(ctx, "/api/kuliah", url.Values{"tahun": {strconv.Itoa(c.cfg.Tahun)}, "semester": {strconv.Itoa(c.cfg.Semester)}}, &v)
	return v, err
}
func (c *Client) Schedules(ctx context.Context, courses []Course) ([]ClassTime, error) {
	body := struct {
		Courses []struct {
			Number int `json:"nomor"`
			Schema int `json:"jenisSchema"`
		} `json:"kuliahs"`
		Tahun    int `json:"tahun"`
		Semester int `json:"semester"`
	}{Tahun: c.cfg.Tahun, Semester: c.cfg.Semester}
	for _, course := range courses {
		body.Courses = append(body.Courses, struct {
			Number int `json:"nomor"`
			Schema int `json:"jenisSchema"`
		}{course.Number, course.Schema})
	}
	var v []ClassTime
	err := c.postJSON(ctx, "/api/kuliah/hari-kuliah-in", body, &v)
	return v, err
}
func (c *Client) Notifications(ctx context.Context) ([]Notification, error) {
	var v []Notification
	err := c.getJSON(ctx, "/api/notifikasi/mahasiswa", url.Values{"filterNotif": {"PRESENSI"}}, &v)
	return v, err
}
func (c *Client) Attend(ctx context.Context, n Notification, student Student) (string, error) {
	parts := strings.Split(n.Data, "-")
	if len(parts) != 2 {
		return "", fmt.Errorf("dataTerkait notifikasi %q tidak valid", n.Data)
	}
	kuliah, _ := strconv.Atoi(parts[0])
	schema, _ := strconv.Atoi(parts[1])
	var details []Course
	if err := c.getJSON(ctx, "/api/kuliah/by-kuliah-js", url.Values{"kuliah": {parts[0]}, "jenisSchema": {parts[1]}}, &details); err != nil {
		return "", err
	}
	if len(details) == 0 {
		return "", fmt.Errorf("detail kuliah %d kosong", kuliah)
	}
	var active []activePresence
	if err := c.getJSON(ctx, "/api/presensi/aktif-kuliah", url.Values{"kuliah": {parts[0]}, "jenis_schema": {parts[1]}}, &active); err != nil {
		return "", err
	}
	var key string
	for _, p := range active {
		if p.Open == 1 {
			key = p.Key
			break
		}
	}
	if key == "" {
		return "", errors.New("tidak ada presensi aktif")
	}
	payload := map[string]any{"kuliah": kuliah, "jenis_schema": schema, "mahasiswa": student.Number, "key": key, "kuliah_asal": details[0].Origin}
	var result struct {
		Success bool   `json:"sukses"`
		Message string `json:"pesan"`
	}
	if err := c.postJSON(ctx, "/api/presensi/mahasiswa", payload, &result); err != nil {
		return "", err
	}
	if !result.Success && result.Message == "Setujui kontrak perkuliahan terlebih dahulu sebelum melakukan presensi." {
		var contract struct {
			Success bool   `json:"sukses"`
			Message string `json:"pesan"`
		}
		if err := c.postJSON(ctx, "/api/kontrak/setuju", map[string]any{
			"kuliah":       kuliah,
			"jenis_schema": schema,
		}, &contract); err != nil {
			return "", fmt.Errorf("setuju kontrak: %w", err)
		}
		if !contract.Success {
			if contract.Message != "" {
				return "", fmt.Errorf("setuju kontrak ditolak: %s", contract.Message)
			}
			return "", errors.New("setuju kontrak ditolak")
		}
		if err := c.postJSON(ctx, "/api/presensi/mahasiswa", payload, &result); err != nil {
			return "", err
		}
	}
	if !result.Success {
		if result.Message != "" {
			return "", fmt.Errorf("server menolak presensi: %s", result.Message)
		}
		return "", errors.New("server menolak presensi")
	}
	var history json.RawMessage
	if err := c.getJSON(ctx, "/api/presensi/riwayat", url.Values{"kuliah": {parts[0]}, "jenis_schema": {parts[1]}, "nomor": {strconv.Itoa(student.Number)}}, &history); err != nil {
		return "", err
	}
	return key, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	u := strings.TrimRight(c.cfg.Host, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	resp, body, err := c.request(ctx, http.MethodGet, u, nil, "")
	return decode(resp, body, err, out)
}
func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	resp, body, err := c.request(ctx, http.MethodPost, strings.TrimRight(c.cfg.Host, "/")+path, strings.NewReader(string(b)), "application/json")
	return decode(resp, body, err, out)
}
func (c *Client) request(ctx context.Context, method, target string, body io.Reader, contentType string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", c.cfg.HTTP.UserAgent)
	if token := c.Token(); token != "" {
		// The cookie jar still carries the session cookie, while these headers
		// make the captured token explicit for every subsequent API request.
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Auth-Token", token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	b, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, b, readErr
}
func decode(resp *http.Response, body []byte, requestErr error, out any) error {
	if requestErr != nil {
		return requestErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}
func responseError(resp *http.Response, err error) error {
	if err != nil {
		return err
	}
	if resp == nil {
		return errors.New("tanpa response")
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}
