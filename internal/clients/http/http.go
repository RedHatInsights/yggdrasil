package http

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
)

type Client struct {
	client    *http.Client
	userAgent string
}

// NewHTTPClient initializes the HTTP Client
func NewHTTPClient(config *tls.Config, ua string) *Client{
	client := &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	client.Transport.(*http.Transport).TLSClientConfig = config

	return &Client{
		client: client,
		userAgent: ua,
	}
}

func (c *Client) Get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}
	req.Header.Add("User-Agent", c.userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot download from URL: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body: %w", err)
	}
	log.Debugf("received HTTP %v: %v", resp.Status, strings.TrimSpace(string(data)))

	if resp.StatusCode >= 400 {
		return nil, &yggdrasil.APIResponseError{Code: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}

	return data, nil
}

func (c *Client) Post(url string, headers map[string]string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot create HTTP request: %w", err)
	}

	for k, v := range headers {
		req.Header.Add(k, strings.TrimSpace(v))
	}
	req.Header.Add("User-Agent", c.userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot post to URL: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot read response body: %w", err)
	}
	log.Debugf("received HTTP %v: %v", resp.Status, strings.TrimSpace(string(data)))

	if resp.StatusCode >= 400 {
		return &yggdrasil.APIResponseError{Code: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}

	return nil
}
