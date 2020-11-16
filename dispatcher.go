package yggdrasil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/godbus/dbus/v5"
	"golang.org/x/crypto/openpgp"
)

// A Dispatcher routes messages received over an MQTT topic to job controllers,
// depending on the message type.
type Dispatcher struct {
	facts      CanonicalFacts
	httpClient HTTPClient
	mqttClient mqtt.Client
	keyring    openpgp.KeyRing
}

// NewDispatcher cretes a new dispatcher, configured with an appropriate HTTP
// client for reporting results.
func NewDispatcher(brokerAddr string, armoredPublicKeyData []byte) (*Dispatcher, error) {
	facts, err := GetCanonicalFacts()
	if err != nil {
		return nil, err
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	object := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
	var consumerCertDir string
	if err := object.Call("com.redhat.RHSM1.Config.Get", dbus.Flags(0), "rhsm.consumercertdir", "").Store(&consumerCertDir); err != nil {
		return nil, err
	}

	httpClient, err := NewHTTPClientCertAuth(filepath.Join(consumerCertDir, "cert.pem"), filepath.Join(consumerCertDir, "key.pem"), "")
	if err != nil {
		return nil, err
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerAddr)
	mqttClient := mqtt.NewClient(opts)

	var entityList openpgp.KeyRing
	if len(armoredPublicKeyData) > 0 {
		reader := bytes.NewReader(armoredPublicKeyData)
		entityList, err = openpgp.ReadArmoredKeyRing(reader)
		if err != nil {
			return nil, err
		}
	}

	return &Dispatcher{
		facts:      *facts,
		httpClient: *httpClient,
		mqttClient: mqttClient,
		keyring:    entityList,
	}, nil
}

// Connect connects to the MQTT broker.
func (d *Dispatcher) Connect() error {
	if token := d.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// PublishFacts publishes canonical facts to an MQTT topic.
func (d *Dispatcher) PublishFacts() error {
	data, err := json.Marshal(d.facts)
	if err != nil {
		return err
	}

	if token := d.mqttClient.Publish("/in", byte(0), false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// Subscribe adds a message handler to a host-specific topic.
func (d *Dispatcher) Subscribe() error {
	topic := fmt.Sprintf("/out/%v", d.facts.SubscriptionManagerID)
	if token := d.mqttClient.Subscribe(topic, byte(0), d.messageHandler); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (d *Dispatcher) messageHandler(client mqtt.Client, msg mqtt.Message) {
	var message struct {
		Kind string    `json:"kind"`
		URL  string    `json:"url"`
		Sent time.Time `json:"sent"`
	}

	if err := json.Unmarshal(msg.Payload(), &message); err != nil {
		log.Error(err)
		return
	}

	resp, err := d.httpClient.Get(message.URL)
	if err != nil {
		log.Error(err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return
	}
	defer resp.Body.Close()

	if d.keyring != nil {
		resp, err := d.httpClient.Get(message.URL + "/asc")
		if err != nil {
			log.Error(err)
			return
		}

		signedBytes := bytes.NewReader(body)
		_, err = openpgp.CheckArmoredDetachedSignature(d.keyring, signedBytes, resp.Body)
		if err != nil {
			log.Error(err)
			return
		}
	}

	switch message.Kind {
	case "playbook":
		var job Job
		if err := json.Unmarshal(body, &job); err != nil {
			log.Error(err)
			log.Debug(string(body))
			return
		}
		controller := PlaybookJobController{
			job:    job,
			client: &d.httpClient,
			url:    message.URL,
		}
		if err := controller.Start(); err != nil {
			log.Error(err)
			return
		}
	default:
		log.Errorf("unsupported message: %+v", message)
	}
}
