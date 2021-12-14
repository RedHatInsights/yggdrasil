package yggdrasil

import (
	"encoding/json"
	"time"
)

// MessageType represents accepted values in the "type" field of messages.
type MessageType string

// The supported message types.
const (
	MessageTypeConnectionStatus MessageType = "connection-status"
	MessageTypeCommand          MessageType = "command"
	MessageTypeEvent            MessageType = "event"
	MessageTypeData             MessageType = "data"
)

// ConnectionState represents accepted values for the "state" field of
// ConnectionStatus messages.
type ConnectionState string

const (
	// ConnectionStateOnline indicates a client is online and subscribing to
	// topics.
	ConnectionStateOnline ConnectionState = "online"

	// ConnectionStateOffline indicates a client is no longer online.
	ConnectionStateOffline ConnectionState = "offline"
)

// CommandName represents accepted values for the "command" field of Command
// messages.
type CommandName string

const (
	// CommandNameReconnect instructs a client to temporarily disconnect and
	// reconnect to the broker.
	CommandNameReconnect CommandName = "reconnect"

	// CommandNamePing instructs a client to respond with a "pong" event.
	CommandNamePing CommandName = "ping"

	// CommandNameDisconnect instructs a client to permanently disconnect.
	CommandNameDisconnect CommandName = "disconnect"
)

// EventName represents accepted values for the "event" field of an Event
// message.
type EventName string

const (
	// EventNameDisconnect informs the server that the client will disconnect.
	EventNameDisconnect EventName = "disconnect"

	// EventNamePong informs the server that the client has received a "ping"
	// command.
	EventNamePong EventName = "pong"
)

// A ConnectionStatus message is published by the client when it connects to
// the broker. The message is expected to be published as a retained message
// and its presence is considered an acceptable way to decide whether a client
// is active and functioning normally.
type ConnectionStatus struct {
	Type       MessageType `json:"type"`
	MessageID  string      `json:"message_id"`
	ResponseTo string      `json:"response_to"`
	Version    int         `json:"version"`
	Sent       time.Time   `json:"sent"`
	Content    struct {
		CanonicalFacts CanonicalFacts               `json:"canonical_facts"`
		Dispatchers    map[string]map[string]string `json:"dispatchers"`
		State          ConnectionState              `json:"state"`
		Tags           map[string]string            `json:"tags,omitempty"`
	} `json:"content"`
}

// A Command message is published by the server on the "control" topic when it
// needs to instruct a client to perform an operation.
type Command struct {
	Type       MessageType `json:"type"`
	MessageID  string      `json:"message_id"`
	ResponseTo string      `json:"response_to"`
	Version    int         `json:"version"`
	Sent       time.Time   `json:"sent"`
	Content    struct {
		Command   CommandName       `json:"command"`
		Arguments map[string]string `json:"arguments"`
	} `json:"content"`
}

// An Event message is published by the client on the "control" topic when it
// wishes to inform the server that a notable event occurred.
type Event struct {
	Type       MessageType `json:"type"`
	MessageID  string      `json:"message_id"`
	ResponseTo string      `json:"response_to"`
	Version    int         `json:"version"`
	Sent       time.Time   `json:"sent"`
	Content    string      `json:"content"`
}

type Control struct {
	Type       MessageType     `json:"type"`
	MessageID  string          `json:"message_id"`
	ResponseTo string          `json:"response_to"`
	Version    int             `json:"version"`
	Sent       time.Time       `json:"sent"`
	Content    json.RawMessage `json:"content"`
}

// Data messages are published by both client and server on their respective
// "data" topic. The client consumes Data messages and routes them to an
// appropriate worker based on the "Directive" field.
type Data struct {
	Type       MessageType       `json:"type"`
	MessageID  string            `json:"message_id"`
	ResponseTo string            `json:"response_to"`
	Version    int               `json:"version"`
	Sent       time.Time         `json:"sent"`
	Directive  string            `json:"directive"`
	Metadata   map[string]string `json:"metadata"`
	Content    json.RawMessage   `json:"content"`
}
