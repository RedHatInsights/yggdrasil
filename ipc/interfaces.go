package ipc

import (
	_ "embed"
)

//go:embed com.redhat.yggdrasil.Dispatcher1.xml
var InterfaceDispatcher string

//go:embed com.redhat.yggdrasil.Worker1.xml
var InterfaceWorker string
