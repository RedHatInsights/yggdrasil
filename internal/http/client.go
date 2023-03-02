package http

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"git.sr.ht/~spc/go-log"
)

// Client is a specialized HTTP client, configured with mutual TLS certificate
// authentication.
type Client struct {
	http.Client
	userAgent string

	// Retries is the number of times the client will attempt to resend failed
	// HTTP requests before giving up.
	Retries int
}

// NewHTTPClient creates a client with the given TLS configuration and
// user-agent string.
func NewHTTPClient(config *tls.Config, ua string) *Client {
	client := http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	client.Transport.(*http.Transport).TLSClientConfig = config.Clone()

	return &Client{
		Client:    client,
		userAgent: ua,
		Retries:   0,
	}
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}
	req.Header.Add("User-Agent", c.userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	return c.Do(req)
}

func (c *Client) Post(url string, headers map[string]string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}

	for k, v := range headers {
		req.Header.Add(k, strings.TrimSpace(v))
	}
	req.Header.Add("User-Agent", c.userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	return c.Do(req)
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	var attempt int

	for {
		if attempt > c.Retries {
			return nil, fmt.Errorf("cannot do HTTP request: too many retries")
		}
		resp, err = c.Client.Do(req)
		if err != nil {
			if err.(*url.Error).Timeout() {
				attempt++
				continue
			}
			return nil, fmt.Errorf("cannot do HTTP request: %v", err)
		}
		return resp, nil
	}
}
