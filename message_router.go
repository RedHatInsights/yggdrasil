package yggdrasil

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/hashicorp/go-memdb"
)

const (
	// SignalMessageSend is emitted when an MQTT message is marshaled and
	// published. The value emitted on the channel is the message ID on the
	// form of a UUIDv4-formatted string.
	SignalMessageSend = "message-send"

	// SignalDataRecv is emitted when an MQTT message is received over the data
	// topic. The value emitted on the channel is the "MessageID" in the form of
	// a UUIDv4-formatted string.
	SignalDataRecv = "data-recv"

	// SignalClientConnect is emitted when the MQTT client is successfully
	// connected. The value emitted on the channel is a bool.
	SignalClientConnect = "client-connect"

	// SignalTopicSubscribe is emitted when the MQTT client successfully
	// subscribes to its data and control topics. The value emitted on the
	// channel is a bool.
	SignalTopicSubscribe = "topic-subscribe"
)

// A MessageRouter receives messages over an MQTT topic and emits events when
// they are decoded.
type MessageRouter struct {
	logger     *log.Logger
	sig        signalEmitter
	client     mqtt.Client
	consumerID string
	db         *memdb.MemDB
}

// NewMessageRouter creates a new router, configured with an MQTT client for
// communcation with remote services.
func NewMessageRouter(db *memdb.MemDB, brokers []string, certFile, keyFile string) (*MessageRouter, error) {
	m := new(MessageRouter)
	m.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), m), log.Flags(), log.CurrentLevel())

	consumerID, err := readCert(certFile)
	if err != nil {
		return nil, err
	}
	m.consumerID = consumerID

	opts := mqtt.NewClientOptions()
	opts.SetClientID(m.consumerID)

	for _, broker := range brokers {
		opts.AddBroker(broker)
	}

	if certFile != "" && keyFile != "" {
		tlsConfig := &tls.Config{}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

		opts.SetTLSConfig(tlsConfig)
	}

	willMessage := ConnectionStatus{
		Type:       "connection-status",
		MessageID:  uuid.New().String(),
		ResponseTo: "",
		Version:    1,
		Sent:       time.Now(),
		Content: struct {
			CanonicalFacts CanonicalFacts               "json:\"canonical_facts\""
			Dispatchers    map[string]map[string]string "json:\"dispatchers\""
			State          ConnectionState              "json:\"state\""
		}{
			State: ConnectionStateOffline,
		},
	}
	data, err := json.Marshal(&willMessage)
	if err != nil {
		return nil, err
	}
	opts.SetBinaryWill(fmt.Sprintf("%v/%v/control/out", TopicPrefix, consumerID), data, 1, false)
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		m.logger.Errorf("error: unhandled message: %v", string(msg.Payload()))
	})
	opts.SetCleanSession(true)
	opts.SetOrderMatters(false)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		options := c.OptionsReader()
		for _, url := range options.Servers() {
			m.logger.Tracef("connected to broker %v", url)
		}

		// Publish a throwaway message in case the topic does not exist; this is a
		// workaround for the Akamai MQTT broker implementation.
		go m.publishData(0, false, []byte{})

		m.sig.emit(SignalClientConnect, true)
		m.logger.Debugf("emitted signal: \"%v\"", SignalClientConnect)
		m.logger.Tracef("emitted value: %#v", true)

		if err := m.SubscribeAndRoute(); err != nil {
			m.logger.Error(err)
			return
		}

		if err := m.PublishConnectionStatus(); err != nil {
			m.logger.Error(err)
			return
		}

	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		m.logger.Errorf("error: connection lost unexpectedly: %v", e)
	})

	m.client = mqtt.NewClient(opts)

	m.db = db

	return m, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (m *MessageRouter) Connect(name string) <-chan interface{} {
	return m.sig.connect(name, 1)
}

// Disconnect removes and closes the channel from the signal table under name
// for the caller.
func (m *MessageRouter) Disconnect(name string, ch <-chan interface{}) {
	m.sig.disconnect(name, ch)
}

// Close closes all channels that have been assigned to signal listeners.
func (m *MessageRouter) Close() {
	m.sig.close()
}

