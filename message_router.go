package yggdrasil

import (
	"encoding/json"
	"fmt"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

const (
	// SignalMessageRecv is emitted when an MQTT message is received and
	// unmarshaled. The value emitted on the channel is a yggdrasil.Message.
	SignalMessageRecv = "message-recv"

	// SignalMessageSend is emitted when an MQTT message is marshaled and
	// published. The value emitted on the channel is a yggdrasil.Message.
	SignalMessageSend = "message-send"
)

// Message is a message sent and received over MQTT.
type Message struct {
	Type       string          `json:"type"`
	MessageID  string          `json:"message_id"`
	ClientUUID string          `json:"client_uuid"`
	Version    uint            `json:"version"`
	Sent       time.Time       `json:"sent"`
	Payload    json.RawMessage `json:"payload"`
}

// PayloadHandshake is a specified type of payload included in messages where
// the "Type" field is "handshake".
type PayloadHandshake struct {
	Type  string         `json:"type"`
	Facts CanonicalFacts `json:"facts"`
}

// PayloadResponse is a specified type of payload included in messages where the
// "Type" field is "response".
type PayloadResponse struct {
	Result        string `json:"result"`
	ResultDetails string `json:"result_details"`
}

// PayloadWork is a specific type of payload included in messages where the
// "Type" field is "work".
type PayloadWork struct {
	Handler    string `json:"handler"`
	PayloadURL string `json:"payload_url"`
	ReturnURL  string `json:"return_url"`
}

// A MessageRouter receives messages over an MQTT topic and emits events when
// they are decoded.
type MessageRouter struct {
	logger     *log.Logger
	sig        signalEmitter
	client     mqtt.Client
	consumerID string
}

// NewMessageRouter creates a new router, configured with an MQTT client for
// communcation with remote services.
func NewMessageRouter(brokers []string) (*MessageRouter, error) {
	m := new(MessageRouter)
	m.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), m), log.Flags(), log.CurrentLevel())

	opts := mqtt.NewClientOptions()
	for _, broker := range brokers {
		opts.AddBroker(broker)
	}
	m.client = mqtt.NewClient(opts)

	consumerID, err := getConsumerUUID()
	if err != nil {
		return nil, err
	}
	m.consumerID = consumerID

	return m, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (m *MessageRouter) Connect(name string) <-chan interface{} {
	return m.sig.connect(name, 1)
}

// ConnectClient connects to the MQTT broker.
func (m *MessageRouter) ConnectClient() error {
	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	m.logger.Trace("connected to broker")
	return nil
}

// Publish sends a message consisting of bytes to the inbound topic.
func (m *MessageRouter) Publish(msgType string, d []byte) error {
	topic := fmt.Sprintf("redhat/insights/in/%v", m.consumerID)
	m.logger.Debugf("Publish(%v, %v) -> %v", msgType, string(d), topic)

	msg := Message{
		Type:       msgType,
		MessageID:  uuid.New().String(),
		ClientUUID: m.consumerID,
		Version:    1,
		Sent:       time.Now(),
		Payload:    json.RawMessage(d),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if token := m.client.Publish(topic, byte(0), false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	m.sig.emit(SignalMessageSend, msg)
	m.logger.Debugf("emitted signal \"%v\"", SignalMessageSend)
	m.logger.Tracef("emitted value: %#v", msg)
	return nil
}

// Subscribe opens a subscription on the outbound topic and registers a message
// handler.
//
// The handler unmarshals messages and emits them on the "message-recv" signal.
func (m *MessageRouter) Subscribe() error {
	topic := fmt.Sprintf("redhat/insights/out/%v", m.consumerID)
	m.logger.Debugf("Subscribe(%v)", topic)
	if token := m.client.Subscribe(topic, byte(0), func(_ mqtt.Client, msg mqtt.Message) {
		m.logger.Debugf("MessageHandler(%v)", msg.MessageID())
		var message Message
		if err := json.Unmarshal(msg.Payload(), &message); err != nil {
			m.logger.Error(err)
			return
		}

		m.sig.emit(SignalMessageRecv, message)
		m.logger.Debugf("emitted signal: \"%v\"", SignalMessageRecv)
		m.logger.Tracef("emitted value: %#v", message)
	}); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}
