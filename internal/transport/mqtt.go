package transport

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
)

// MQTT is a Transporter that sends and receives data and control
// messages over MQTT by subscribing and publishing to topics on an MQTT broker.
type MQTT struct {
	client         mqtt.Client
	receiveHandler RxHandlerFunc
	opts           *mqtt.ClientOptions
}

// NewMQTTTransport creates a transport suitable for transmitting data over a
// set of MQTT topics.
func NewMQTTTransport(clientID string, broker string, tlsConfig *tls.Config) (*MQTT, error) {
	var t MQTT

	if _, ok := os.LookupEnv("MQTT_DEBUG"); ok {
		mqtt.DEBUG = log.New(os.Stderr, "[MQTT_DEBUG] ", log.LstdFlags, log.LevelDebug)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetTLSConfig(tlsConfig.Clone())
	opts.SetCleanSession(true)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		opts := c.OptionsReader()
		for _, url := range opts.Servers() {
			log.Tracef("connected to broker: %v", url)
		}

		// Publish a throwaway message in case the topic does not exist;
		// this is a workaround for the Akamai MQTT broker implementation.
		go func() {
			topic := fmt.Sprintf("%v/%v/data/out", yggdrasil.PathPrefix, opts.ClientID())
			c.Publish(topic, 0, false, []byte{})
		}()

		var topic string
		topic = fmt.Sprintf("%v/%v/data/in", yggdrasil.PathPrefix, opts.ClientID())
		c.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
			go func() {
				if t.receiveHandler == nil {
					return
				}
				if err := t.receiveHandler("data", nil, m.Payload()); err != nil {
					log.Errorf("cannot receive data message: %v", err)
				}
			}()
		})
		log.Tracef("subscribed to topic: %v", topic)

		topic = fmt.Sprintf("%v/%v/control/in", yggdrasil.PathPrefix, opts.ClientID())
		c.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
			go func() {
				if t.receiveHandler == nil {
					return
				}
				if err := t.receiveHandler("control", nil, m.Payload()); err != nil {
					log.Errorf("cannot receive control message: %v", err)
				}
			}()
		})
		log.Tracef("subscribed to topic: %v", topic)
	})

	opts.SetDefaultPublishHandler(func(c mqtt.Client, m mqtt.Message) {
		log.Errorf("unhandled message: %v", string(m.Payload()))
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		log.Errorf("connection lost unexpectedly: %v", e)
	})

	data, err := json.Marshal(&yggdrasil.ConnectionStatus{
		Type:      yggdrasil.MessageTypeConnectionStatus,
		MessageID: uuid.New().String(),
		Version:   1,
		Sent:      time.Now(),
		Content: struct {
			CanonicalFacts yggdrasil.CanonicalFacts     "json:\"canonical_facts\""
			Dispatchers    map[string]map[string]string "json:\"dispatchers\""
			State          yggdrasil.ConnectionState    "json:\"state\""
			Tags           map[string]string            "json:\"tags,omitempty\""
		}{
			State: yggdrasil.ConnectionStateOffline,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot marshal message to JSON: %w", err)
	}

	opts.SetBinaryWill(fmt.Sprintf("%v/%v/control/out", yggdrasil.PathPrefix, opts.ClientID), data, 1, false)

	t.opts = opts
	t.client = mqtt.NewClient(opts)

	return &t, nil
}

// Connect connects an MQTT client to the configured broker and waits for the
// connection to open.
func (t *MQTT) Connect() error {
	if token := t.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("cannot connect to broker: %w", token.Error())
	}
	return nil
}

// ReloadTLSConfig creates a new MQTT client with the given TLS config, disconnects the
// previous client, and connects the new one.
func (t *MQTT) ReloadTLSConfig(tlsConfig *tls.Config) error {
	// take a reference to the old client in order to disconnect it when the
	// function returns.
	client := t.client
	defer client.Disconnect(1)

	t.opts.SetTLSConfig(tlsConfig.Clone())
	t.client = mqtt.NewClient(t.opts)
	return t.Connect()
}

// Disconnect closes the connection to the MQTT broker, waiting for the
// specified number of milliseconds for work to complete.
func (t *MQTT) Disconnect(quiesce uint) {
	t.client.Disconnect(quiesce)
}

// Tx publishes data to an MQTT topic created by combining client information
// with addr.
func (t *MQTT) Tx(addr string, metadata map[string]string, data []byte) (responseCode int, responseMetadata map[string]string, responseData []byte, err error) {
	opts := t.client.OptionsReader()
	topic := fmt.Sprintf("%v/%v/%v/out", yggdrasil.PathPrefix, opts.ClientID(), addr)

	if token := t.client.Publish(topic, 1, false, data); token.Wait() && token.Error() != nil {
		log.Errorf("failed to publish message: %v", token.Error())
		return TxResponseErr, nil, nil, token.Error()
	}
	log.Debugf("published message to topic %v", topic)
	log.Tracef("message: %v", string(data))

	return TxResponseOK, nil, nil, nil
}

// SetRxHandler stores a reference to f, which is then called whenever data is
// received over the inbound data topic.
func (t *MQTT) SetRxHandler(f RxHandlerFunc) error {
	t.receiveHandler = f
	return nil
}
