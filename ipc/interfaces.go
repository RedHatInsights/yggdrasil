package ipc

import (
	_ "embed"
)

//go:embed com.redhat.Yggdrasil1.Dispatcher1.xml
var InterfaceDispatcher string

// DispatcherEvent is an event emitted by the
// com.redhat.Yggdrasil1.Dispatcher1.Event signal.
type DispatcherEvent uint

const (
	// DispatcherEventReceivedDisconnect is emitted when the dispatcher receives
	// the "disconnect" command.
	DispatcherEventReceivedDisconnect DispatcherEvent = 1

	// DispatcherEventUnexpectedDisconnect is emitted when the transport unexpected
	// disconnects from the network.
	DispatcherEventUnexpectedDisconnect DispatcherEvent = 2

	// DispatcherEventConnectionRestored is emitted when the transport reconnects to the network.
	DispatcherEventConnectionRestored DispatcherEvent = 3
)

//go:embed com.redhat.Yggdrasil1.Worker1.xml
var InterfaceWorker string

type WorkerEventName uint

const (

	// WorkerEventNameBegin is emitted when the worker "accepts"
	// a dispatched message and begins "working".
	WorkerEventNameBegin WorkerEventName = 1

	// WorkerEventNameEnd is emitted when the worker finishes "working".
	WorkerEventNameEnd WorkerEventName = 2

	// WorkerEventNameWorking is emitted when the worker wishes to continue
	// to announce it is working.
	WorkerEventNameWorking WorkerEventName = 3

	// WorkerEventNameConnecting is emitted when the worker starts connecting
	WorkerEventNameConnecting WorkerEventName = 4

	// WorkerEventNameConnected is emitted when the worker finishes connecting
	WorkerEventNameConnected WorkerEventName = 5
)

// String returns textual representation of event name
func (e WorkerEventName) String() string {
	switch e {
	case WorkerEventNameBegin:
		return "BEGIN"
	case WorkerEventNameEnd:
		return "END"
	case WorkerEventNameWorking:
		return "WORKING"
	case WorkerEventNameConnecting:
		return "CONNECTING"
	case WorkerEventNameConnected:
		return "CONNECTED"
	}
	return "UNKNOWN"
}

type WorkerEvent struct {
	Worker  string
	Name    WorkerEventName
	Message string
}
