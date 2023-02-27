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

type WorkerEvent uint

const (

	// Emitted when the worker "accepts" a dispatched message and begins
	// "working".
	WorkerEventBegin WorkerEvent = 1

	// Emitted when the worker finishes "working".
	WorkerEventEnd WorkerEvent = 2

	// Emitted when the worker wishes to continue to announce it is
	// working.
	WorkerEventWorking WorkerEvent = 3
)

func (e WorkerEvent) String() string {
	switch e {
	case 1:
		return "BEGIN"
	case 2:
		return "END"
	case 3:
		return "WORKING"
	}
	return ""
}
