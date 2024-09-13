package work

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/config"
	internalhttp "github.com/redhatinsights/yggdrasil/internal/http"
	"github.com/redhatinsights/yggdrasil/internal/messagejournal"
	"github.com/redhatinsights/yggdrasil/internal/sync"
	"github.com/redhatinsights/yggdrasil/ipc"
)

const (
	TransmitResponseErr int = -1
	TransmitResponseOK  int = 0
)

// Dispatcher implements the com.redhat.Yggdrasil1.Dispatcher1 D-Bus interface
// and is suitable to be exported onto a bus.
//
// Dispatcher receives values on its 'inbound' channel and sends them via D-Bus
// to the destination worker. It sends values on the 'outbound' channel to relay
// data received from workers to a remote address.
type Dispatcher struct {
	HTTPClient     *internalhttp.Client
	conn           *dbus.Conn
	features       sync.RWMutexMap[map[string]string]
	MessageJournal *messagejournal.MessageJournal
	Dispatchers    chan map[string]map[string]string
	WorkerEvents   chan ipc.WorkerEvent
	Inbound        chan yggdrasil.Data
	Outbound       chan struct {
		Data yggdrasil.Data
		Resp chan yggdrasil.Response
	}
}

func NewDispatcher(client *internalhttp.Client) *Dispatcher {
	return &Dispatcher{
		HTTPClient:     client,
		features:       sync.RWMutexMap[map[string]string]{},
		MessageJournal: nil,
		Dispatchers:    make(chan map[string]map[string]string),
		WorkerEvents:   make(chan ipc.WorkerEvent),
		Inbound:        make(chan yggdrasil.Data),
		Outbound: make(chan struct {
			Data yggdrasil.Data
			Resp chan yggdrasil.Response
		}),
	}
}

