package ethol

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
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
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	timeout, _ := time.ParseDuration(cfg.HTTP.Timeout)
	return &Client{cfg: cfg, http: &http.Client{Jar: jar, Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

var errNotLoggedIn = errors.New("anda belum melakukan login")
