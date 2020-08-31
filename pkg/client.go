package yggdrasil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type authType string

const (
	authTypeBasic authType = "basic"
	authTypeCert           = "cert"
)

// HTTPClient is a specialized HTTP client, preconfigured to authenticate using
// either certificate or basic authentication.
type HTTPClient struct {
	*http.Client
	authType  authType
	username  string
	password  string
	baseURL   string
	userAgent string
}

// NewHTTPClientBasicAuth creates a client configured for basic authentication with
// the given username and password.
func NewHTTPClientBasicAuth(baseURL, username, password, userAgent string) (*HTTPClient, error) {
	if userAgent == "" {
		userAgent = fmt.Sprintf("yggdrasil/%v", Version)
	}
	return &HTTPClient{
		Client:    &http.Client{},
		authType:  authTypeBasic,
		baseURL:   baseURL,
		username:  username,
		password:  password,
		userAgent: userAgent,
	}, nil
}

// NewHTTPClientCertAuth creates a client configured for certificate authentication
// with the given CA root, and certificate key-pair.
func NewHTTPClientCertAuth(baseURL, caRoot, certFile, keyFile, userAgent string) (*HTTPClient, error) {
	if userAgent == "" {
		userAgent = fmt.Sprintf("yggdrasil/%v", Version)
	}
	client := &HTTPClient{
		Client:    &http.Client{},
		authType:  authTypeCert,
		baseURL:   baseURL,
		userAgent: userAgent,
	}

	tlsConfig := tls.Config{
		MaxVersion: tls.VersionTLS12, // cloud.redhat.com appears to exhibit this openssl bug https://github.com/openssl/openssl/issues/9767
	}

	caCert, err := ioutil.ReadFile(caRoot)
	if err != nil {
		return nil, err
	}
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig.RootCAs = caCertPool

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	tlsConfig.Certificates = []tls.Certificate{cert}
	tlsConfig.BuildNameToCertificate()

	// Recreate the default transport with a custom tls.Config
	client.Transport = &http.Transport{
		TLSClientConfig: &tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return client, nil
}

// Do sends an HTTP request and returns an HTTP response, following policy
// as configured on the client.
//
// See http.Client documentation for more details.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.authType == authTypeBasic {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Add("User-Agent", c.userAgent)
	return c.Client.Do(req)
}

// Get issues a GET to the specified URL.
func (c *HTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post issues a POST to the specified URL.
func (c *HTTPClient) Post(url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}
