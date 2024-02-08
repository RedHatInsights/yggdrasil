package ipc

import (
	_ "embed"
	"fmt"
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

	// DispatcherEventConnectionRestored is emitted when the transport reconnects
	// to the network.
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

	// WorkerEventNameWorking is emitted when the worker wishes
	// to continue to announce it is working.
	WorkerEventNameWorking WorkerEventName = 3

	// WorkerEventNameStarted is emitted when worker finished starting
	// process, and it can start process received messages.
	WorkerEventNameStarted WorkerEventName = 4

	// WorkerEventNameStopped is emitted when worker is stopped,
	// and it cannot process any message.
	WorkerEventNameStopped WorkerEventName = 5
)

const WorkerEventNameMap := make(map[WorkerEventName]string) {
	WorkerEventNameBegin: "BEGIN",
	WorkerEventNameEnd: "END",
	WorkerEventNameWorking: "WORKING",
	WorkerEventNameStarted: "STARTED",
	WorkerEventNameStopped: "STOPPED",
}

func (e WorkerEventName) String() string {
	val, found := WorkerEventNameMap[e]
	if found {
		return val
	} else {
		return fmt.Sprintf("UNKNOWN (value: %d)", e)
	}
}

type WorkerEvent struct {
	Worker     string
	Name       WorkerEventName
	MessageID  string
	ResponseTo string
	Data       map[string]string
}
