package main

import (
	"fmt"

	yggdrasil "github.com/redhatinsights/yggdrasil/pkg"
)

func newClient(baseURL, authMode, username, password, certFile, keyFile, caRoot string) (*yggdrasil.HTTPClient, error) {
	userAgent := fmt.Sprintf("ygg-exec/%v", yggdrasil.Version)

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
		return nil, &invalidArugmentError{"auth-mode", authMode}
	}

	return client, nil
}
