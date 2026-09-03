package ethol

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"auto-attendance/internal/config"
)

type Client struct {
	cfg     config.Config
	http    *http.Client
	tokenMu sync.RWMutex
	token   string
	loginMu sync.Mutex
}

func New(cfg config.Config) (*Client, error) {
	timeout, _ := time.ParseDuration(cfg.HTTP.Timeout)
	return &Client{cfg: cfg, http: &http.Client{
		// Authentication is sent explicitly with the token headers. No
		// cookie jar is used or persisted.
		Jar:     nil,
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}, nil
}

var errNotLoggedIn = errors.New("anda belum melakukan login")
