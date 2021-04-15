package yggdrasil

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// CanonicalFacts contain several identification strings that collectively
// combine to uniquely identify a system to the platform services.
type CanonicalFacts struct {
	InsightsID            string   `json:"insights_id"`
	MachineID             string   `json:"machine_id"`
	BIOSUUID              string   `json:"bios_uuid"`
	SubscriptionManagerID string   `json:"subscription_manager_id"`
	IPAddresses           []string `json:"ip_addresses"`
	MACAddresses          []string `json:"mac_addresses"`
	FQDN                  string   `json:"fqdn"`
}

// CanonicalFactsFromMap creates a CanonicalFacts struct from the key-value
// pairs in a map.
func CanonicalFactsFromMap(m map[string]interface{}) (*CanonicalFacts, error) {
	var facts CanonicalFacts

	if val, ok := m["insights_id"]; ok {
		switch val := val.(type) {
		case string:
			facts.InsightsID = val
		default:
			return nil, &InvalidValueTypeError{key: "insights_id", val: val}
		}
	}

	if val, ok := m["machine_id"]; ok {
		switch val := val.(type) {
		case string:
			facts.MachineID = val
		default:
			return nil, &InvalidValueTypeError{key: "machine_id", val: val}
		}
	}

	if val, ok := m["bios_uuid"]; ok {
		switch val := val.(type) {
		case string:
			facts.BIOSUUID = val
		default:
			return nil, &InvalidValueTypeError{key: "bios_uuid", val: val}
		}
	}

	if val, ok := m["subscription_manager_id"]; ok {
		switch val := val.(type) {
		case string:
			facts.SubscriptionManagerID = val
		default:
			return nil, &InvalidValueTypeError{key: "subscription_manager_id", val: val}
		}
	}

	if val, ok := m["ip_addresses"]; ok {
		switch val := val.(type) {
		case []string:
			facts.IPAddresses = val
		default:
			return nil, &InvalidValueTypeError{key: "ip_addresses", val: val}
		}
	}

	if val, ok := m["fqdn"]; ok {
		switch val := val.(type) {
		case string:
			facts.FQDN = val
		default:
			return nil, &InvalidValueTypeError{key: "fqdn", val: val}
		}
	}

	if val, ok := m["mac_addresses"]; ok {
		switch val := val.(type) {
		case []string:
			facts.MACAddresses = val
		default:
			return nil, &InvalidValueTypeError{key: "mac_addresses", val: val}
		}
	}

	return &facts, nil
}

// GetCanonicalFacts attempts to construct a CanonicalFacts struct by collecting
// data from the localhost.
func GetCanonicalFacts() (*CanonicalFacts, error) {
	var facts CanonicalFacts
	var err error

	if _, err := os.Stat("/etc/insights-client/machine-id"); !os.IsNotExist(err) {
		insightsID, err := readFile("/etc/insights-client/machine-id")
		if err != nil {
			return nil, err
		}
		facts.InsightsID = insightsID
	}

	machineID, err := readFile("/etc/machine-id")
	if err != nil {
		return nil, err
	}
	facts.MachineID, err = toUUIDv4(machineID)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat("/sys/devices/virtual/dmi/id/product_uuid"); !os.IsNotExist(err) {
		BIOSUUID, err := readFile("/sys/devices/virtual/dmi/id/product_uuid")
		if err != nil {
			return nil, err
		}
		facts.BIOSUUID = BIOSUUID
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
		if iface.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			switch addr := addr.(type) {
			case *net.IPNet:
				netAddr := addr
				if netAddr.IP.To4() == nil {
					continue
				}
				addresses = append(addresses, netAddr.IP.String())
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
	sort.Slice(ifaces, func(i, j int) bool {
		return ifaces[i].Name < ifaces[j].Name
	})
	for _, iface := range ifaces {
		addr := iface.HardwareAddr.String()
		if addr == "" {
			addr = "00:00:00:00:00:00"
		}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

// toUUIDv4 parses id as a UUID and returns the "dashed" notation string format.
func toUUIDv4(id string) (string, error) {
	UUID, err := uuid.Parse(id)
	if err != nil {
		return "", err
	}
	return UUID.String(), nil
}
