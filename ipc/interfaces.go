package ipc

import (
	_ "embed"
)

//go:embed com.redhat.yggdrasil.Dispatcher1.xml
var InterfaceDispatcher string

// DispatcherEvent is an event emitted by the
// com.redhat.yggdrasil.Dispatcher1.Event signal.
type DispatcherEvent uint

const (
	// Emitted when the dispatcher receives the "disconnect" command.
	DispatcherEventReceivedDisconnect DispatcherEvent = 1

	// Emitted when the transport unexpected disconnects from the network.
	DispatcherEventUnexpectedDisconnect DispatcherEvent = 2

	// Emitted when the transport reconnects to the network.
	DispatcherEventConnectionRestored DispatcherEvent = 3
)

//go:embed com.redhat.yggdrasil.Worker1.xml
var InterfaceWorker string
