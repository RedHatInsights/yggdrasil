package yggdrasil

import (
	"fmt"

	yggdrasil "github.com/redhatinsights/yggdrasil/pkg"
)

// NewClient creates and configures an HTTPClient for either certificate-based
// authentication if authMode is "cert" or basic HTTP authentication if authMode
// is "basic".
func NewClient(name, baseURL, authMode, username, password, certFile, keyFile, caRoot string) (*yggdrasil.HTTPClient, error) {
	userAgent := fmt.Sprintf("%v/%v", name, yggdrasil.Version)

	var client *yggdrasil.HTTPClient
	var err error
	switch authMode {
	case "basic":
		client, err = yggdrasil.NewHTTPClientBasicAuth(
			baseURL,
			username,
			password,
			userAgent)
		if err != nil {
			return nil, err
		}
	case "cert":
		client, err = yggdrasil.NewHTTPClientCertAuth(
			baseURL,
			caRoot,
			certFile,
			keyFile,
			userAgent)
		if err != nil {
			return nil, err
		}
	default:
		return nil, &InvalidArgumentError{"auth-mode", authMode}
	}

	return client, nil
}
