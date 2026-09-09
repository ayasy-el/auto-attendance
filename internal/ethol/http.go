package ethol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	err := c.getJSONNoRelogin(ctx, path, query, out)
	if !errors.Is(err, errNotLoggedIn) {
		return err
	}
	if refreshErr := c.refreshAccessToken(ctx); refreshErr == nil {
		return c.getJSONNoRelogin(ctx, path, query, out)
	} else {
		log.Printf("refresh token gagal, mencoba login ulang: %v", refreshErr)
	}
	if _, loginErr := c.Login(ctx); loginErr != nil {
		return fmt.Errorf("login ulang: %w", loginErr)
	}
	return c.getJSONNoRelogin(ctx, path, query, out)
}
func (c *Client) getJSONNoRelogin(ctx context.Context, path string, query url.Values, out any) error {
	u := strings.TrimRight(c.cfg.Host, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	resp, body, err := c.request(ctx, http.MethodGet, u, nil, "")
	return decode(resp, body, err, out)
}
func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	err := c.postJSONNoRelogin(ctx, path, in, out)
	if !errors.Is(err, errNotLoggedIn) {
		return err
	}
	if refreshErr := c.refreshAccessToken(ctx); refreshErr == nil {
		return c.postJSONNoRelogin(ctx, path, in, out)
	} else {
		log.Printf("refresh token gagal, mencoba login ulang: %v", refreshErr)
	}
	if _, loginErr := c.Login(ctx); loginErr != nil {
		return fmt.Errorf("login ulang: %w", loginErr)
	}
	return c.postJSONNoRelogin(ctx, path, in, out)
}
func (c *Client) postJSONNoRelogin(ctx context.Context, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	resp, body, err := c.request(ctx, http.MethodPost, strings.TrimRight(c.cfg.Host, "/")+path, strings.NewReader(string(b)), "application/json")
	return decode(resp, body, err, out)
}
func (c *Client) putJSON(ctx context.Context, path string, in, out any) error {
	err := c.putJSONNoRelogin(ctx, path, in, out)
	if !errors.Is(err, errNotLoggedIn) {
		return err
	}
	if refreshErr := c.refreshAccessToken(ctx); refreshErr == nil {
		return c.putJSONNoRelogin(ctx, path, in, out)
	} else {
		log.Printf("refresh token gagal, mencoba login ulang: %v", refreshErr)
	}
	if _, loginErr := c.Login(ctx); loginErr != nil {
		return fmt.Errorf("login ulang: %w", loginErr)
	}
	return c.putJSONNoRelogin(ctx, path, in, out)
}
func (c *Client) putJSONNoRelogin(ctx context.Context, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	resp, body, err := c.request(ctx, http.MethodPut, strings.TrimRight(c.cfg.Host, "/")+path, strings.NewReader(string(b)), "application/json")
	return decode(resp, body, err, out)
}
func (c *Client) request(ctx context.Context, method, target string, body io.Reader, contentType string) (*http.Response, []byte, error) {
	return c.requestWithOptions(ctx, method, target, body, contentType, "", true)
}

func (c *Client) requestWithCookie(ctx context.Context, method, target string, body io.Reader, contentType, cookie string) (*http.Response, []byte, error) {
	return c.requestWithOptions(ctx, method, target, body, contentType, cookie, false)
}

func (c *Client) requestWithOptions(ctx context.Context, method, target string, body io.Reader, contentType, cookie string, includeToken bool) (*http.Response, []byte, error) {
	started := time.Now()
	log.Printf("http request: method=%s url=%s", method, target)
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		log.Printf("http response: status=client_error duration=%s error=%v", time.Since(started), err)
		return nil, nil, err
	}
	req.Header.Set("User-Agent", c.cfg.HTTP.UserAgent)
	if includeToken {
		if token := c.Token(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Auth-Token", token)
		}
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		log.Printf("http response: status=network_error duration=%s error=%v", time.Since(started), err)
		return nil, nil, err
	}
	b, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		log.Printf("http response: status=%d duration=%s read_error=%v body=%q", resp.StatusCode, time.Since(started), readErr, responseBodyForLog(b))
	} else if resp.StatusCode >= 400 {
		log.Printf("http response: status=%d duration=%s bytes=%d body=%q", resp.StatusCode, time.Since(started), len(b), responseBodyForLog(b))
	} else if isErrorResponseBody(b) {
		log.Printf("http response: status=%d duration=%s bytes=%d response_error=true body=%q", resp.StatusCode, time.Since(started), len(b), responseBodyForLog(b))
	} else {
		log.Printf("http response: status=%d duration=%s bytes=%d", resp.StatusCode, time.Since(started), len(b))
	}
	return resp, b, readErr
}

func isErrorResponseBody(body []byte) bool {
	var response struct {
		Success *bool `json:"sukses"`
	}
	return json.Unmarshal(body, &response) == nil && response.Success != nil && !*response.Success
}

func responseBodyForLog(body []byte) string {
	const maxResponseBodyLog = 4096
	value := strings.TrimSpace(string(body))
	if len(value) > maxResponseBodyLog {
		return value[:maxResponseBodyLog] + "... [terpotong]"
	}
	return value
}
func decode(resp *http.Response, body []byte, requestErr error, out any) error {
	if requestErr != nil {
		return requestErr
	}
	if isNotLoggedIn(body) {
		return errNotLoggedIn
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}
func isNotLoggedIn(body []byte) bool {
	var response struct {
		Success *bool  `json:"sukses"`
		Message string `json:"pesan"`
	}
	if json.Unmarshal(body, &response) != nil || response.Success == nil {
		return false
	}
	return !*response.Success && strings.Contains(strings.ToLower(response.Message), "login") // Anda belum melakukan Login || Harap login terlebih dahulu
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
