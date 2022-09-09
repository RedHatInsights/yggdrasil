package http

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"git.sr.ht/~spc/go-log"
)

// Client is a specialized HTTP client, configured with mutual TLS certificate
// authentication.
type Client struct {
	client    *http.Client
	userAgent string
}

// NewHTTPClient creates a client with the given TLS configuration and
// user-agent string.
func NewHTTPClient(config *tls.Config, ua string) *Client {
	client := &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	client.Transport.(*http.Transport).TLSClientConfig = config.Clone()

	return &Client{
		client:    client,
		userAgent: ua,
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

	return c.client.Do(req)
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

	return c.client.Do(req)
}
