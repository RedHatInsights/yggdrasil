package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
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
		data, err := os.ReadFile(filename)
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
		asn1Data, err = os.ReadFile(filename)
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
func createClientID(file string, serviceUser *user.User) ([]byte, error) {
	if _, err := os.Stat(file); os.IsExist(err) {
		return nil, fmt.Errorf("cannot create client-id: %w", err)
	}

	data := []byte(randomString(64))

	if err := setClientID(data, file, serviceUser); err != nil {
		return nil, fmt.Errorf("cannot set client-id: %w", err)
	}

	return data, nil
}

// setClientID writes data to the client ID file and set ownership of file to
// given service user, when this user is specified
func setClientID(data []byte, file string, serviceUser *user.User) error {
	var uid int
	var gid int
	var err error

	if serviceUser != nil {
		uid, err = strconv.Atoi(serviceUser.Uid)
		if err != nil {
			return fmt.Errorf("unable to convert uid: %s to int: %s", serviceUser.Uid, err)
		}
		gid, err = strconv.Atoi(serviceUser.Gid)
		if err != nil {
			return fmt.Errorf("unable to convert gid: %s to int: %s", serviceUser.Gid, err)
		}
	}

	dirPath := filepath.Dir(file)
	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}

	// Change owner of directory containing client ID file
	if serviceUser != nil {
		err := os.Chown(file, uid, gid)
		if err != nil {
			return fmt.Errorf("unable to chance owner of %s to service user: %s, %s",
				dirPath, serviceUser.Username, err)
		}
	}

	if err := os.WriteFile(file, data, 0600); err != nil {
		return fmt.Errorf("cannot write file: %w", err)
	}

	// Change owner of client-id
	if serviceUser != nil {
		err := os.Chown(file, uid, gid)
		if err != nil {
			// When it wasn't possible to change owner of the file, then delete the file first
			_ = os.Remove(file)
			return fmt.Errorf("unable to chance owner of %s to service user: %s, %s",
				file, serviceUser.Username, err)
		}
	}

	return nil

}

// getClientID reads data from the client ID file.
func getClientID(file string) ([]byte, error) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	return data, nil
}
