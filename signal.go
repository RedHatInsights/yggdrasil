package yggdrasil

import "time"

// Signal is a message sent and received over MQTT.
type Signal struct {
	Type       string    `json:"type"`
	MessageID  string    `json:"message_id"`
	ClientUUID string    `json:"client_uuid"`
	Version    uint      `json:"version"`
	Sent       time.Time `json:"sent"`

	Payload interface{} `json:"payload"`
}

// Work is a specific type of payload included in signals where the "Type" field
// is "work".
type Work struct {
	Handler    string `json:"handler"`
	PayloadURL string `json:"payload_url"`
	ReturnURL  string `json:"return_url"`
}

// Resposne is a specified type of payload included in signals where the "Type"
// field is "response".
type Response struct {
	Result        string `json:"result"`
	ResultDetails string `json:"result_details"`
}

// Handshake is a specified type of payload included in signals where the "Type"
// field is "handshake".
type Handshake struct {
	Type  string         `json:"type"`
	Facts CanonicalFacts `json:"facts"`
}

// SignalType is a concrete type to specify the payload type of a signal.
type SignalType string

const (
	SignalTypeWork      string = "work"
	SignalTypeHandshake string = "handshake"
	SignalTypeResponse  string = "response"
)
