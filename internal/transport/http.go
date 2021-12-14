package transport

import (
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil/internal/http"
)

// HTTP is a Transporter that sends and receives data and control
// messages by sending HTTP requests to a URL.
type HTTP struct {
	clientID        string
	client          *http.Client
	server          string
	dataHandler     DataRecvHandlerFunc
	pollingInterval time.Duration
	disconnected    atomic.Value
}

func NewHTTPTransport(clientID string, server string, tlsConfig *tls.Config, userAgent string, pollingInterval time.Duration, dataRecvFunc DataRecvHandlerFunc) (*HTTP, error) {
	disconnected := atomic.Value{}
	disconnected.Store(false)
	return &HTTP{
		clientID:        clientID,
		client:          http.NewHTTPClient(tlsConfig.Clone(), userAgent),
		dataHandler:     dataRecvFunc,
		pollingInterval: pollingInterval,
		disconnected:    disconnected,
		server:          server,
	}, nil
}

func (t *HTTP) Connect() error {
	t.disconnected.Store(false)
	go func() {
		for {
			if t.disconnected.Load().(bool) {
				return
			}
			payload, err := t.client.Get(t.getUrl("in", "control"))
			if err != nil {
				log.Tracef("Error while getting work: %v", err)
			}
			if len(payload) > 0 {
				t.RecvData(payload, "control")
			}
			time.Sleep(t.pollingInterval)
		}
	}()

	go func() {
		for {
			if t.disconnected.Load().(bool) {
				return
			}
			payload, err := t.client.Get(t.getUrl("in", "data"))
			if err != nil {
				log.Tracef("Error while getting work: %v", err)
			}
			if len(payload) > 0 {
				t.RecvData(payload, "data")
			}
			time.Sleep(t.pollingInterval)
		}
	}()

	return nil
}

func (t *HTTP) Disconnect(quiesce uint) {
	time.Sleep(time.Millisecond * time.Duration(quiesce))
	t.disconnected.Store(true)
}

func (t *HTTP) SendData(data []byte, dest string) error {
	return t.send(data, dest)
}

func (t *HTTP) RecvData(data []byte, dest string) error {
	t.dataHandler(data, dest)
	return nil
}

func (t *HTTP) send(message []byte, channel string) error {
	if t.disconnected.Load().(bool) {
		return nil
	}
	url := t.getUrl("out", channel)
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	log.Tracef("Sending %s", string(message))
	return t.client.Post(url, headers, message)
}

func (t *HTTP) getUrl(direction string, channel string) string {
	return fmt.Sprintf("http://%s/%s/%s/%s", t.server, channel, t.clientID, direction)
}
