package main

import (
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/godbus/dbus/v5"
)

type Worker struct {
	conn     *dbus.Conn
	Features map[string]string
}

// Dispatch implements the com.redhat.yggdrasil.Worker1 Dispatch interface
// method.
func (w *Worker) Dispatch(addr string, id string, metadata map[string]string, data []byte) *dbus.Error {
	var (
		responseCode     int
		responseMetadata map[string]string
		responseData     []byte
	)

	// Log the data received at a high log level for debugging purposes.
	log.Tracef("addr = %v", addr)
	log.Tracef("id = %v", id)
	log.Tracef("metadata = %#v", metadata)
	log.Tracef("data = %v", data)

	// Look up the Dispatcher object on the bus connection and call its Transmit
	// method, returning the data received.
	obj := w.conn.Object("com.redhat.yggdrasil.Dispatcher1", "/com/redhat/yggdrasil/Dispatcher1")
	err := obj.Call("com.redhat.yggdrasil.Dispatcher1.Transmit", 0, addr, id, metadata, data).Store(&responseCode, &responseMetadata, &responseData)
	if err != nil {
		log.Errorf("cannot call com.redhat.yggdrasil.Dispatcher1.Transmit: %v", err)
		return dbus.NewError("com.redhat.yggdrasil.Worker1.Echo.ReplyError", nil)
	}

	// Log the responses received from the Dispatcher, if any.
	log.Infof("responseCode = %v", responseCode)
	log.Infof("responseMetadata = %#v", responseMetadata)
	log.Infof("responseData = %v", responseData)

	// Set the Features map and manually emit the PropertiesChanged signal. This
	// is necessary because the signal is only emitted by the package when the
	// property is set through the 'Set' method. Since `Features` is defined as
	// read-only, there's no public interface for setting the property, thereby
	// triggering the PropertiesChanged signal emission.
	w.Features["DispatchedAt"] = time.Now().Format(time.RFC3339)
	w.conn.Emit("/com/redhat/yggdrasil/Worker1/echo", "org.freedesktop.DBus.Properties.PropertiesChanged", "com.redhat.yggdrasil.Worker1", map[string]dbus.Variant{"Features": dbus.MakeVariant(w.Features)})

	return nil
}
