package transport

type DataRecvHandlerFunc func([]byte, string)

// Transporter is an interface representing the ability to send and receive
// data. It abstracts away the concrete inplementation, leaving that up to the
// implementing type.
type Transporter interface {
	Connect() error
	Disconnect(quiesce uint)
	SendData(data []byte, dest string) error
	RecvData(data []byte, dest string) error
}