// ConnectClient connects to the MQTT broker.
func (m *MessageRouter) ConnectClient() error {
	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// PublishConnectionStatus constructs a ConnectionStatus message and publishes
// it to the client status topic.
func (m *MessageRouter) PublishConnectionStatus() error {
	facts, err := GetCanonicalFacts()
	if err != nil {
		return err
	}

	msg := ConnectionStatus{
		Type:      MessageTypeConnectionStatus,
		MessageID: uuid.New().String(),
		Version:   1,
		Sent:      time.Now(),
		Content: struct {
			CanonicalFacts CanonicalFacts               "json:\"canonical_facts\""
			Dispatchers    map[string]map[string]string "json:\"dispatchers\""
			State          ConnectionState              "json:\"state\""
		}{
			CanonicalFacts: *facts,
			Dispatchers:    make(map[string]map[string]string),
			State:          "online",
		},
	}

	tx := m.db.Txn(false)
	all, err := tx.Get(tableNameWorker, indexNameID)
	if err != nil {
		return err
	}

	for obj := all.Next(); obj != nil; obj = all.Next() {
		worker := obj.(Worker)
		msg.Content.Dispatchers[worker.handler] = worker.features
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	go m.publishControl(1, false, data)

	m.sig.emit(SignalMessageSend, msg)
	m.logger.Debugf("emitted signal: \"%v\"", SignalMessageSend)
	m.logger.Tracef("emitted value: %#v", msg)

	return nil
}

// SubscribeAndRoute starts two subscription routines; one for the control
// topic and one for the data topic. The message handler routine unmarshals any
// message payloads into the appropriate data type and emits a "message-recv"
// signal.
func (m *MessageRouter) SubscribeAndRoute() error {
	var err error

	err = m.subscribeControl(func(_ mqtt.Client, msg mqtt.Message) {
		m.logger.Debugf("subscribeControlMsgHandler(%+v)", msg)

		var cmd Command

		if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
			m.logger.Error(err)
			return
		}

		m.logger.Tracef("received command %#v", cmd)
		switch cmd.Content.Command {
		case CommandNameDisconnect:
			m.logger.Info("forced disconnection in 500 milliseconds")
			m.client.Disconnect(500)
		case CommandNamePing:
			event := Event{
				Type:       MessageTypeEvent,
				MessageID:  uuid.New().String(),
				ResponseTo: cmd.MessageID,
				Version:    1,
				Sent:       time.Now(),
				Content:    string(EventNamePong),
			}

			data, err := json.Marshal(&event)
			if err != nil {
				m.logger.Error(err)
				return
			}
			go m.publishControl(1, false, data)
		case CommandNameReconnect:
			m.client.Disconnect(500)
			if err := m.ConnectClient(); err != nil {
				m.logger.Error(err)
			}
			if err := m.PublishConnectionStatus(); err != nil {
				m.logger.Error(err)
			}
			if err := m.SubscribeAndRoute(); err != nil {
				m.logger.Error(err)
			}
		}
	})
	if err != nil {
		return err
	}

	err = m.subscribeData(func(_ mqtt.Client, msg mqtt.Message) {
		m.logger.Debugf("subscribeDataMsgHandler(%+v)", msg)

		go m.handleDataMessage(msg.Payload())
	})
	if err != nil {
		return err
	}

	m.sig.emit(SignalTopicSubscribe, true)
	m.logger.Debugf("emitted signal: \"%v\"", SignalTopicSubscribe)
	m.logger.Tracef("emitted value: %#v", true)

	return nil
}

// HandleDataConsumeSignal receives values on the channel, looks up the message
// by ID and worker by handler and publishes a Data message on the data topic
// if the worker does not require detached payloads.
func (m *MessageRouter) HandleDataConsumeSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			var (
				tx  *memdb.Txn
				obj interface{}
				err error
			)

			messageID := e.(string)
			m.logger.Debug("HandleDataConsumeSignal")
			m.logger.Tracef("emitted value: %#v", messageID)

			tx = m.db.Txn(false)
			obj, err = tx.First(tableNameData, indexNameID, messageID)
			if err != nil {
				m.logger.Error(err)
				return
			}
			if obj == nil {
				m.logger.Errorf("no data message with ID %v", messageID)
				return
			}
			dataMessage := obj.(Data)

			tx = m.db.Txn(false)
			obj, err = tx.First(tableNameWorker, indexNameHandler, dataMessage.Directive)
			if err != nil {
				m.logger.Error(err)
				return
			}
			if obj != nil {
				worker := obj.(Worker)
				m.logger.Tracef("found worker %#v", worker)

				if !worker.detachedContent {
					m.logger.Tracef("worker.detachedContent = %v", worker.detachedContent)
					data, err := json.Marshal(dataMessage)
					if err != nil {
						m.logger.Error(err)
						return
					}

					go m.publishData(0, false, data)
				}
			}

			tx = m.db.Txn(true)
			if err := tx.Delete(tableNameData, dataMessage); err != nil {
				m.logger.Error(err)
				tx.Abort()
				return
			}
			tx.Commit()

			if dataMessage.ResponseTo != "" {
				tx := m.db.Txn(false)
				obj, err := tx.First(tableNameData, indexNameID, dataMessage.ResponseTo)
				if err != nil {
					m.logger.Error(err)
					return
				}
				if obj == nil {
					m.logger.Errorf("no original data message with ID %v", messageID)
					return
				}

				tx = m.db.Txn(true)
				if err := tx.Delete(tableNameData, obj); err != nil {
					m.logger.Error(err)
					tx.Abort()
					return
				}
				tx.Commit()
			}
		}()
	}
}