// Connect connects the dispatcher to an appropriate D-Bus broker and begins
// processing messages received on the inbound channel.
func (d *Dispatcher) Connect() error {
	var err error
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		log.Debugf(
			"connecting to session bus for worker IPC: %v",
			os.Getenv("DBUS_SESSION_BUS_ADDRESS"),
		)
		d.conn, err = dbus.ConnectSessionBus()
	} else {
		log.Debug("connecting to system bus for worker IPC")
		d.conn, err = dbus.ConnectSystemBus()
	}
	if err != nil {
		return fmt.Errorf("cannot connect to bus: %w", err)
	}

	if err := d.conn.Export(d, "/com/redhat/Yggdrasil1/Dispatcher1", "com.redhat.Yggdrasil1.Dispatcher1"); err != nil {
		return fmt.Errorf("cannot export com.redhat.Yggdrasil1.Dispatcher1 interface: %v", err)
	}

	if err := d.conn.Export(introspect.Introspectable(ipc.InterfaceDispatcher), "/com/redhat/Yggdrasil1/Dispatcher1", "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("cannot export org.freedesktop.DBus.Introspectable interface: %v", err)
	}

	reply, err := d.conn.RequestName("com.redhat.Yggdrasil1.Dispatcher1", dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("cannot request name com.redhat.Yggdrasil1.Dispatcher1 on bus: %v", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("name com.redhat.Yggdrasil1.Dispatcher1 already taken")
	}

	log.Infof("exported /com/redhat/Yggdrasil1/Dispatcher1 on bus")

	// Add a match signal on the
	// org.freedesktop.DBus.Properties.PropertiesChanged signal.
	if err := d.conn.AddMatchSignal(dbus.WithMatchPathNamespace("/com/redhat/Yggdrasil1/Worker1"), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged")); err != nil {
		return fmt.Errorf(
			"cannot add signal match on org.freedesktop.DBus.Properties.PropertiesChanged: %v",
			err,
		)
	}

	if err := d.conn.AddMatchSignal(dbus.WithMatchPathNamespace("/com/redhat/Yggdrasil1/Worker1"), dbus.WithMatchInterface("com.redhat.Yggdrasil1.Worker1"), dbus.WithMatchMember("Event")); err != nil {
		return fmt.Errorf(
			"cannot add signal match on com.redhat.Yggdrasil1.Worker1.Event: %v",
			err,
		)
	}

	err = d.conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg0Namespace("com.redhat.Yggdrasil1.Worker1"),
	)
	if err != nil {
		return fmt.Errorf(
			"cannot add signal match on org.freedesktop.DBus.NameOwnerChanged: %v",
			err,
		)
	}

	// start goroutine that receives values on the signals channel and handles
	// them appropriately.
	signals := make(chan *dbus.Signal)
	d.conn.Signal(signals)
	go func() {
		for s := range signals {
			log.Tracef("received signal: %#v", s)

			switch s.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				changedProperties, ok := s.Body[1].(map[string]dbus.Variant)
				if !ok {
					log.Error(
						"cannot convert body element 1 (changed_properties) to map[string]dbus.Variant",
					)
					continue
				}
				log.Debugf("%+v", changedProperties)
				directive := filepath.Base(string(s.Path))

				if _, has := changedProperties["Features"]; has {
					d.features.Set(
						directive,
						changedProperties["Features"].Value().(map[string]string),
					)
					d.Dispatchers <- d.FlattenDispatchers()
				}
			case "com.redhat.Yggdrasil1.Worker1.Event":
				event, err := workerEventFromSignal(s)
				if err != nil {
					log.Errorf("cannot unpack signal: %v", err)
					continue
				}
				event.Worker = filepath.Base(string(s.Path))

				d.WorkerEvents <- *event

				// Start goroutine to add a new message journal entry.
				go func() {
					// Skip adding a new entry if the message journal is disabled.
					if d.MessageJournal == nil {
						return
					}
					workerMessage := yggdrasil.WorkerMessage{
						MessageID:  event.MessageID,
						Sent:       time.Now().UTC(),
						WorkerName: event.Worker,
						ResponseTo: event.ResponseTo,
						WorkerEvent: struct {
							EventName uint              "json:\"event_name\""
							EventData map[string]string "json:\"event_data\""
						}{
							uint(event.Name),
							event.Data,
						},
					}
					if err := d.MessageJournal.AddEntry(workerMessage); err != nil {
						log.Errorf("cannot add journal entry: %v", err)
					}
				}()
			case "org.freedesktop.DBus.NameOwnerChanged":
				name, ok := s.Body[0].(string)
				if !ok {
					log.Error("cannot convert body element 0 (name) to string")
					continue
				}
				oldOwner, ok := s.Body[1].(string)
				if !ok {
					log.Error("cannot convert body element 1 (old_owner) to string")
					continue
				}
				newOwner, ok := s.Body[2].(string)
				if !ok {
					log.Error("cannot convert body element 2 (new_owner) to string")
					continue
				}

				workerName := strings.TrimPrefix(name, "com.redhat.Yggdrasil1.Worker1.")

				// If there was an old owner, this signal means the old
				// owner no longer owns the name; clean up the feature map.
				if oldOwner != "" {
					d.features.Del(workerName)
				}

				// If there is a new owner, this signal means a new process
				// owns the name; add a record to the feature map.
				if newOwner != "" {
					obj := d.conn.Object(
						name,
						dbus.ObjectPath(
							filepath.Join("/com/redhat/Yggdrasil1/Worker1/", workerName),
						),
					)
					v, err := obj.GetProperty("com.redhat.Yggdrasil1.Worker1.Features")
					if err != nil {
						log.Errorf(
							"cannot get property 'com.redhat.Yggdrasil1.Worker1.Features': %v",
							err,
						)
						continue
					}
					features, ok := v.Value().(map[string]string)
					if !ok {
						log.Errorf("cannot convert %T to map[string]string", v.Value())
						continue
					}
					d.features.Set(workerName, features)
				}
				d.Dispatchers <- d.FlattenDispatchers()
			}
		}
	}()

	// start goroutine that finds workers already active on the bus connection
	// and get their features.
	go func() {
		workers, err := d.findWorkers()
		if err != nil {
			log.Errorf("cannot find workers: %v", err)
			return
		}

		for _, worker := range workers {
			directive := strings.TrimPrefix(worker, "com.redhat.Yggdrasil1.Worker1.")
			obj := d.conn.Object(
				worker,
				dbus.ObjectPath(filepath.Join("/com/redhat/Yggdrasil1/Worker1/", directive)),
			)

			result, err := obj.GetProperty("com.redhat.Yggdrasil1.Worker1.Features")
			if err != nil {
				log.Errorf("cannot get property 'com.redhat.Yggdrasil1.Worker1.Features: %v", err)
				continue
			}
			features, ok := result.Value().(map[string]string)
			if !ok {
				log.Errorf("cannot convert %T to map[string]string", result.Value())
				continue
			}
			d.features.Set(directive, features)
		}
		d.Dispatchers <- d.FlattenDispatchers()
	}()

	// start goroutine receiving values from the inbound channel and send them
	// via the Worker D-Bus interface.
	go func() {
		for data := range d.Inbound {
			if err := d.Dispatch(data); err != nil {
				log.Errorf("cannot dispatch data: %v", err)
				continue
			}
		}
	}()

	return nil
}

