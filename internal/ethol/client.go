package ethol

import (
	"crypto/tls"
	"errors"
	"net/http"
	"sync"
	"time"

	"auto-attendance/internal/config"
)

type Client struct {
	cfg            config.Config
	http           *http.Client
	tokenMu        sync.RWMutex
	token          string
	refreshTokenMu sync.RWMutex
	refreshToken   string
	loginMu        sync.Mutex
	reauthMu       sync.Mutex
	periodMu       sync.RWMutex
	activeYear     int
	activeSemester int
}

func (c *Client) ResetActivePeriod() {
	c.periodMu.Lock()
	c.activeYear = 0
	c.activeSemester = 0
	c.periodMu.Unlock()
}

func (c *Client) Config() config.Config {
	return c.cfg
}

func New(cfg config.Config) (*Client, error) {
	timeout, _ := time.ParseDuration(cfg.HTTP.Timeout)
	var transport http.RoundTripper
	if cfg.HTTP.InsecureSkipVerify {
		baseTransport := http.DefaultTransport.(*http.Transport).Clone()
		if baseTransport.TLSClientConfig == nil {
			baseTransport.TLSClientConfig = &tls.Config{}
		}
		baseTransport.TLSClientConfig.InsecureSkipVerify = true
		transport = baseTransport
	}
	return &Client{cfg: cfg, http: &http.Client{
		// Authentication is sent explicitly with the token headers. No
		// cookie jar is used or persisted.
		Jar:           nil,
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}, nil
}

var (
	errNotLoggedIn  = errors.New("anda belum melakukan login")
	errUnauthorized = errors.New("request tidak terautorisasi")
)
