package http

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"git.sr.ht/~spc/go-log"
)

type Response struct {
	// StatusCode response
	StatusCode int
	// Response Body
	Body json.RawMessage
	// Mestadata added by the transport, in case of http are the headers
	Metadata map[string]string
}

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

func (c *Client) Get(url string) (*Response, error) {
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
	return createResponse(resp)
}

func (c *Client) Post(url string, headers map[string]string, body []byte) (*Response, error) {
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot post to URL: %w", err)
	}
	return createResponse(resp)
}

func createResponse(resp *http.Response) (*Response, error) {
	result := &Response{
		StatusCode: resp.StatusCode,
		Metadata:   map[string]string{},
	}

	for k, v := range resp.Header {
		result.Metadata[k] = strings.Join(v, ";")
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("cannot read response body: %w", err)
	}
	log.Debugf("received HTTP %v: %v", resp.Status, strings.TrimSpace(string(data)))
	result.Body = data
	if resp.StatusCode >= 400 {
		return result, &APIResponseError{Code: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}

	return result, nil
}
