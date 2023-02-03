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
type RxFunc func(w *Worker, addr string, id string, metadata map[string]string, data []byte) error

// Worker implements the com.redhat.yggdrasil.Worker1 interface.
type Worker struct {
	directive     string
	features      map[string]string
	remoteContent bool
	rx            RxFunc
	conn          *dbus.Conn
	objectPath    dbus.ObjectPath
	busName       string
}

// NewWorker creates a new worker.
func NewWorker(directive string, remoteContent bool, features map[string]string, rx RxFunc) (*Worker, error) {
	r := regexp.MustCompile("-")
	if r.Match([]byte(directive)) {
		return nil, fmt.Errorf("invalid directive '%v'", directive)
	}

	w := Worker{
		directive:     directive,
		features:      features,
		remoteContent: remoteContent,
		rx:            rx,
		objectPath:    dbus.ObjectPath(path.Join("/com/redhat/yggdrasil/Worker1", directive)),
		busName:       fmt.Sprintf("com.redhat.yggdrasil.Worker1.%v", directive),
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

	if os.Getenv("DBUS_STARTER_BUS_TYPE") == "session" {
		log.Debugf("connecting to private bus: %v", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
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
		"com.redhat.yggdrasil.Worker1": {
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
		return fmt.Errorf("cannot export com.redhat.yggdrasil.Worker1 properties: %w", err)
	}

	// Export worker onto the bus, implementing the com.redhat.yggdrasil.Worker1
	// and org.freedesktop.DBus.Introspectable interfaces. The path name the
	// worker exports includes the directive name.
	if err := w.conn.ExportMethodTable(map[string]interface{}{"Dispatch": w.dispatch}, w.objectPath, "com.redhat.yggdrasil.Worker1"); err != nil {
		return fmt.Errorf("cannot export com.redhat.yggdrasil.Worker1 interface: %w", err)
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

	<-quit

	return nil
}

// SetFeature sets the value for the given key in the feature map and emits the
// PropertiesChanged signal.
func (w *Worker) SetFeature(name, value string) error {
	w.features[name] = value
	return w.conn.Emit(w.objectPath, "org.freedesktop.DBus.Properties.PropertiesChanged", "com.redhat.yggdrasil.Worker1.Features", map[string]dbus.Variant{"Features": dbus.MakeVariant(w.features)})
}

// GetFeature retrieves the value from the feature map for given key.
func (w *Worker) GetFeature(name string) string {
	return w.features[name]
}

// Transmit wraps a com.redhat.yggdrasil.Dispatcher1.Transmit method call for
// ease of use from the worker.
func (w *Worker) Transmit(addr string, id string, metadata map[string]string, data []byte) (responseCode int, responseMetadata map[string]string, responseData []byte, err error) {
	// Look up the Dispatcher object on the bus connection and call its Transmit
	// method, returning the data received.
	obj := w.conn.Object("com.redhat.yggdrasil.Dispatcher1", "/com/redhat/yggdrasil/Dispatcher1")
	err = obj.Call("com.redhat.yggdrasil.Dispatcher1.Transmit", 0, addr, id, metadata, data).Store(&responseCode, &responseMetadata, &responseData)
	if err != nil {
		responseCode = -1
		return
	}
	return
}

// dispatch implements com.redhat.yggdrasil.Worker1.dispatch by calling the
// worker's RxFunc in a goroutine.
func (w *Worker) dispatch(addr string, id string, metadata map[string]string, data []byte) *dbus.Error {
	// Log the data received at a high log level for debugging purposes.
	log.Tracef("addr = %v", addr)
	log.Tracef("id = %v", id)
	log.Tracef("metadata = %#v", metadata)
	log.Tracef("data = %v", data)

	go func() {
		if err := w.rx(w, addr, id, metadata, data); err != nil {
			log.Errorf("cannot call rx: %v", err)
		}
	}()

	return nil
}
