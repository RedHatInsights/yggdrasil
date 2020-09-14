package yggdrasil

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// CanonicalFacts contain several identification strings that collectively
// combine to uniquely identify a system to the platform services.
type CanonicalFacts struct {
	InsightsID            string   `json:"insights_id"`
	MachineID             string   `json:"machine_id"`
	BIOSUUID              string   `json:"bios_uuid"`
	SubscriptionManagerID string   `json:"subscription_manager_id"`
	IPAddresses           []string `json:"ip_addresses"`
	FQDN                  string   `json:"fqdn"`
	MACAddresses          []string `json:"mac_addresses"`
}

// GetCanonicalFacts attempts to construct a CanonicalFacts struct by collecting
// data from the localhost.
func GetCanonicalFacts() (*CanonicalFacts, error) {
	var facts CanonicalFacts
	var err error

	facts.InsightsID, err = readFile("/etc/insights-client/machine-id")
	if err != nil {
		return nil, err
	}

	facts.MachineID, err = readFile("/etc/machine-id")
	if err != nil {
		return nil, err
	}

	facts.BIOSUUID, err = readFile("/sys/devices/virtual/dmi/id/product_uuid")
	if err != nil {
		return nil, err
	}

	facts.SubscriptionManagerID, err = readCert("/etc/pki/consumer/cert.pem")
	if err != nil {
		return nil, err
	}

	facts.IPAddresses, err = collectIPAddresses()
	if err != nil {
		return nil, err
	}

	facts.FQDN, err = os.Hostname()
	if err != nil {
		return nil, err
	}

	facts.MACAddresses, err = collectMACAddresses()
	if err != nil {
		return nil, err
	}

	return &facts, nil
}

// readFile reads the contents of filename into a string, trims whitespace,
// and returns the result.
func readFile(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// readCert reads the data in filename, decodes it if necessary, and returns
// the certificate subject CN.
func readCert(filename string) (string, error) {
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

// collectIPAddresses iterates over network interfaces and collects IP
// addresses.
func collectIPAddresses() ([]string, error) {
	addresses := make([]string, 0)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			ip := net.ParseIP(addr.String())
			if !ip.IsLinkLocalUnicast() {
				addresses = append(addresses, addr.String())
			}
		}
	}

	return addresses, nil
}

// collectMACAddresses iterates over network interfaces and collects hardware
// addresses.
func collectMACAddresses() ([]string, error) {
	addresses := make([]string, 0)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		addr := iface.HardwareAddr.String()
		if len(addr) > 0 {
			addresses = append(addresses, addr)
		}
	}
	return addresses, nil
}
