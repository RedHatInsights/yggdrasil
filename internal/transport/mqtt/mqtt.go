package mqtt

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/transport"
	"time"
)

type Transport struct {
	ClientID   string
	MqttClient mqtt.Client
}

func NewMQTTTransport(ClientID string, brokers []string, tlsConfig *tls.Config, controlHandler transport.CommandHandler, dataHandler transport.DataHandler) (*Transport, error) {
	t := Transport{
		ClientID: ClientID,
	}
	// Create and configure MQTT client
	mqttClientOpts := mqtt.NewClientOptions()
	for _, broker := range brokers {
		mqttClientOpts.AddBroker(broker)
	}
	mqttClientOpts.SetClientID(ClientID)
	mqttClientOpts.SetTLSConfig(tlsConfig)
	mqttClientOpts.SetCleanSession(true)
	mqttClientOpts.SetOnConnectHandler(func(client mqtt.Client) {
		opts := client.OptionsReader()
		for _, url := range opts.Servers() {
			log.Tracef("connected to broker: %v", url)
		}

		// Publish a throwaway message in case the topic does not exist;
		// this is a workaround for the Akamai MQTT broker implementation.
		go func() {
			topic := fmt.Sprintf("%v/%v/data/out", yggdrasil.TopicPrefix, ClientID)
			client.Publish(topic, 0, false, []byte{})
		}()

		var topic string
		topic = fmt.Sprintf("%v/%v/data/in", yggdrasil.TopicPrefix, t.ClientID)
		client.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
			go t.handleDataMessage(m, dataHandler)
		})
		log.Tracef("subscribed to topic: %v", topic)

		topic = fmt.Sprintf("%v/%v/control/in", yggdrasil.TopicPrefix, t.ClientID)
		client.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
			go t.handleControlMessage(m, controlHandler)
		})
		log.Tracef("subscribed to topic: %v", topic)

		go transport.PublishConnectionStatus(&t, map[string]map[string]string{})
	})
	mqttClientOpts.SetDefaultPublishHandler(func(c mqtt.Client, m mqtt.Message) {
		log.Errorf("unhandled message: %v", string(m.Payload()))
	})
	mqttClientOpts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
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
	mqttClientOpts.SetBinaryWill(fmt.Sprintf("%v/%v/control/out", yggdrasil.TopicPrefix, ClientID), data, 1, false)

	t.MqttClient = mqtt.NewClient(mqttClientOpts)

	return &t, nil
}

func (t *Transport) Start() error {
	if token := t.MqttClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("cannot connect to broker: %w", token.Error())
	}
	return nil
}

func (t *Transport) SendData(data yggdrasil.Data) error {
	topic := fmt.Sprintf("%v/%v/data/out", yggdrasil.TopicPrefix, t.ClientID)

	d, err := json.Marshal(data)
	if err != nil {
		log.Errorf("cannot marshal message to JSON: %v", err)
		return err
	}

	if token := t.MqttClient.Publish(topic, 1, false, d); token.Wait() && token.Error() != nil {
		log.Errorf("failed to publish message: %v", token.Error())
		return token.Error()
	}
	log.Debugf("published message %v to topic %v", data.MessageID, topic)
	log.Tracef("message: %+v", data)
	return nil
}

func (t *Transport) SendControl(ctrlMsg interface{}) error {
	topic := fmt.Sprintf("%v/%v/control/out", yggdrasil.TopicPrefix, t.ClientID)

	data, err := json.Marshal(ctrlMsg)
	if err != nil {
		log.Errorf("cannot marshal message to JSON: %v", err)
		return err
	}

	if token := t.MqttClient.Publish(topic, 1, false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (t *Transport) handleDataMessage(msg mqtt.Message, handler transport.DataHandler) {
	log.Debugf("received a message %s on topic %v", msg.MessageID(), msg.Topic())
	handler(msg.Payload())
}

func (t *Transport) handleControlMessage(msg mqtt.Message, handler transport.CommandHandler) {
	log.Debugf("received a message %s on topic %v", msg.MessageID(), msg.Topic())
	handler(msg.Payload(), t)
}

func (t *Transport) Disconnect(quiesce uint) {
	t.MqttClient.Disconnect(quiesce)
}
