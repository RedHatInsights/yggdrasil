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
	"github.com/redhatinsights/yggdrasil/internal/config"
	"github.com/redhatinsights/yggdrasil/internal/transport"
)

type Client struct {
	t transport.Transporter
	d *dispatcher
}

// NewClient creates a new Client configured with dispatcher and transporter.
func NewClient(dispatcher *dispatcher, transporter transport.Transporter) *Client {
	return &Client{
		d: dispatcher,
		t: transporter,
	}
}

// Connect starts a goroutine receiving values from the client's dispatcher and
// transmits the data using the transporter.
func (c *Client) Connect() error {
	if c.t == nil {
		return fmt.Errorf("cannot connect client: missing transport")
	}

	// start receiving values from the dispatcher and transmit them using the
	// provided transporter.
	go func() {
		for msg := range c.d.sendQ {
			code, metadata, data, err := c.SendDataMessage(&msg, msg.Metadata)
			if err != nil {
				log.Errorf("cannot send data message: %v", err)
				continue
			}
			msg.Resp <- struct {
				Code     int
				Metadata map[string]string
				Data     []byte
			}{
				Code:     code,
				Metadata: metadata,
				Data:     data,
			}
		}
	}()

	// set a transport RxHandlerFunc that calls the client's control and data
	// receive handler functions.
	err := c.t.SetRxHandler(func(addr string, metadata map[string]interface{}, data []byte) error {
		switch addr {
		case "data":
			var message yggdrasil.Data

			if err := json.Unmarshal(data, &message); err != nil {
				return fmt.Errorf("cannot unmarshal data message: %w", err)
			}
			if err := c.ReceiveDataMessage(&message); err != nil {
				return fmt.Errorf("cannot process data message: %w", err)
			}
		case "control":
			var message yggdrasil.Control

			if err := json.Unmarshal(data, &message); err != nil {
				return fmt.Errorf("cannot unmarshal control message: %w", err)
			}
			if err := c.ReceiveControlMessage(&message); err != nil {
				return fmt.Errorf("cannot process control message: %w", err)
			}
		default:
			return fmt.Errorf("unsupported destination type: %v", addr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("cannot set RxHandler: %v", err)
	}

	return c.t.Connect()
}

func (c *Client) SendDataMessage(msg *yggdrasil.Data, metadata map[string]string) (int, map[string]string, []byte, error) {
	return c.sendMessage("data", metadata, msg)
}

func (c *Client) SendConnectionStatusMessage(msg *yggdrasil.ConnectionStatus) (int, map[string]string, []byte, error) {
	code, metadata, data, err := c.sendMessage("control", nil, msg)
	if err != nil {
		return -1, nil, nil, err
	}
	return code, metadata, data, nil
}

func (c *Client) SendEventMessage(msg *yggdrasil.Event) (int, map[string]string, []byte, error) {
	code, metadata, data, err := c.sendMessage("control", nil, msg)
	if err != nil {
		return -1, nil, nil, err
	}
	return code, metadata, data, nil
}

// sendMessage marshals msg as data and transmits it via the transport.
func (c *Client) sendMessage(dest string, metadata map[string]string, msg interface{}) (int, map[string]string, []byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("cannot marshal message: %w", err)
	}
	return c.t.Tx(dest, metadata, data)
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
			if _, _, _, err := c.t.Tx("control", nil, data); err != nil {
				return fmt.Errorf("cannot send data: %w", err)
			}
		case yggdrasil.CommandNameDisconnect:
			log.Info("disconnecting...")
			c.d.DisconnectWorkers()
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

// ConnectionStatus creates a connection-status message using the current state
// of the client.
func (c *Client) ConnectionStatus() (*yggdrasil.ConnectionStatus, error) {
	facts, err := yggdrasil.GetCanonicalFacts(config.DefaultConfig.CertFile)
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
