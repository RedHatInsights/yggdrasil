package worker

import (
	"fmt"
	"os"
	"path"
	"regexp"

	"git.sr.ht/~spc/go-log"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/redhatinsights/yggdrasil/ipc"
)

// RxFunc is a function type that gets called each time the worker receives data.
type RxFunc func(w *Worker, addr string, id string, responseTo string, metadata map[string]string, data []byte) error

// CancelRxFunc is a function type that gets called each time the worker receives
// a cancel message
type CancelRxFunc func(w *Worker, addr string, id string, cancelID string) error

// EventHandlerFunc is a function type that gets called each time the worker
// receives a com.redhat.Yggdrasil1.Dispatcher1.Event signal.
type EventHandlerFunc func(e ipc.DispatcherEvent)

// Worker implements the com.redhat.Yggdrasil1.Worker1 interface.
type Worker struct {
	directive     string
	features      map[string]string
	remoteContent bool
	rx            RxFunc
	cancelRx      CancelRxFunc
	conn          *dbus.Conn
	objectPath    dbus.ObjectPath
	busName       string
	eventHandler  EventHandlerFunc
}

// NewWorker creates a new worker.
func NewWorker(
	directive string,
	remoteContent bool,
	features map[string]string,
	cancel CancelRxFunc,
	rx RxFunc,
	events EventHandlerFunc,
) (*Worker, error) {
	r := regexp.MustCompile("-")
	if r.Match([]byte(directive)) {
		return nil, fmt.Errorf("invalid directive '%v'", directive)
	}

	w := Worker{
		directive:     directive,
		features:      features,
		remoteContent: remoteContent,
		cancelRx:      cancel,
		rx:            rx,
		objectPath:    dbus.ObjectPath(path.Join("/com/redhat/Yggdrasil1/Worker1", directive)),
		busName:       fmt.Sprintf("com.redhat.Yggdrasil1.Worker1.%v", directive),
		eventHandler:  events,
	}

	return &w, nil
}

