package transport

import (
	"github.com/redhatinsights/yggdrasil"
)

type CommandHandler func(command yggdrasil.Command, t Transport)
type DataHandler func(data yggdrasil.Data)

type Transport interface {
	Start() error
	SendData(data yggdrasil.Data) error
	SendControl(ctrlMsg interface{}) error
	Disconnect(quiesce uint)
}

