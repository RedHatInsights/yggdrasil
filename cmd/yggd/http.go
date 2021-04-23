package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
)

func get(c *yggdrasil.HTTPClient, url string) ([]byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot download from URL: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, &yggdrasil.APIResponseError{Code: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	log.Debugf("received HTTP %v: %v", resp.Status, string(data))

	return data, nil
}

func post(c *yggdrasil.HTTPClient, url string, headers map[string]string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot create HTTP request: %w", err)
	}

	for k, v := range headers {
		req.Header.Add(k, strings.TrimSpace(v))
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("cannot post to URL: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &yggdrasil.APIResponseError{Code: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	log.Debugf("received HTTP %v: %v", resp.Status, string(data))

	return nil
}