// Connect connects to the bus, exports the worker on its object path, and
// requests a well-known bus name. It connects to a private session bus, if
// DBUS_SESSION_BUS_ADDRESS is set in the environment. Otherwise it connects to
// the system bus. It exports w onto the bus and waits until a signal is
// received on quit.
func (w *Worker) Connect(quit <-chan os.Signal) error {
	var err error

	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		log.Debugf("connecting to session bus: %v", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
		w.conn, err = dbus.ConnectSessionBus()
	} else {
		w.conn, err = dbus.ConnectSystemBus()
	}
	if err != nil {
		return fmt.Errorf("error: cannot connect to bus: %w", err)
	}

	// Export properties onto the bus as an org.freedesktop.DBus.Properties
	// interface.
	propertySpec := prop.Map{
		"com.redhat.Yggdrasil1.Worker1": {
			"Features": {
				Value:    w.features,
				Writable: false,
				Emit:     prop.EmitTrue,
			},
			"RemoteContent": {
				Value:    w.remoteContent,
				Writable: false,
				Emit:     prop.EmitTrue,
			},
		},
	}

	_, err = prop.Export(w.conn, w.objectPath, propertySpec)
	if err != nil {
		return fmt.Errorf("cannot export com.redhat.Yggdrasil1.Worker1 properties: %w", err)
	}

	// Export worker onto the bus, implementing the com.redhat.Yggdrasil1.Worker1
	// and org.freedesktop.DBus.Introspectable interfaces. The path name the
	// worker exports includes the directive name.
	if err := w.conn.ExportMethodTable(map[string]interface{}{"Dispatch": w.dispatch, "Cancel": w.cancel}, w.objectPath, "com.redhat.Yggdrasil1.Worker1"); err != nil {
		return fmt.Errorf("cannot export com.redhat.Yggdrasil1.Worker1 interface: %w", err)
	}

	if err := w.conn.Export(introspect.Introspectable(ipc.InterfaceWorker), w.objectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("cannot export org.freedesktop.DBus.Introspectable interface: %w", err)
	}

	// Request ownership of the well-known bus address.
	reply, err := w.conn.RequestName(w.busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("cannot request name on bus: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("request name failed")
	}

	// Emit a started event
	err = w.EmitEvent(
		ipc.WorkerEventNameStarted,
		"",
		"",
		map[string]string{},
	)
	if err != nil {
		return fmt.Errorf("cannot emit event: %w", err)
	}

	signals := make(chan *dbus.Signal)
	w.conn.Signal(signals)
	go func() {
		for s := range signals {
			switch s.Name {
			case "com.redhat.Yggdrasil1.Dispatcher1.Event":
				event := s.Body[0].(uint)
				if w.eventHandler == nil {
					continue
				}
				w.eventHandler(ipc.DispatcherEvent(event))
			}
		}
	}()

	<-quit

	// Emit a stopped event
	err = w.EmitEvent(
		ipc.WorkerEventNameStopped,
		"",
		"",
		map[string]string{},
	)
	if err != nil {
		return fmt.Errorf("cannot emit event: %w", err)
	}

	return nil
}

// SetFeature sets the value for the given key in the feature map and emits the
// PropertiesChanged signal.
func (w *Worker) SetFeature(name, value string) error {
	w.features[name] = value
	return w.conn.Emit(
		w.objectPath,
		"org.freedesktop.DBus.Properties.PropertiesChanged",
		"com.redhat.Yggdrasil1.Worker1.Features",
		map[string]dbus.Variant{"Features": dbus.MakeVariant(w.features)},
	)
}

// GetFeature retrieves the value from the feature map for given key.
func (w *Worker) GetFeature(name string) string {
	return w.features[name]
}

// Transmit wraps a com.redhat.Yggdrasil1.Dispatcher1.Transmit method call for
// ease of use from the worker.
func (w *Worker) Transmit(
	addr string,
	id string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) (responseCode int, responseMetadata map[string]string, responseData []byte, err error) {
	// Look up the Dispatcher object on the bus connection and call its Transmit
	// method, returning the data received.
	obj := w.conn.Object("com.redhat.Yggdrasil1.Dispatcher1", "/com/redhat/Yggdrasil1/Dispatcher1")
	err = obj.Call("com.redhat.Yggdrasil1.Dispatcher1.Transmit", 0, addr, id, responseTo, metadata, data).
		Store(&responseCode, &responseMetadata, &responseData)
	if err != nil {
		responseCode = -1
		return
	}
	return
}

// EmitEvent emits a WorkerEvent, worker message id, and key-value pairs of optional data.
func (w *Worker) EmitEvent(
	event ipc.WorkerEventName,
	messageID string,
	responseTo string,
	data map[string]string,
) error {
	args := []interface{}{
		event,
		messageID,
		responseTo,
		data,
	}
	log.Debugf("emitting event %v", event)
	return w.conn.Emit(
		dbus.ObjectPath(path.Join("/com/redhat/Yggdrasil1/Worker1", w.directive)),
		"com.redhat.Yggdrasil1.Worker1.Event",
		args...)
}

// cancel implements com.redhat.Yggdrasil1.Worker1.Cancel method by calling the
// worker's cancelRxFunc in a goroutine.
func (w *Worker) cancel(addr string, id string, cancelID string) *dbus.Error {
	// If worker doesn't implement cancellation it does nothing
	if w.cancelRx == nil {
		log.Debug("worker does not support cancellation messages")
		return dbus.NewError(
			"org.freedesktop.DBus.UnknownInterface",
			[]interface{}{"cancel method not implemented"},
		)
	}

	log.Tracef("cancelling message:")
	log.Tracef("addr = %v", addr)
	log.Tracef("id = %v", id)
	log.Tracef("cancel-id = %v", cancelID)

	// Communicate the worker accepts the message and its starting to
	// work on it.
	if err := w.EmitEvent(ipc.WorkerEventNameBegin, id, "", map[string]string{}); err != nil {
		return dbus.NewError("com.redhat.Yggdrasil1.Worker1.EventError", []interface{}{err.Error()})
	}

	go func() {
		if err := w.cancelRx(w, addr, id, cancelID); err != nil {
			log.Errorf("callback function cancelRx() was terminated: %v", err)
		}
		// Communicate to yggd client that the work has finished.
		if err := w.EmitEvent(ipc.WorkerEventNameEnd, id, "", map[string]string{}); err != nil {
			log.Errorf("cannot emit event: %v", err)
		}
	}()

	return nil
}

// dispatch implements com.redhat.Yggdrasil1.Worker1.Dispatch by calling the
// worker's RxFunc in a goroutine.
func (w *Worker) dispatch(
	addr string,
	id string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) *dbus.Error {
	// Log the data received at a high log level for debugging purposes.
	log.Tracef("addr = %v", addr)
	log.Tracef("id = %v", id)
	log.Tracef("responseTo = %v", responseTo)
	log.Tracef("metadata = %#v", metadata)
	log.Tracef("data = %v", data)

	if err := w.EmitEvent(ipc.WorkerEventNameBegin, id, responseTo, map[string]string{}); err != nil {
		return dbus.NewError("com.redhat.Yggdrasil1.Worker1.EventError", []interface{}{err.Error()})
	}

	go func() {
		if err := w.rx(w, addr, id, responseTo, metadata, data); err != nil {
			log.Errorf("cannot call rx: %v", err)
		}
		if err := w.EmitEvent(ipc.WorkerEventNameEnd, id, responseTo, map[string]string{}); err != nil {
			log.Errorf("cannot emit event: %v", err)
		}
	}()

	return nil
}
