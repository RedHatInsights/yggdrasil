package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
)

func handleDataMessage(client mqtt.Client, msg mqtt.Message, sendQ chan<- yggdrasil.Data) {
	log.Debugf("received a message on topic %v", msg.Topic())

	var data yggdrasil.Data
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		log.Errorf("cannot unmarshal data message: %v", err)
		return
	}
	log.Tracef("message: %+v", data)

	sendQ <- data
}

func handleControlMessage(client mqtt.Client, msg mqtt.Message) {
	log.Debugf("received a message on topic %v", msg.Topic())

	var cmd yggdrasil.Command
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		log.Errorf("cannot unmarshal control message: %v", err)
		return
	}

	log.Debugf("received message %v", cmd.MessageID)
	log.Tracef("command: %+v", cmd)
	switch cmd.Content.Command {
	case yggdrasil.CommandNamePing:
		event := yggdrasil.Event{
			Type:       yggdrasil.MessageTypeEvent,
			MessageID:  uuid.New().String(),
			ResponseTo: cmd.MessageID,
			Version:    1,
			Sent:       time.Now(),
			Content:    string(yggdrasil.EventNamePong),
		}

		data, err := json.Marshal(&event)
		if err != nil {
			log.Errorf("cannot marshal message to JSON: %v", err)
			return
		}
		topic := fmt.Sprintf("%v/%v/control/out", yggdrasil.TopicPrefix, ClientID)

		if token := client.Publish(topic, 1, false, data); token.Wait() && token.Error() != nil {
			log.Errorf("failed to publish message: %v", token.Error())
		}
	case yggdrasil.CommandNameDisconnect:
		log.Info("disconnecting...")
		client.Disconnect(500)
	case yggdrasil.CommandNameReconnect:
		log.Info("reconnecting...")
		client.Disconnect(500)
		delay, err := strconv.ParseInt(cmd.Content.Arguments["delay"], 10, 64)
		if err != nil {
			log.Errorf("cannot parse data to int: %v", err)
			return
		}
		time.Sleep(time.Duration(delay) * time.Second)

		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Errorf("cannot reconnect to broker: %v", token.Error())
			return
		}
	default:
		log.Warnf("unknown command: %v", cmd.Content.Command)
	}
}

func publishConnectionStatus(c mqtt.Client, dispatchers map[string]map[string]string) {
	facts, err := yggdrasil.GetCanonicalFacts()
	if err != nil {
		log.Errorf("cannot get canonical facts: %v", err)
		return
	}

	tagsFilePath := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "tags.toml")

	var tags map[string]string
	if _, err := os.Stat(tagsFilePath); !os.IsNotExist(err) {
		var err error
		tags, err = readTagsFile(tagsFilePath)
		if err != nil {
			log.Errorf("cannot load tags: %v", err)
		}
	}

	msg := yggdrasil.ConnectionStatus{
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
			CanonicalFacts: *facts,
			Dispatchers:    dispatchers,
			State:          yggdrasil.ConnectionStateOnline,
			Tags:           tags,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Errorf("cannot marshal message to JSON: %v", err)
		return
	}

	topic := fmt.Sprintf("%v/%v/control/out", yggdrasil.TopicPrefix, ClientID)

	if token := c.Publish(topic, 1, false, data); token.Wait() && token.Error() != nil {
		log.Errorf("failed to publish message: %v", token.Error())
	}
	log.Debugf("published message %v to topic %v", msg.MessageID, topic)
	log.Tracef("message: %+v", msg)
}

func publishReceivedData(client mqtt.Client, c <-chan yggdrasil.Data) {
	for d := range c {
		topic := fmt.Sprintf("%v/%v/data/out", yggdrasil.TopicPrefix, ClientID)

		data, err := json.Marshal(d)
		if err != nil {
			log.Errorf("cannot marshal message to JSON: %v", err)
			continue
		}

		if token := client.Publish(topic, 1, false, data); token.Wait() && token.Error() != nil {
			log.Errorf("failed to publish message: %v", token.Error())
		}
		log.Debugf("published message %v to topic %v", d.MessageID, topic)
		log.Tracef("message: %+v", d)
	}
}
