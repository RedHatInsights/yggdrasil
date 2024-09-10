package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	internaldbus "github.com/redhatinsights/yggdrasil/dbus"
	"github.com/redhatinsights/yggdrasil/internal/config"
	"github.com/redhatinsights/yggdrasil/internal/constants"
	"github.com/redhatinsights/yggdrasil/internal/messagejournal"
	"github.com/redhatinsights/yggdrasil/internal/tags"
	"github.com/redhatinsights/yggdrasil/internal/transport"
	"github.com/redhatinsights/yggdrasil/internal/work"
	"github.com/redhatinsights/yggdrasil/ipc"
)

type Client struct {
	conn                *dbus.Conn
	transporter         transport.Transporter
	dispatcher          *work.Dispatcher
	prevDispatchersHash atomic.Value
}

// NewClient creates a new Client configured with dispatcher and transporter.
func NewClient(dispatcher *work.Dispatcher, transporter transport.Transporter) *Client {
	return &Client{
		transporter: transporter,
		dispatcher:  dispatcher,
	}
}

// Connect starts a goroutine receiving values from the client's dispatcher and
// transmits the data using the transporter.
func (c *Client) Connect() error {
	if c.transporter == nil {
		return fmt.Errorf("cannot connect client: missing transport")
	}

	// Connect the Dispatcher
	if err := c.dispatcher.Connect(); err != nil {
		return fmt.Errorf("cannot connect dispatcher: %w", err)
	}

	// Start a goroutine that receives values on the 'dispatchers' channel
	// and publishes "connection-status" messages to MQTT.
	go func() {
		for dispatchers := range c.dispatcher.Dispatchers {
			data, err := json.Marshal(dispatchers)
			if err != nil {
				log.Errorf("cannot marshal dispatcher map to JSON: %v", err)
				continue
			}

			// Create a checksum of the dispatchers map. If it's identical
			// to the previous checksum, skip publishing a connection-status
			// message.
			sum := fmt.Sprintf("%x", sha256.Sum256(data))
			oldSum := c.prevDispatchersHash.Load()
			if oldSum != nil {
				if sum == oldSum.(string) {
					continue
				}
			}
			c.prevDispatchersHash.Store(sum)
			go func() {
				msg, err := c.ConnectionStatus()
				if err != nil {
					log.Fatalf("cannot get connection status: %v", err)
				}
				if _, _, _, err = c.SendConnectionStatusMessage(msg); err != nil {
					log.Errorf("cannot send connection status: %v", err)
				}
			}()
		}
	}()

	// start receiving values from the dispatcher and transmit them using the
	// provided transporter.
	go func() {
		for msg := range c.dispatcher.Outbound {
			code, metadata, data, err := c.SendDataMessage(&msg.Data, msg.Data.Metadata)
			if err != nil {
				log.Errorf("cannot send data message: %v", err)
				continue
			}
			msg.Resp <- yggdrasil.Response{
				Code:     code,
				Metadata: metadata,
				Data:     data,
			}
		}
	}()

	// set a transport RxHandlerFunc that calls the client's control and data
	// receive handler functions.
	err := c.transporter.SetRxHandler(
		func(addr string, metadata map[string]interface{}, data []byte) error {
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
		},
	)
	if err != nil {
		return fmt.Errorf("cannot set RxHandler: %v", err)
	}

	_ = c.transporter.SetEventHandler(func(e transport.TransporterEvent) {
		switch e {
		case transport.TransporterEventConnected:
			if err := c.dispatcher.EmitEvent(ipc.DispatcherEventConnectionRestored); err != nil {
				log.Errorf("cannot emit event: %v", err)
			}
		case transport.TransporterEventDisconnected:
			if err := c.dispatcher.EmitEvent(ipc.DispatcherEventUnexpectedDisconnect); err != nil {
				log.Errorf("cannot emit event: %v", err)
			}
		}
	})

	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		log.Debugf("connecting to session bus: %v", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
		c.conn, err = dbus.ConnectSessionBus()
	} else {
		log.Debug("connecting to system bus")
		c.conn, err = dbus.ConnectSystemBus()
	}
	if err != nil {
		return fmt.Errorf("cannot connect to bus: %v", err)
	}

	if err := c.conn.Export(c, "/com/redhat/Yggdrasil1", "com.redhat.Yggdrasil1"); err != nil {
		return fmt.Errorf("cannot export com.redhat.Yggdrasil1 interface: %v", err)
	}

	if err := c.conn.Export(introspect.Introspectable(internaldbus.InterfaceYggdrasil), "/com/redhat/Yggdrasil1", "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("cannot export org.freedesktop.DBus.Introspectable interface: %v", err)
	}

	reply, err := c.conn.RequestName("com.redhat.Yggdrasil1", dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("cannot request name on bus: %v", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("name already taken")
	}
	log.Infof("exported /com/redhat/Yggdrasil1 on bus")

	// Start a goroutine receiving values from the dispatcher's WorkerEvents
	// channel, emitting a D-Bus "WorkerEvent" signal for each.
	go func() {
		for e := range c.dispatcher.WorkerEvents {
			args := []interface{}{e.Worker, e.Name, e.MessageID, e.ResponseTo}
			switch e.Name {
			case ipc.WorkerEventNameWorking:
				args = append(args, e.Data)
			}
			if err := c.conn.Emit("/com/redhat/Yggdrasil1", "com.redhat.Yggdrasil1.WorkerEvent", args...); err != nil {
				log.Errorf("cannot emit event: %v", err)
				continue
			}
			log.Debugf("emitted event: %+v", e)
		}
	}()

	return c.transporter.Connect()
}

// ListWorkers implements the com.redhat.Yggdrasil1.ListWorkers method.
func (c *Client) ListWorkers() (map[string]map[string]string, *dbus.Error) {
	return c.dispatcher.FlattenDispatchers(), nil
}

func (c *Client) GetClientID() (string, *dbus.Error) {
	return config.DefaultConfig.ClientID, nil
}

// MessageJournal implements the com.redhat.Yggdrasil1.MessageJournal method.
func (c *Client) MessageJournal(
	messageID string,
	worker string,
	since string,
	until string,
	persistent bool,
) ([]map[string]string, *dbus.Error) {
	filter := messagejournal.Filter{
		Persistent: persistent,
		MessageID:  messageID,
		Worker:     worker,
		Since:      since,
		Until:      until,
	}

	if c.dispatcher.MessageJournal == nil {
		return nil, dbus.MakeFailedError(fmt.Errorf("message journal is not enabled"))
	}
	journal, err := c.dispatcher.MessageJournal.GetEntries(filter)
	if err != nil {
		return nil, dbus.MakeFailedError(err)
	}
	return journal, nil
}

// Dispatch implements the com.redhat.Yggdrasil1.Dispatch method.
func (c *Client) Dispatch(
	directive string,
	messageID string,
	metadata map[string]string,
	data []byte,
) *dbus.Error {
	msg := yggdrasil.Data{
		Type:       yggdrasil.MessageTypeData,
		MessageID:  messageID,
		ResponseTo: "",
		Version:    1,
		Sent:       time.Now(),
		Directive:  directive,
		Metadata:   metadata,
		Content:    data,
	}
	if err := c.dispatcher.Dispatch(msg); err != nil {
		return work.NewDBusError(
			"com.redhat.Yggdrasil1.Dispatch",
			fmt.Sprintf("cannot dispatch to directive: %v", err),
		)
	}
	return nil
}

func (c *Client) SendDataMessage(
	msg *yggdrasil.Data,
	metadata map[string]string,
) (int, map[string]string, []byte, error) {
	return c.sendMessage("data", metadata, msg)
}

func (c *Client) SendConnectionStatusMessage(
	msg *yggdrasil.ConnectionStatus,
) (int, map[string]string, []byte, error) {
	code, metadata, data, err := c.sendMessage("control", nil, msg)
	if err != nil {
		return transport.TxResponseErr, nil, nil, err
	}
	return code, metadata, data, nil
}

func (c *Client) SendEventMessage(msg *yggdrasil.Event) (int, map[string]string, []byte, error) {
	code, metadata, data, err := c.sendMessage("control", nil, msg)
	if err != nil {
		return transport.TxResponseErr, nil, nil, err
	}
	return code, metadata, data, nil
}

// sendMessage marshals msg as data and transmits it via the transport.
func (c *Client) sendMessage(
	dest string,
	metadata map[string]string,
	msg interface{},
) (int, map[string]string, []byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return transport.TxResponseErr, nil, nil, fmt.Errorf("cannot marshal message: %w", err)
	}
	return c.transporter.Tx(dest, metadata, data)
}

