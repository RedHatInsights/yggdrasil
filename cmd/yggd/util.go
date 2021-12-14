package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := make([]byte, n)
	for i := range data {
		data[i] = letters[rand.Intn(len(letters))]
	}
	return string(data)
}

// parseCertCN parses the contents of filename as an x509 certificate and
// returns the Subject CommonName.
func parseCertCN(filename string) (string, error) {
	var asn1Data []byte
	switch filepath.Ext(filename) {
	case ".pem":
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return "", err
		}

		block, _ := pem.Decode(data)
		if block == nil {
			return "", fmt.Errorf("failed to decode PEM data: %v", filename)
		}
		asn1Data = append(asn1Data, block.Bytes...)
	default:
		var err error
		asn1Data, err = ioutil.ReadFile(filename)
		if err != nil {
			return "", err
		}
	}

	cert, err := x509.ParseCertificate(asn1Data)
	if err != nil {
		return "", err
	}
	return cert.Subject.CommonName, nil
}

// createClientID will generate a semi-random string to be used as the MQTT
// client ID and save the value to the client-id file.
func createClientID(file string) ([]byte, error) {
	if _, err := os.Stat(file); os.IsExist(err) {
		return nil, fmt.Errorf("cannot create client-id: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("cannot get hostname: %w", err)
	}

	generatedUUID, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate UUID: %w", err)
	}

	data := []byte(hostname + "-" + generatedUUID.String())

	if err := setClientID(data, file); err != nil {
		return nil, fmt.Errorf("cannot set client-id: %w", err)
	}

	return data, nil
}

// setClientID writes data to the client ID file.
func setClientID(data []byte, file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}

	if err := ioutil.WriteFile(file, data, 0600); err != nil {
		return fmt.Errorf("cannot write file: %w", err)
	}

	return nil

}

// getClientID reads data from the client ID file.
func getClientID(file string) ([]byte, error) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, nil
	}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	return data, nil
}
