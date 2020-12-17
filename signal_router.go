package yggdrasil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/godbus/dbus/v5"
)

// Signal is a message sent and received over MQTT.
type Signal struct {
	Type       string          `json:"type"`
	MessageID  string          `json:"message_id"`
	ClientUUID string          `json:"client_uuid"`
	Version    uint            `json:"version"`
	Sent       time.Time       `json:"sent"`
	Payload    json.RawMessage `json:"payload"`
}

// PayloadHandshake is a specified type of payload included in signals where the
// "Type" field is "handshake".
type PayloadHandshake struct {
	Type  string         `json:"type"`
	Facts CanonicalFacts `json:"facts"`
}

// PayloadResponse is a specified type of payload included in signals where the
// "Type" field is "response".
type PayloadResponse struct {
	Result        string `json:"result"`
	ResultDetails string `json:"result_details"`
}

// PayloadWork is a specific type of payload included in signals where the
// "Type" field is "work".
type PayloadWork struct {
	Handler    string `json:"handler"`
	PayloadURL string `json:"payload_url"`
	ReturnURL  string `json:"return_url"`
}

// A SignalRouter receives messages over an MQTT topic and routes them to a dispatcher
// for execution. It receives completed messages and sends them back via an
// HTTP return URL or an MQTT message.
type SignalRouter struct {
	consumerID string
	httpClient *HTTPClient
	mqttClient mqtt.Client
	logger     *log.Logger
	out        chan *Assignment
	in         chan *Assignment
	work       map[string]*PayloadWork
	lock       sync.RWMutex
}

// NewSignalRouter creates a new router, configured with an appropriate HTTP client
// and MQTT client for communcation with remote services.
func NewSignalRouter(brokers []string, armoredPublicKeyData []byte, in, out chan *Assignment) (*SignalRouter, error) {
	logger := log.New(log.Writer(), fmt.Sprintf("%v[router] ", log.Prefix()), log.Flags(), log.CurrentLevel())

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

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

	return &SignalRouter{
		consumerID: consumerID,
		httpClient: httpClient,
		mqttClient: mqttClient,
		logger:     logger,
		out:        out,
		in:         in,
		work:       make(map[string]*PayloadWork),
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
	r.logger.Tracef("published %#v to %v", string(d), topic)
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
			r.logger.Trace("received assignment: %#v", assignment)

			r.lock.RLock()
			work := r.work[assignment.ID]
			r.lock.RUnlock()

			req, err := http.NewRequest(http.MethodPost, work.ReturnURL, bytes.NewReader(assignment.Data))
			if err != nil {
				r.logger.Error(err)
				continue
			}
			for k, v := range assignment.Headers {
				req.Header.Add(k, strings.TrimSpace(v))
			}
			resp, err := r.httpClient.Do(req)
			if err != nil {
				r.logger.Error(err)
				continue
			}
			r.logger.Tracef("sent %#v to %v", assignment.Data, work.ReturnURL)

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

			if assignment.Complete {
				r.lock.Lock()
				delete(r.work, assignment.ID)
				r.lock.Unlock()
				r.logger.Tracef("remove assignment: %v", assignment.ID)
			}
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

		switch s.Type {
		case "work":
			var w PayloadWork
			if err := json.Unmarshal([]byte(s.Payload), &w); err != nil {
				r.logger.Error(err)
				return
			}
			r.logger.Tracef("received work signal: %#v", w)

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

			assignment := &Assignment{
				ID:      s.MessageID,
				Handler: w.Handler,
				Data:    body,
			}

			r.out <- assignment
			r.logger.Tracef("routed assignment: %v", assignment.ID)

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