// ReceiveDataMessage sends a value to a channel for dispatching to worker processes.
func (c *Client) ReceiveDataMessage(msg *yggdrasil.Data) error {
	c.dispatcher.Inbound <- *msg

	return nil
}

// ReceiveControlMessage unpacks a control message and acts accordingly.
func (c *Client) ReceiveControlMessage(msg *yggdrasil.Control) error {
	switch msg.Type {
	case yggdrasil.MessageTypeCommand:
		var cmd yggdrasil.Command
		if err := json.Unmarshal(msg.Content, &cmd); err != nil {
			return fmt.Errorf("cannot unmarshal command message: %w", err)
		}

		log.Debugf("received message %v", msg.MessageID)
		log.Tracef("command: %+v", cmd.Command)
		log.Tracef("Control message: %v", msg)

		switch cmd.Command {
		case yggdrasil.CommandNamePing:
			event := yggdrasil.Event{
				Type:       yggdrasil.MessageTypeEvent,
				MessageID:  uuid.New().String(),
				ResponseTo: msg.MessageID,
				Version:    1,
				Sent:       time.Now(),
				Content:    string(yggdrasil.EventNamePong),
			}

			data, err := json.Marshal(event)
			if err != nil {
				return fmt.Errorf("cannot marshal event: %w", err)
			}
			if _, _, _, err := c.transporter.Tx("control", nil, data); err != nil {
				return fmt.Errorf("cannot send data: %w", err)
			}
		case yggdrasil.CommandNameDisconnect:
			log.Info("disconnecting...")
			c.dispatcher.DisconnectWorkers()
			c.transporter.Disconnect(500)
		case yggdrasil.CommandNameReconnect:
			log.Info("reconnecting...")
			c.transporter.Disconnect(500)
			delay, err := strconv.ParseInt(cmd.Arguments["delay"], 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse data to int: %w", err)
			}
			time.Sleep(time.Duration(delay) * time.Second)

			if err := c.transporter.Connect(); err != nil {
				return fmt.Errorf("cannot reconnect to broker: %w", err)
			}
		case yggdrasil.CommandNameCancel:
			log.Info("cancelling message...")
			// Unmarshall comand arguments
			// cmd contains the directive and the message id to be canceled.
			directive, exists := cmd.Arguments["directive"]
			if !exists {
				return fmt.Errorf("cancel command does not contain 'directive' argument")
			}
			cancelID, exists := cmd.Arguments["messageID"]
			if !exists {
				return fmt.Errorf("cancel command does not contain 'messageID' argument")
			}

			directive, err := work.ScrubName(directive)
			if err != nil {
				log.Debug(err)
			}

			// Dispatch to appropriate worker.
			if err := c.dispatcher.CancelMessage(directive, msg.MessageID, cancelID); err != nil {
				return fmt.Errorf("cannot dispatch cancel message: %w", err)
			}
		default:
			return fmt.Errorf("unknown command: %v", cmd.Command)
		}
	default:
		return fmt.Errorf("unsupported control message: %v", msg)
	}

	return nil
}

