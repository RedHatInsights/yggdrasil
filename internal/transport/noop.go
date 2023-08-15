package transport

import "crypto/tls"

// Noop is a Transporter that does nothing. This Transport is used to enable a
// "local only" dispatch mode.
type Noop struct{}

func NewNoopTransport() (*Noop, error) {
	return &Noop{}, nil
}

func (t *Noop) Connect() error {
	return nil
}

func (t *Noop) Disconnect(quiesce uint) {}

func (t *Noop) Tx(addr string, metadata map[string]string, data []byte) (responseCode int, responseMetadata map[string]string, responseData []byte, err error) {
	return
}

func (t *Noop) SetRxHandler(f RxHandlerFunc) error {
	return nil
}

func (t *Noop) ReloadTLSConfig(tlsConfig *tls.Config) error {
	return nil
}

func (t *Noop) SetEventHandler(f EventHandlerFunc) error {
	return nil
}
