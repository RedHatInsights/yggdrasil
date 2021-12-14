package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/transport"
)

type Client struct {
	t transport.Transporter
	d *dispatcher
}

func (c *Client) Connect() error {
	return c.t.Connect()
}

func (c *Client) SendDataMessage(msg *yggdrasil.Data) error {
	return c.sendMessage(msg, "data")
}

func (c *Client) SendConnectionStatusMessage(msg *yggdrasil.ConnectionStatus) error {
	return c.sendMessage(msg, "control")
}

func (c *Client) SendEventMessage(msg *yggdrasil.Event) error {
	return c.sendMessage(msg, "control")
}

func (c *Client) sendMessage(msg interface{}, dest string) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("cannot marshal message: %w", err)
	}
	return c.t.SendData(data, dest)
}

// ReceiveDataMessage sends a value to a channel for dispatching to worker processes.
func (c *Client) ReceiveDataMessage(msg *yggdrasil.Data) error {
	c.d.sendQ <- *msg

	return nil
}

// ReceiveControlMessage unpacks a control message and acts accordingly.
func (c *Client) ReceiveControlMessage(msg *yggdrasil.Control) error {
	switch msg.Type {
	case yggdrasil.MessageTypeCommand:
		var cmd yggdrasil.Command
		if err := json.Unmarshal(msg.Content, &cmd.Content); err != nil {
			return fmt.Errorf("cannot unmarshal command message: %w", err)
		}

		log.Debugf("received message %v", cmd.MessageID)
		log.Tracef("command: %+v", cmd)
		log.Tracef("Control message: %v", cmd)

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

			data, err := json.Marshal(event)
			if err != nil {
				return fmt.Errorf("cannot marshal event: %w", err)
			}
			if err := c.t.SendData(data, "control"); err != nil {
				return fmt.Errorf("cannot send data: %w", err)
			}
		case yggdrasil.CommandNameDisconnect:
			log.Info("disconnecting...")
			c.t.Disconnect(500)
		case yggdrasil.CommandNameReconnect:
			log.Info("reconnecting...")
			c.t.Disconnect(500)
			delay, err := strconv.ParseInt(cmd.Content.Arguments["delay"], 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse data to int: %w", err)
			}
			time.Sleep(time.Duration(delay) * time.Second)

			if err := c.t.Connect(); err != nil {
				return fmt.Errorf("cannot reconnect to broker: %w", err)
			}
		default:
			return fmt.Errorf("unknown command: %v", cmd.Content.Command)
		}
	default:
		return fmt.Errorf("unsupported control message: %v", msg)
	}

	return nil
}

func (c *Client) DataReceiveHandlerFunc(data []byte, dest string) {
	switch dest {
	case "data":
		var message yggdrasil.Data

		if err := json.Unmarshal(data, &message); err != nil {
			log.Errorf("cannot unmarshal data message: %v", err)
			return
		}
		if err := c.ReceiveDataMessage(&message); err != nil {
			log.Errorf("cannot process data message: %v", err)
			return
		}
	case "control":
		var message yggdrasil.Control

		if err := json.Unmarshal(data, &message); err != nil {
			log.Errorf("cannot unmarshal control message: %v", err)
			return
		}
		if err := c.ReceiveControlMessage(&message); err != nil {
			log.Errorf("cannot process control message: %v", err)
			return
		}
	default:
		log.Errorf("unsupported destination type: %v", dest)
		return
	}
}

// ReceiveData receives values from workers via a dispatch receive queue and
// sends them using the configured transport.
func (c *Client) ReceiveData() {
	for msg := range c.d.recvQ {
		if err := c.SendDataMessage(&msg); err != nil {
			log.Errorf("failed to send data message: %v", err)
		}
	}
}

// ConnectionStatus creates a connection-status message using the current state
// of the client.
func (c *Client) ConnectionStatus() (*yggdrasil.ConnectionStatus, error) {
	facts, err := yggdrasil.GetCanonicalFacts()
	if err != nil {
		return nil, fmt.Errorf("cannot get canonical facts: %w", err)
	}

	tagsFilePath := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "tags.toml")

	var tagMap map[string]string
	if _, err := os.Stat(tagsFilePath); !os.IsNotExist(err) {
		var err error
		tagMap, err = readTagsFile(tagsFilePath)
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
			Dispatchers:    c.d.makeDispatchersMap(),
			State:          yggdrasil.ConnectionStateOnline,
			Tags:           tagMap,
		},
	}

	return &msg, nil
}
