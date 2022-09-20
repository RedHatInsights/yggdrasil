package main

import (
	"os"
	"os/signal"
	"syscall"

	"git.sr.ht/~spc/go-log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/redhatinsights/yggdrasil/ipc"
)

var yggdDispatchSocketAddr string

func main() {
	// Get the log level specified by yggd via the YGG_LOG_LEVEL environment
	// variable.
	if logLevel, has := os.LookupEnv("YGG_LOG_LEVEL"); has {
		level, err := log.ParseLevel(logLevel)
		if err != nil {
			log.Fatalf("error: cannot parse log level: %v", err)
		}
		log.SetLevel(level)
	}

	// Connect to the bus, either the system bus or a private session bus,
	// depending on whether DBUS_SESSION_BUS_ADDRESS is set in the environment.
	var conn *dbus.Conn
	var err error
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		log.Debugf("connecting to private bus: %v", os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
		conn, err = dbus.Connect(os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
	} else {
		conn, err = dbus.ConnectSystemBus()
	}
	if err != nil {
		log.Fatalf("error: cannot connect to bus: %v", err)
	}

	// Create the worker.
	worker := Worker{
		conn: conn,
		Features: map[string]string{
			"DispatchedAt": "",
		},
	}

	// Export properties onto the bus as an org.freedesktop.DBus.Properties
	// interface.
	propertySpec := prop.Map{
		"com.redhat.yggdrasil.Worker1": {
			"Features": {
				Value:    worker.Features,
				Writable: false,
				Emit:     prop.EmitTrue,
			},
		},
	}

	_, err = prop.Export(conn, "/com/redhat/yggdrasil/Worker1/echo", propertySpec)
	if err != nil {
		log.Fatalf("cannot export com.redhat.yggdrasil.Worker1 properties: %v", err)
	}

	// Export worker onto the bus, implementing the com.redhat.yggdrasil.Worker1
	// and org.freedesktop.DBus.Introspectable interfaces. The path name the
	// worker exports includes the directive name (in this case "echo").
	if err := conn.Export(&worker, "/com/redhat/yggdrasil/Worker1/echo", "com.redhat.yggdrasil.Worker1"); err != nil {
		log.Fatalf("cannot export com.redhat.yggdrasil.Worker1 interface: %v", err)
	}

	if err := conn.Export(introspect.Introspectable(ipc.InterfaceWorker), "/com/redhat/yggdrasil/Worker1/echo", "org.freedesktop.DBus.Introspectable"); err != nil {
		log.Fatalf("cannot export org.freedesktop.DBus.Introspectable interface: %v", err)
	}

	// Request ownership of the well-known bus address
	// com.redhat.yggdrasil.Worker1.echo.
	reply, err := conn.RequestName("com.redhat.yggdrasil.Worker1.echo", dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Fatalf("error: cannot request name on bus: %v", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("error: failed to request ownership of name on bus")
	}

	// Set up a channel to receive the TERM or INT signal over and clean up
	// before quitting.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	<-quit
}