func (d *Dispatcher) Dispatch(data yggdrasil.Data) error {
	var err error
	data.Directive, err = ScrubName(data.Directive)
	if err != nil {
		log.Debug(err)
	}

	obj := d.conn.Object(
		"com.redhat.Yggdrasil1.Worker1."+data.Directive,
		dbus.ObjectPath(filepath.Join("/com/redhat/Yggdrasil1/Worker1/", data.Directive)),
	)
	propertyName := "com.redhat.Yggdrasil1.Worker1.RemoteContent"
	r, err := obj.GetProperty(propertyName)
	if err != nil {
		return fmt.Errorf(
			"cannot get property '%s' of object: %s: using destination interface: %s: %v",
			propertyName, obj.Path(), obj.Destination(), err,
		)
	}

	if r.Value().(bool) {
		// Because the data.Content field is typed as json.RawMessage, it must first be
		// unmarshalled into a Go string before parsing as a URL.
		var urlStr string
		err = json.Unmarshal(data.Content, &urlStr)
		if err != nil {
			return fmt.Errorf("unable to unmarshal JSON string fragment: %v", err)
		}

		// When string fragment was unmarshalled, then we can try to parse string as URL
		URL, err := url.Parse(urlStr)
		if err != nil {
			return fmt.Errorf("cannot parse content %v as URL: %v", urlStr, err)
		}
		if config.DefaultConfig.DataHost != "" {
			URL.Host = config.DefaultConfig.DataHost
		}

		resp, err := d.HTTPClient.Get(URL.String())
		if err != nil {
			return fmt.Errorf("cannot get detached message content: %v", err)
		}
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("cannot read response body: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			return fmt.Errorf("cannot close response body: %v", err)
		}
		data.Content = content
	}

	call := obj.Call(
		"com.redhat.Yggdrasil1.Worker1.Dispatch",
		0,
		data.Directive,
		data.MessageID,
		data.ResponseTo,
		data.Metadata,
		data.Content,
	)
	if err := call.Store(); err != nil {
		return fmt.Errorf(
			"cannot call 'Dispatch' method on worker: %s of object: %s: using destination interface: %s: %v",
			data.Directive,
			obj.Path(),
			obj.Destination(),
			err,
		)
	}
	log.Debugf("send message %v to worker %v", data.MessageID, data.Directive)

	v, err := obj.GetProperty("com.redhat.Yggdrasil1.Worker1.Features")
	if err != nil {
		return fmt.Errorf("cannot get property 'com.redhat.Yggdrasil1.Worker1.Features': %v", err)
	}
	features, ok := v.Value().(map[string]string)
	if !ok {
		return fmt.Errorf("cannot convert %T to map[string]string", v.Value())
	}
	d.features.Set(data.Directive, features)
	d.Dispatchers <- d.FlattenDispatchers()

	return nil
}