// ConnectionStatus creates a connection-status message using the current state
// of the client.
func (c *Client) ConnectionStatus() (*yggdrasil.ConnectionStatus, error) {
	var facts map[string]interface{}

	if config.DefaultConfig.FactsFile != "" {
		data, err := os.ReadFile(config.DefaultConfig.FactsFile)
		if err != nil {
			log.Errorf("cannot read facts file: %v", err)
		}
		if err := json.Unmarshal(data, &facts); err != nil {
			log.Errorf("cannot unmarshal facts: %v", err)
		}
	}

	tagsFilePath := filepath.Join(constants.ConfigDir, "tags.toml")

	var tagMap map[string]string
	if _, err := os.Stat(tagsFilePath); !os.IsNotExist(err) {
		var err error
		tagMap, err = tags.ReadTagsFile(tagsFilePath)
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
			CanonicalFacts map[string]interface{}       "json:\"canonical_facts\""
			Dispatchers    map[string]map[string]string "json:\"dispatchers\""
			State          yggdrasil.ConnectionState    "json:\"state\""
			Tags           map[string]string            "json:\"tags,omitempty\""
			ClientVersion  string                       "json:\"client_version,omitempty\""
		}{
			CanonicalFacts: facts,
			Dispatchers:    c.dispatcher.FlattenDispatchers(),
			State:          yggdrasil.ConnectionStateOnline,
			Tags:           tagMap,
			ClientVersion:  constants.Version,
		},
	}

	return &msg, nil
}
