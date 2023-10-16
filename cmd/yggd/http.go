package main

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

var (
	client    *http.Client
	userAgent string
)

// initHTTPClient initializes the HTTP Client that is used by the get and post
// functions.
func initHTTPClient(config *tls.Config, ua string) {
	client = &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	client.Transport.(*http.Transport).TLSClientConfig = config

	userAgent = ua
}

func get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}
	req.Header.Add("User-Agent", userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot download from URL: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body: %w", err)
	}
	log.Debugf("received HTTP %v", resp.Status)

	if resp.StatusCode >= 400 {
		return nil, &yggdrasil.APIResponseError{
			Code: resp.StatusCode,
			Body: strings.TrimSpace(string(data)),
		}
	}

	return data, nil
}

func post(url string, headers map[string]string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot create HTTP request: %w", err)
	}

	for k, v := range headers {
		req.Header.Add(k, strings.TrimSpace(v))
	}
	req.Header.Add("User-Agent", userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot post to URL: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot read response body: %w", err)
	}
	log.Debugf("received HTTP %v", resp.Status)

	if resp.StatusCode >= 400 {
		return &yggdrasil.APIResponseError{
			Code: resp.StatusCode,
			Body: strings.TrimSpace(string(data)),
		}
	}

	return nil
}