func (d *Dispatcher) DisconnectWorkers() {
	if err := d.EmitEvent(ipc.DispatcherEventReceivedDisconnect); err != nil {
		log.Errorf("cannot emit event: %v", err)
	}
}

func (d *Dispatcher) FlattenDispatchers() map[string]map[string]string {
	dispatchers := make(map[string]map[string]string)
	d.features.Visit(func(k string, v map[string]string) {
		dispatchers[k] = v
		// Include a second entry in the dispatchers map replacing any
		// underscores with hyphens to support the "legacy" names of workers.
		// This cannot cause any conflict with existing/local workers since
		// hyphens in worker names is disallowed by the D-Bus name policy.
		dispatchers[strings.ReplaceAll(k, "_", "-")] = v
	})

	return dispatchers
}

func (d *Dispatcher) EmitEvent(event ipc.DispatcherEvent) error {
	return d.conn.Emit(
		"/com/redhat/Yggdrasil1/Dispatcher1",
		"com.redhat.Yggdrasil1.Dispatcher1.Event",
		event,
	)
}

// Transmit implements the com.redhat.Yggdrasil1.Dispatcher1.Transmit method.
func (d *Dispatcher) Transmit(
	sender dbus.Sender,
	addr string,
	messageID string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) (responseCode int, responseMetadata map[string]string, responseData []byte, responseError *dbus.Error) {
	name, err := d.senderName(sender)
	if err != nil {
		return TransmitResponseErr, nil, nil, NewDBusError(
			"Transmit",
			fmt.Sprintf("cannot get name for sender: %v", err),
		)
	}

	directive := strings.TrimPrefix(name, "com.redhat.Yggdrasil1.Worker1.")

	obj := d.conn.Object(
		"com.redhat.Yggdrasil1.Worker1."+directive,
		dbus.ObjectPath(filepath.Join("/com/redhat/Yggdrasil1/Worker1/", directive)),
	)
	r, err := obj.GetProperty("com.redhat.Yggdrasil1.Worker1.RemoteContent")
	if err != nil {
		return -1, nil, nil, NewDBusError(
			"Transmit",
			"cannot get property 'com.redhat.Yggdrasil1.Worker1.RemoteContent'",
		)
	}

	if r.Value().(bool) {
		URL, err := url.Parse(addr)
		if err != nil {
			return TransmitResponseErr, nil, nil, NewDBusError(
				"Transmit",
				fmt.Sprintf("cannot parse addr as URL: %v", err),
			)
		}
		if URL.Scheme != "" {
			if config.DefaultConfig.DataHost != "" {
				URL.Host = config.DefaultConfig.DataHost
			}
			resp, err := d.HTTPClient.Post(URL.String(), metadata, data)
			if err != nil {
				return TransmitResponseErr, nil, nil, NewDBusError(
					"Transmit",
					fmt.Sprintf("cannot perform HTTP request: %v", err),
				)
			}
			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return TransmitResponseErr, nil, nil, NewDBusError(
					"Transmit",
					fmt.Sprintf("cannot read HTTP response body: %v", err),
				)
			}

			err = resp.Body.Close()
			if err != nil {
				return TransmitResponseErr, nil, nil, NewDBusError(
					"Transmit",
					fmt.Sprintf("cannot close HTTP response body: %v", err),
				)
			}
			responseCode = resp.StatusCode
			responseMetadata = make(map[string]string)
			for header := range resp.Header {
				responseMetadata[header] = resp.Header.Get(header)
			}
			responseData = data
		} else {
			return TransmitResponseErr, nil, nil, NewDBusError("Transmit", fmt.Sprintf("URL: '%v' has no scheme", addr))
		}
	} else {
		ch := make(chan yggdrasil.Response)
		d.Outbound <- struct {
			Data yggdrasil.Data
			Resp chan yggdrasil.Response
		}{
			Data: yggdrasil.Data{
				Type:       yggdrasil.MessageTypeData,
				MessageID:  messageID,
				ResponseTo: responseTo,
				Version:    1,
				Sent:       time.Now(),
				Directive:  addr,
				Metadata:   metadata,
				Content:    data,
			},
			Resp: ch,
		}

		select {
		case resp := <-ch:
			responseCode = resp.Code
			responseMetadata = resp.Metadata
			responseData = resp.Data
		case <-time.After(1 * time.Second):
			return TransmitResponseErr, nil, nil, NewDBusError("com.redhat.Yggdrasil1.Dispatcher1.Transmit", "timeout reached waiting for response")
		}
	}
	return
}

