package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

func newTLSConfig(certPEMBlock []byte, keyPEMBlock []byte, CARootPEMBlocks [][]byte) (*tls.Config, error) {
	config := &tls.Config{}

	if len(certPEMBlock) > 0 && len(keyPEMBlock) > 0 {
		cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("cannot parse x509 key pair: %w", err)
		}

		config.Certificates = []tls.Certificate{cert}
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("cannot copy system certificate pool: %w", err)
	}
	for _, data := range CARootPEMBlocks {
		pool.AppendCertsFromPEM(data)
	}
	config.RootCAs = pool

	return config, nil
}