// HandleWorkerUnregisterSignal receives values on the channel, and publishes
// a new ConnectionStatus message.
func (m *MessageRouter) HandleWorkerUnregisterSignal(c <-chan interface{}) {
	for range c {
		m.logger.Debug("HandleWorkerUnregisterSignal")
		if err := m.PublishConnectionStatus(); err != nil {
			m.logger.Error(err)
		}
	}
}

// HandleWorkerRegisterSignal receives values on the channel, and publishes
// a new ConnectionStatus message.
func (m *MessageRouter) HandleWorkerRegisterSignal(c <-chan interface{}) {
	for range c {
		m.logger.Debug("HandleWorkerRegisterSignal")
		if err := m.PublishConnectionStatus(); err != nil {
			m.logger.Error(err)
		}
	}
}

func (m *MessageRouter) handleDataMessage(d []byte) {
	var dataMessage Data

	if err := json.Unmarshal(d, &dataMessage); err != nil {
		m.logger.Errorf("cannot unmarshal data message: %v", err)
		return
	}

	tx := m.db.Txn(true)
	if err := tx.Insert(tableNameData, dataMessage); err != nil {
		m.logger.Error(err)
		tx.Abort()
		return
	}
	tx.Commit()

	m.sig.emit(SignalDataRecv, dataMessage.MessageID)
	m.logger.Debugf("emitted signal \"%v\"", SignalDataRecv)
	m.logger.Tracef("emitted value %#v", dataMessage.MessageID)
}

func (m *MessageRouter) subscribeData(handler func(mqtt.Client, mqtt.Message)) error {
	topic := fmt.Sprintf("%v/%v/data/in", TopicPrefix, m.consumerID)
	m.logger.Debugf("subscribeData(%v)", topic)

	return m.subscribe(topic, 1, handler)
}

func (m *MessageRouter) subscribeControl(handler func(mqtt.Client, mqtt.Message)) error {
	topic := fmt.Sprintf("%v/%v/control/in", TopicPrefix, m.consumerID)
	m.logger.Debugf("subscribeControl(%v)", topic)

	return m.subscribe(topic, 1, handler)
}

func (m *MessageRouter) subscribe(topic string, qos byte, handler func(mqtt.Client, mqtt.Message)) error {
	m.logger.Debugf("subscribe(%v)", topic)

	if token := m.client.Subscribe(topic, qos, handler); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (m *MessageRouter) publishData(qos byte, retained bool, payload []byte) {
	m.publish(fmt.Sprintf("%v/%v/data/out", TopicPrefix, m.consumerID), qos, retained, payload)
}

func (m *MessageRouter) publishControl(qos byte, retained bool, payload []byte) {
	m.publish(fmt.Sprintf("%v/%v/control/out", TopicPrefix, m.consumerID), qos, retained, payload)
}

func (m *MessageRouter) publish(topic string, qos byte, retained bool, payload []byte) {
	m.logger.Debugf("publish(%v, %v, %v, %T)", topic, qos, retained, payload)
	m.logger.Tracef("published payload: %#v", string(payload))
	if token := m.client.Publish(topic, qos, retained, payload); token.Wait() && token.Error() != nil {
		m.logger.Error(token.Error())
	}
}
