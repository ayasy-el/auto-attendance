package ethol

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

func (c *Client) Login(ctx context.Context) (Student, error) {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	return c.login(ctx)
}

func (c *Client) login(ctx context.Context) (Student, error) {
	// CAS dapat membutuhkan cookie untuk mempertahankan sesi antara halaman
	// login, POST credential, dan callback. Cookie ini hanya hidup selama
	// handshake; setelah token didapat, request API kembali token-only.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return Student{}, fmt.Errorf("buat cookie jar login: %w", err)
	}
	c.http.Jar = jar
	defer func() { c.http.Jar = nil }()
	c.setToken("")

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
		if callbackURL, parseErr := url.Parse(ticketURL); parseErr == nil {
			if token := tokenFromCookies(c.http.Jar.Cookies(callbackURL)); token != "" {
				c.setToken(token)
			}
		}
	}
	var student Student
	if err := c.getJSONNoRelogin(ctx, "/api/auth/validasi-token", nil, &student); err != nil {
		return Student{}, fmt.Errorf("validasi token: %w", err)
	}
	if c.Token() == "" {
		return Student{}, errors.New("token CAS tidak ditemukan setelah validasi")
	}
	return student, nil
}

func (c *Client) Token() string         { c.tokenMu.RLock(); defer c.tokenMu.RUnlock(); return c.token }
func (c *Client) setToken(token string) { c.tokenMu.Lock(); c.token = token; c.tokenMu.Unlock() }
func tokenFromCookies(cookies []*http.Cookie) string {
	for _, cookie := range cookies {
		if cookie.Name == "token" {
			return cookie.Value
		}
	}
	return ""
}
