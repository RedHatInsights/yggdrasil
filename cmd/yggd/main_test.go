package main

import (
	"testing"
)

func TestDetectProtocolFromURL(t *testing.T) {
	tests := []struct {
		name       string
		serverURLs []string
		wantProto  string
		wantURL    string
		wantErr    bool
	}{
		{
			name:       "Single HTTP URL",
			serverURLs: []string{"http://example.com"},
			wantProto:  "http",
			wantURL:    "http://example.com",
			wantErr:    false,
		},
		{
			name:       "Single HTTPS URL",
			serverURLs: []string{"https://example.com"},
			wantProto:  "http",
			wantURL:    "https://example.com",
			wantErr:    false,
		},
		{
			name:       "Single MQTT URL",
			serverURLs: []string{"mqtt://example.com"},
			wantProto:  "mqtt",
			wantURL:    "mqtt://example.com",
			wantErr:    false,
		},
		{
			name:       "Single MQTTS URL",
			serverURLs: []string{"mqtts://example.com"},
			wantProto:  "mqtt",
			wantURL:    "mqtts://example.com",
			wantErr:    false,
		},
		{
			name:       "Unsupported Protocol",
			serverURLs: []string{"ftp://example.com"},
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
		{
			name:       "Empty URL List",
			serverURLs: []string{},
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
		{
			name:       "Nil URL List",
			serverURLs: nil,
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
		{
			name:       "Invalid URL Format",
			serverURLs: []string{":://invalid-url"},
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
		{
			name:       "Mixed Invalid and Valid URLs",
			serverURLs: []string{"ftp://example.com", "http://valid.com"},
			wantProto:  "http",
			wantURL:    "http://valid.com",
			wantErr:    false,
		},
		{
			name:       "Mixed Protocols HTTP First",
			serverURLs: []string{"http://example.com", "mqtt://broker.com"},
			wantProto:  "http",
			wantURL:    "http://example.com",
			wantErr:    false,
		},
		{
			name:       "Mixed Protocols MQTT First",
			serverURLs: []string{"mqtt://broker.com", "https://example.com"},
			wantProto:  "mqtt",
			wantURL:    "mqtt://broker.com",
			wantErr:    false,
		},
		{
			name:       "Mixed Protocols HTTPS and MQTTS",
			serverURLs: []string{"https://secure.example.com", "mqtts://secure.broker.com"},
			wantProto:  "http",
			wantURL:    "https://secure.example.com",
			wantErr:    false,
		},
		{
			name:       "Mixed Protocols MQTTS First",
			serverURLs: []string{"mqtts://secure.broker.com", "http://example.com"},
			wantProto:  "mqtt",
			wantURL:    "mqtts://secure.broker.com",
			wantErr:    false,
		},
		{
			name: "Multiple HTTP URLs",
			serverURLs: []string{
				"http://first.com",
				"https://second.com",
				"http://third.com",
			},
			wantProto: "http",
			wantURL:   "http://first.com",
			wantErr:   false,
		},
		{
			name: "Multiple MQTT URLs",
			serverURLs: []string{
				"mqtt://broker1.com",
				"mqtts://broker2.com",
				"mqtt://broker3.com",
			},
			wantProto: "mqtt",
			wantURL:   "mqtt://broker1.com",
			wantErr:   false,
		},
		{
			name: "Mixed Invalid Then HTTP and MQTT",
			serverURLs: []string{
				"ftp://invalid.com",
				"invalid-url",
				"https://valid.com",
				"mqtt://valid.broker.com",
			},
			wantProto: "http",
			wantURL:   "https://valid.com",
			wantErr:   false,
		},
		{
			name: "Mixed Invalid Then MQTT and HTTP",
			serverURLs: []string{
				"ftp://invalid.com",
				"invalid-url",
				"mqtts://valid.broker.com",
				"http://valid.com",
			},
			wantProto: "mqtt",
			wantURL:   "mqtts://valid.broker.com",
			wantErr:   false,
		},
		{
			name:       "Multiple HTTP Variants",
			serverURLs: []string{"http://example.com", "https://secure.example.com"},
			wantProto:  "http",
			wantURL:    "http://example.com",
			wantErr:    false,
		},
		{
			name:       "Multiple MQTT Variants",
			serverURLs: []string{"mqtt://broker.com", "mqtts://secure.broker.com"},
			wantProto:  "mqtt",
			wantURL:    "mqtt://broker.com",
			wantErr:    false,
		},
		{
			name: "Mixed Protocols With Ports",
			serverURLs: []string{
				"https://example.com:8443",
				"mqtt://broker.com:1883",
				"mqtts://secure.broker.com:8883",
			},
			wantProto: "http",
			wantURL:   "https://example.com:8443",
			wantErr:   false,
		},
		{
			name:       "IPv4 HTTP No Port",
			serverURLs: []string{"http://127.0.0.1"},
			wantProto:  "http",
			wantURL:    "http://127.0.0.1",
			wantErr:    false,
		},
		{
			name:       "IPv4 MQTT No Port",
			serverURLs: []string{"mqtt://127.0.0.1"},
			wantProto:  "mqtt",
			wantURL:    "mqtt://127.0.0.1",
			wantErr:    false,
		},
		{
			name:       "IPv4 HTTPS No Port",
			serverURLs: []string{"https://127.0.0.1"},
			wantProto:  "http",
			wantURL:    "https://127.0.0.1",
			wantErr:    false,
		},
		{
			name:       "IPv4 MQTTS No Port",
			serverURLs: []string{"mqtts://127.0.0.1"},
			wantProto:  "mqtt",
			wantURL:    "mqtts://127.0.0.1",
			wantErr:    false,
		},
		{
			name:       "IPv4 Invalid Protocol No Port",
			serverURLs: []string{"ftp://127.0.0.1"},
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
		{
			name:       "IPv4 HTTP With Port",
			serverURLs: []string{"http://127.0.0.1:8080"},
			wantProto:  "http",
			wantURL:    "http://127.0.0.1:8080",
			wantErr:    false,
		},
		{
			name:       "IPv4 MQTT With Port",
			serverURLs: []string{"mqtt://127.0.0.1:1883"},
			wantProto:  "mqtt",
			wantURL:    "mqtt://127.0.0.1:1883",
			wantErr:    false,
		},
		{
			name:       "IPv4 HTTPS With Port",
			serverURLs: []string{"https://127.0.0.1:8443"},
			wantProto:  "http",
			wantURL:    "https://127.0.0.1:8443",
			wantErr:    false,
		},
		{
			name:       "IPv4 MQTTS With Port",
			serverURLs: []string{"mqtts://127.0.0.1:8883"},
			wantProto:  "mqtt",
			wantURL:    "mqtts://127.0.0.1:8883",
			wantErr:    false,
		},
		{
			name:       "IPv4 Invalid Protocol With Port",
			serverURLs: []string{"ftp://127.0.0.1:21"},
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
		{
			name:       "IPv6 HTTP No Port",
			serverURLs: []string{"http://[::1]"},
			wantProto:  "http",
			wantURL:    "http://[::1]",
			wantErr:    false,
		},
		{
			name:       "IPv6 MQTT No Port",
			serverURLs: []string{"mqtt://[::1]"},
			wantProto:  "mqtt",
			wantURL:    "mqtt://[::1]",
			wantErr:    false,
		},
		{
			name:       "IPv6 HTTPS No Port",
			serverURLs: []string{"https://[2001:db8::1]"},
			wantProto:  "http",
			wantURL:    "https://[2001:db8::1]",
			wantErr:    false,
		},
		{
			name:       "IPv6 MQTTS No Port",
			serverURLs: []string{"mqtts://[2001:db8::1]"},
			wantProto:  "mqtt",
			wantURL:    "mqtts://[2001:db8::1]",
			wantErr:    false,
		},
		{
			name:       "IPv6 Invalid Protocol No Port",
			serverURLs: []string{"ftp://[::1]"},
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
		{
			name:       "IPv6 HTTP With Port",
			serverURLs: []string{"http://[::1]:8080"},
			wantProto:  "http",
			wantURL:    "http://[::1]:8080",
			wantErr:    false,
		},
		{
			name:       "IPv6 MQTT With Port",
			serverURLs: []string{"mqtt://[2001:db8::1]:1883"},
			wantProto:  "mqtt",
			wantURL:    "mqtt://[2001:db8::1]:1883",
			wantErr:    false,
		},
		{
			name:       "IPv6 HTTPS With Port",
			serverURLs: []string{"https://[::1]:8443"},
			wantProto:  "http",
			wantURL:    "https://[::1]:8443",
			wantErr:    false,
		},
		{
			name:       "IPv6 MQTTS With Port",
			serverURLs: []string{"mqtts://[2001:db8::1]:8883"},
			wantProto:  "mqtt",
			wantURL:    "mqtts://[2001:db8::1]:8883",
			wantErr:    false,
		},
		{
			name:       "IPv6 Invalid Protocol With Port",
			serverURLs: []string{"ftp://[::1]:21"},
			wantProto:  "",
			wantURL:    "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProto, gotURL, err := detectProtocolFromURL(tt.serverURLs)
			if err != nil && !tt.wantErr {
				t.Errorf("detectProtocolFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotProto != tt.wantProto {
				t.Errorf("detectProtocolFromURL() gotProto = %v, want %v", gotProto, tt.wantProto)
			}
			if gotURL != tt.wantURL {
				t.Errorf("detectProtocolFromURL() gotURL = %v, want %v", gotURL, tt.wantURL)
			}
		})
	}
}
