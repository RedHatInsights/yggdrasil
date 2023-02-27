package work

import (
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
	"github.com/redhatinsights/yggdrasil/internal/sync"
	"github.com/redhatinsights/yggdrasil/ipc"
)

const (
	TransmitResponseErr int = -1
	TransmitResponseOK  int = 0
)

// Dispatcher implements the com.redhat.yggdrasil.Dispatcher1 D-Bus interface
// and is suitable to be exported onto a bus.
//
// Dispatcher receives values on its 'inbound' channel and sends them via D-Bus
// to the destination worker. It sends values on the 'outbound' channel to relay
// data received from workers to a remote address.
type Dispatcher struct {
	HTTPClient  *internalhttp.Client
	conn        *dbus.Conn
	features    sync.RWMutexMap[map[string]string]
	Dispatchers chan map[string]map[string]string
	Inbound     chan yggdrasil.Data
	Outbound    chan struct {
		Data yggdrasil.Data
		Resp chan yggdrasil.Response
	}
}

func NewDispatcher(client *internalhttp.Client) *Dispatcher {
	return &Dispatcher{
		HTTPClient:  client,
		features:    sync.RWMutexMap[map[string]string]{},
		Dispatchers: make(chan map[string]map[string]string),
		Inbound:     make(chan yggdrasil.Data),
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
		log.Debugf("connecting to session bus for worker IPC: %v", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
		d.conn, err = dbus.ConnectSessionBus()
	} else {
		log.Debug("connecting to system bus for worker IPC")
		d.conn, err = dbus.ConnectSystemBus()
	}
	if err != nil {
		return fmt.Errorf("cannot connect to bus: %w", err)
	}

	if err := d.conn.Export(d, "/com/redhat/yggdrasil/Dispatcher1", "com.redhat.yggdrasil.Dispatcher1"); err != nil {
		return fmt.Errorf("cannot export com.redhat.yggdrasil.Dispatcher1 interface: %v", err)
	}

	if err := d.conn.Export(introspect.Introspectable(ipc.InterfaceDispatcher), "/com/redhat/yggdrasil/Dispatcher1", "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("cannot export org.freedesktop.DBus.Introspectable interface: %v", err)
	}

	reply, err := d.conn.RequestName("com.redhat.yggdrasil.Dispatcher1", dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("cannot request name on bus: %v", err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("name already taken")
	}

	log.Infof("exported /com/redhat/yggdrasil/Dispatcher1 on bus")

	// Add a match signal on the
	// org.freedesktop.DBus.Properties.PropertiesChanged signal.
	if err := d.conn.AddMatchSignal(dbus.WithMatchPathNamespace("/com/redhat/yggdrasil/Worker1"), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged")); err != nil {
		return fmt.Errorf("cannot add signal match: %v", err)
	}

	if err := d.conn.AddMatchSignal(dbus.WithMatchPathNamespace("/com/redhat/yggdrasil/Worker1"), dbus.WithMatchInterface("com.redhat.yggdrasil.Worker1"), dbus.WithMatchMember("Event")); err != nil {
		return fmt.Errorf("cannot add signal match: %v", err)
	}

	// start goroutine that receives values on the signals channel and handles
	// them appropriately.
	signals := make(chan *dbus.Signal)
	d.conn.Signal(signals)
	go func() {
		for s := range signals {
			log.Tracef("received signal: %#v", s)
			dest, err := d.senderName(dbus.Sender(s.Sender))
			if err != nil {
				log.Errorf("cannot find sender: %v", err)
				continue
			}
			switch s.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				changedProperties, ok := s.Body[1].(map[string]dbus.Variant)
				if !ok {
					log.Errorf("cannot convert body element 1 (changed_properties) to map[string]dbus.Variant: %v", err)
					continue
				}
				log.Debugf("%+v", changedProperties)
				directive := strings.TrimPrefix(dest, "com.redhat.yggdrasil.Worker1.")

				if _, has := changedProperties["Features"]; has {
					d.features.Set(directive, changedProperties["Features"].Value().(map[string]string))
					d.Dispatchers <- d.FlattenDispatchers()
				}
			case "com.redhat.yggdrasil.Worker1.Event":
				eventName, ok := s.Body[0].(uint32)
				if !ok {
					log.Errorf("cannot convert %T to uint32", s.Body[0])
					continue
				}
				switch ipc.WorkerEvent(eventName) {
				case ipc.WorkerEventBegin, ipc.WorkerEventEnd:
					if err := d.conn.Emit("/com/redhat/Yggdrasil1", "com.redhat.Yggdrasil1.WorkerEvent", strings.TrimPrefix(dest, "com.redhat.yggdrasil.Worker1."), eventName); err != nil {
						log.Errorf("cannot emit event: %v", err)
					}
					log.Debugf("worker emitted event: %v", ipc.WorkerEvent(eventName))
				case ipc.WorkerEventWorking:
					eventMessage, ok := s.Body[1].(string)
					if !ok {
						log.Errorf("cannot convert %T to string", s.Body[1])
						continue
					}
					if err := d.conn.Emit("/com/redhat/Yggdrasil1", "com.redhat.Yggdrasil1.WorkerEvent", strings.TrimPrefix(dest, "com.redhat.yggdrasil.Worker1."), eventName, eventMessage); err != nil {
						log.Errorf("cannot emit event: %v", err)
					}
					log.Debugf("worker emitted event: %v with message '%v'", ipc.WorkerEvent(eventName), eventMessage)
				}
			}
		}
	}()

	// start goroutine receiving values from the inbound channel and send them
	// via the Worker D-Bus interface.
	go func() {
		for data := range d.Inbound {
			if err := d.dispatch(data); err != nil {
				log.Errorf("cannot dispatch data: %v", err)
				continue
			}
		}
	}()

	return nil
}

func (d *Dispatcher) dispatch(data yggdrasil.Data) error {
	obj := d.conn.Object("com.redhat.yggdrasil.Worker1."+data.Directive, dbus.ObjectPath(filepath.Join("/com/redhat/yggdrasil/Worker1/", data.Directive)))
	r, err := obj.GetProperty("com.redhat.yggdrasil.Worker1.RemoteContent")
	if err != nil {
		return fmt.Errorf("cannot get property 'com.redhat.yggdrasil.Worker1.RemoteContent': %v", err)
	}

	if r.Value().(bool) {
		URL, err := url.Parse(string(data.Content))
		if err != nil {
			return fmt.Errorf("cannot parse content as URL: %v", err)
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

	call := obj.Call("com.redhat.yggdrasil.Worker1.Dispatch", 0, data.Directive, data.MessageID, data.Metadata, data.Content)
	if err := call.Store(); err != nil {
		return fmt.Errorf("cannot call Dispatch method on worker: %v", err)
	}
	log.Debugf("send message %v to worker %v", data.MessageID, data.Directive)

	return nil
}

// Dispatch implements the com.redhat.Yggdrasil1.Dispatch method.
func (d *Dispatcher) Dispatch(directive string, messageID string, metadata map[string]string, data []byte) *dbus.Error {
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
	if err := d.dispatch(msg); err != nil {
		return newDBusError("Dispatch", fmt.Sprintf("cannot dispatch to directive: %v", err))
	}
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
	})

	return dispatchers
}

func (d *Dispatcher) EmitEvent(event ipc.DispatcherEvent) error {
	return d.conn.Emit("/com/redhat/yggdrasil/Dispatcher1", "com.redhat.yggdrasil.Dispatcher1.Event", event)
}

// Transmit implements the com.redhat.yggdrasil.Dispatcher1.Transmit method.
func (d *Dispatcher) Transmit(sender dbus.Sender, addr string, messageID string, metadata map[string]string, data []byte) (responseCode int, responseMetadata map[string]string, responseData []byte, responseError *dbus.Error) {
	name, err := d.senderName(sender)
	if err != nil {
		return TransmitResponseErr, nil, nil, newDBusError("Transmit", fmt.Sprintf("cannot get name for sender: %v", err))
	}

	directive := strings.TrimPrefix(name, "com.redhat.yggdrasil.Worker1.")

	obj := d.conn.Object("com.redhat.yggdrasil.Worker1."+directive, dbus.ObjectPath(filepath.Join("/com/redhat/yggdrasil/Worker1/", directive)))
	r, err := obj.GetProperty("com.redhat.yggdrasil.Worker1.RemoteContent")
	if err != nil {
		return -1, nil, nil, newDBusError("Transmit", "cannot get property 'com.redhat.yggdrasil.Worker1.RemoteContent'")
	}

	if r.Value().(bool) {
		URL, err := url.Parse(addr)
		if err != nil {
			return TransmitResponseErr, nil, nil, newDBusError("Transmit", fmt.Sprintf("cannot parse addr as URL: %v", err))
		}
		if URL.Scheme != "" {
			if config.DefaultConfig.DataHost != "" {
				URL.Host = config.DefaultConfig.DataHost
			}
			resp, err := d.HTTPClient.Post(URL.String(), metadata, data)
			if err != nil {
				return TransmitResponseErr, nil, nil, newDBusError("Transmit", fmt.Sprintf("cannot perform HTTP request: %v", err))
			}
			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return TransmitResponseErr, nil, nil, newDBusError("Transmit", fmt.Sprintf("cannot read HTTP response body: %v", err))
			}
			resp.Body.Close()
		}
	}

	ch := make(chan yggdrasil.Response)
	d.Outbound <- struct {
		Data yggdrasil.Data
		Resp chan yggdrasil.Response
	}{
		Data: yggdrasil.Data{
			Type:       yggdrasil.MessageTypeData,
			MessageID:  messageID,
			ResponseTo: "",
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
		return TransmitResponseErr, nil, nil, newDBusError("Transmit", "timeout reached waiting for response")
	}
	return
}

// senderName retrieves a list of names from the bus object, iterating over each
// name, looking for a name owned by sender, returning the name if one is found.
func (d *Dispatcher) senderName(sender dbus.Sender) (string, error) {
	var names []string
	if err := d.conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return "", fmt.Errorf("cannot call org.freedesktop.DBus.ListNames: %v", err)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "com.redhat.yggdrasil.Worker1.") {
			var owner string
			if err := d.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner); err != nil {
				return "", fmt.Errorf("cannot call org.freedesktop.DBus.GetNameOwner: %v", err)
			}
			if owner == string(sender) {
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("cannot get name for sender: %v", sender)
}

func newDBusError(name string, body ...string) *dbus.Error {
	e := dbus.Error{}
	e.Name = "com.redhat.yggdrasil.Dispatcher1." + name
	for _, v := range body {
		e.Body = append(e.Body, v)
	}
	return &e
}
