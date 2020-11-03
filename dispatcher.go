package yggdrasil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/godbus/dbus/v5"
)

type Dispatcher struct {
	facts      CanonicalFacts
	httpClient HTTPClient
	mqttClient mqtt.Client
}

func NewDispatcher() (*Dispatcher, error) {
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
	opts.AddBroker(BrokerAddr)
	mqttClient := mqtt.NewClient(opts)

	return &Dispatcher{
		facts:      *facts,
		httpClient: *httpClient,
		mqttClient: mqttClient,
	}, nil
}

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

	log.Infof("publishing canonical facts...")
	if token := d.mqttClient.Publish("/in", byte(0), false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (d *Dispatcher) Subscribe() error {
	topic := fmt.Sprintf("/out/%v", d.facts.SubscriptionManagerID)
	log.Infof("subscribing to topic %v...", topic)
	if token := d.mqttClient.Subscribe(topic, byte(0), d.MessageHandler); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (d *Dispatcher) MessageHandler(client mqtt.Client, msg mqtt.Message) {
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

	var executor MessageExecutor
	switch message.Kind {
	case "echo":
		executor = EchoMessageExecutor{
			Text: string(body),
		}
	default:
		log.Error("unknown message kind: %v", message.Kind)
		return
	}

	if err := executor.Run(); err != nil {
		log.Error(err)
		return
	}
}
