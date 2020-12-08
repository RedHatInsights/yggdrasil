package yggdrasil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/godbus/dbus/v5"
	"golang.org/x/crypto/openpgp"
)

// A SignalRouter receives messages over an MQTT topic and routes them to a dispatcher
// for execution. It receives completed messages and sends them back via an
// HTTP return URL or an MQTT message.
type SignalRouter struct {
	consumerID string
	httpClient *HTTPClient
	mqttClient mqtt.Client
	keyring    openpgp.KeyRing
	logger     *log.Logger
	out        chan Assignment
	in         chan Assignment
	work       map[string]*Work
	lock       sync.RWMutex
}

// NewSignalRouter creates a new router, configured with an appropriate HTTP client
// and MQTT client for communcation with remote services.
func NewSignalRouter(brokers []string, armoredPublicKeyData []byte, in, out chan Assignment) (*SignalRouter, error) {
	logger := log.New(log.Writer(), fmt.Sprintf("%v[router] ", log.Prefix()), log.Flags(), log.CurrentLevel())

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var consumerID string
	{
		object := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Consumer")
		if err := object.Call("com.redhat.RHSM1.Consumer.GetUuid", dbus.Flags(0), "").Store(&consumerID); err != nil {
			return nil, err
		}
	}

	var consumerCertDir string
	{
		object := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
		if err := object.Call("com.redhat.RHSM1.Config.Get", dbus.Flags(0), "rhsm.consumercertdir", "").Store(&consumerCertDir); err != nil {
			return nil, err
		}
	}

	httpClient, err := NewHTTPClientCertAuth(filepath.Join(consumerCertDir, "cert.pem"), filepath.Join(consumerCertDir, "key.pem"), "")
	if err != nil {
		return nil, err
	}

	opts := mqtt.NewClientOptions()
	for _, broker := range brokers {
		opts.AddBroker(broker)
	}
	mqttClient := mqtt.NewClient(opts)

	var entityList openpgp.KeyRing
	if len(armoredPublicKeyData) > 0 {
		reader := bytes.NewReader(armoredPublicKeyData)
		entityList, err = openpgp.ReadArmoredKeyRing(reader)
		if err != nil {
			return nil, err
		}
	}

	return &SignalRouter{
		consumerID: consumerID,
		httpClient: httpClient,
		mqttClient: mqttClient,
		keyring:    entityList,
		logger:     logger,
		out:        out,
		in:         in,
		work:       make(map[string]*Work),
	}, nil
}

// Connect connects to the MQTT broker.
func (r *SignalRouter) Connect() error {
	if token := r.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	r.logger.Trace("connected to broker")
	return nil
}

// Publish sends a message consisting of bytes to the inbound topic.
func (r *SignalRouter) Publish(d []byte) error {
	topic := fmt.Sprintf("redhat/insights/in/%v", r.consumerID)
	if token := r.mqttClient.Publish(topic, byte(0), false, d); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	r.logger.Tracef("published %#v to %v", d, topic)
	return nil
}

// Subscribe opens a subscription on the outbound topic and registers a message
// handler.
//
// The handler unmarshals messages and constructs work assignments, dispatches
// them to a dispatch queue, and runs a goroutine to wait for incoming completed
// work assignments.
func (r *SignalRouter) Subscribe() error {
	go func() {
		for {
			assignment := <-r.in
			r.logger.Trace("received completed assignment: %#v", assignment)

			r.lock.RLock()
			work := r.work[assignment.id]
			r.lock.RUnlock()

			resp, err := r.httpClient.Post(work.ReturnURL, bytes.NewReader(assignment.payload))
			if err != nil {
				r.logger.Error(err)
				continue
			}
			r.logger.Tracef("sent %#v to %v", assignment.payload, work.ReturnURL)

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				r.logger.Error(err)
				continue
			}
			defer resp.Body.Close()

			switch {
			case resp.StatusCode >= 400:
				r.logger.Error(&APIResponseError{Code: resp.StatusCode, body: string(body)})
				continue
			}

			r.lock.Lock()
			delete(r.work, assignment.id)
			r.lock.Unlock()
			r.logger.Tracef("remove assignment: %v", assignment.id)
		}
	}()

	topic := fmt.Sprintf("redhat/insights/out/%v", r.consumerID)
	r.logger.Tracef("subscribing to topic: %v", topic)
	if token := r.mqttClient.Subscribe(topic, byte(0), func(_ mqtt.Client, msg mqtt.Message) {
		var s Signal
		if err := json.Unmarshal(msg.Payload(), &s); err != nil {
			r.logger.Error(err)
			return
		}
		r.logger.Tracef("received signal: %#v", s)

		data, err := json.Marshal(s.Payload)
		if err != nil {
			r.logger.Error(err)
			return
		}

		switch s.Type {
		case "work":
			var w Work
			if err := json.Unmarshal(data, &w); err != nil {
				r.logger.Error(err)
				return
			}
			r.logger.Tracef("found work signal: %#v", w)

			resp, err := r.httpClient.Get(w.PayloadURL)
			if err != nil {
				log.Error(err)
				return
			}
			r.logger.Tracef("got payload from: %v", w.PayloadURL)

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Error(err)
				return
			}

			if r.keyring != nil {
				resp, err := r.httpClient.Get(w.PayloadURL + "/asc")
				if err != nil {
					log.Error(err)
					return
				}

				signedBytes := bytes.NewReader(body)
				_, err = openpgp.CheckArmoredDetachedSignature(r.keyring, signedBytes, resp.Body)
				if err != nil {
					log.Error(err)
					return
				}
			}

			assignment := Assignment{
				id:      s.MessageID,
				handler: w.Handler,
				payload: body,
			}

			r.out <- assignment
			r.logger.Tracef("routed assignment: %v", assignment.id)

			r.lock.Lock()
			r.work[s.MessageID] = &w
			r.lock.Unlock()
			r.logger.Tracef("stored work: %#v", w)
		}

	}); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}