// senderName retrieves a list of names from the bus object, iterating over each
// name, looking for a name owned by sender, returning the name if one is found.
func (d *Dispatcher) senderName(sender dbus.Sender) (string, error) {
	workers, err := d.findWorkers()
	if err != nil {
		return "", fmt.Errorf("cannot get list of workers: %v", err)
	}

	for _, worker := range workers {
		var owner string
		if err := d.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, worker).Store(&owner); err != nil {
			return "", fmt.Errorf("cannot call org.freedesktop.DBus.GetNameOwner: %v", err)
		}
		if owner == string(sender) {
			return worker, nil
		}
	}

	return "", fmt.Errorf("cannot get name for sender: %v", sender)
}

// findWorkers scans the list of names returned by the bus object and collects
// all names that begin with the com.redhat.Yggdrasil1.Worker1 prefix.
func (d *Dispatcher) findWorkers() ([]string, error) {
	var names []string
	if err := d.conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return nil, fmt.Errorf("cannot call org.freedesktop.DBus.ListNames: %v", err)
	}

	var workers []string
	for _, name := range names {
		if strings.HasPrefix(name, "com.redhat.Yggdrasil1.Worker1") {
			workers = append(workers, name)
		}
	}

	return workers, nil
}

// workerEventFromSignal creates an ipc.WorkerEvent from a DBus signal.
func workerEventFromSignal(s *dbus.Signal) (*ipc.WorkerEvent, error) {
	event := ipc.WorkerEvent{}

	for i, v := range s.Body {
		switch i {
		case 0:
			name, ok := v.(uint32)
			if !ok {
				return nil, newUint32TypeConversionError(v)
			}
			event.Name = ipc.WorkerEventName(name)
		case 1:
			messageID, ok := v.(string)
			if !ok {
				return nil, newStringTypeConversionError(v)
			}
			event.MessageID = messageID
		case 2:
			responseTo, ok := v.(string)
			if !ok {
				return nil, newStringTypeConversionError(v)
			}
			event.ResponseTo = responseTo
		case 3:
			data, ok := v.(map[string]string)
			if !ok {
				return nil, newStringMapTypeConversionError(v)
			}
			event.Data = data
		}
	}

	return &event, nil
}

// CancelMessage implements the dispatching of a cancel message to the worker.
func (d *Dispatcher) CancelMessage(directive, message_id, cancel_id string) error {
	// Send the message through the cancel interface
	obj := d.conn.Object("com.redhat.Yggdrasil1.Worker1."+directive,
		dbus.ObjectPath(filepath.Join("/com/redhat/Yggdrasil1/Worker1/", directive)))
	call := obj.Call("com.redhat.Yggdrasil1.Worker1.Cancel", 0,
		directive,
		message_id,
		cancel_id)
	if err := call.Store(); err != nil {
		return fmt.Errorf(
			"cannot call Cancel method with message %v on worker %v: %v",
			cancel_id,
			directive,
			err,
		)
	}
	log.Debugf("sent cancel message %v to worker %v", cancel_id, directive)
	d.Dispatchers <- d.FlattenDispatchers()
	return nil
}
