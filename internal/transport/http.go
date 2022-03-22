package transport

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/http"
)

// HTTP is a Transporter that sends and receives data and control
// messages by sending HTTP requests to a URL.
type HTTP struct {
	clientID        string
	client          *http.Client
	server          string
	dataHandler     DataReceiveHandlerFunc
	pollingInterval time.Duration
	disconnected    atomic.Value
	userAgent       string
	isTLS           atomic.Value
}

func NewHTTPTransport(clientID string, server string, tlsConfig *tls.Config, userAgent string, pollingInterval time.Duration, dataRecvFunc DataReceiveHandlerFunc) (*HTTP, error) {
	disconnected := atomic.Value{}
	disconnected.Store(false)
	isTls := atomic.Value{}
	isTls.Store(tlsConfig != nil)
	return &HTTP{
		clientID:        clientID,
		client:          http.NewHTTPClient(tlsConfig.Clone(), userAgent),
		dataHandler:     dataRecvFunc,
		pollingInterval: pollingInterval,
		disconnected:    disconnected,
		server:          server,
		userAgent:       userAgent,
		isTLS:           isTls,
	}, nil
}

func (t *HTTP) Connect() error {
	t.disconnected.Store(false)
	go func() {
		for {
			if t.disconnected.Load().(bool) {
				return
			}
			resp, err := t.client.Get(t.getUrl("in", "control"))
			if err != nil {
				log.Tracef("cannot get HTTP request: %v", err)
			}
			if resp != nil && len(resp.Body) > 0 {
				_ = t.ReceiveData(resp.Body, "control")
			}
			time.Sleep(t.pollingInterval)
		}
	}()

	go func() {
		for {
			if t.disconnected.Load().(bool) {
				return
			}
			resp, err := t.client.Get(t.getUrl("in", "data"))
			if err != nil {
				log.Tracef("cannot get HTTP request: %v", err)
			}

			if resp != nil && len(resp.Body) > 0 {
				_ = t.ReceiveData(resp.Body, "data")
			}
			time.Sleep(t.pollingInterval)
		}
	}()

	return nil
}

// ReloadTLSConfig creates a new HTTP client with the provided TLS config.
func (t *HTTP) ReloadTLSConfig(tlsConfig *tls.Config) error {
	*t.client = *http.NewHTTPClient(tlsConfig, t.userAgent)
	t.isTLS.Store(tlsConfig != nil)
	return nil
}

func (t *HTTP) Disconnect(quiesce uint) {
	time.Sleep(time.Millisecond * time.Duration(quiesce))
	t.disconnected.Store(true)
}

func (t *HTTP) SendData(data []byte, dest string) ([]byte, error) {
	return t.send(data, dest)
}

func (t *HTTP) ReceiveData(data []byte, dest string) error {
	t.dataHandler(data, dest)
	return nil
}

func (t *HTTP) send(message []byte, channel string) ([]byte, error) {
	if t.disconnected.Load().(bool) {
		return nil, nil
	}
	url := t.getUrl("out", channel)
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	log.Tracef("posting HTTP request body: %s", string(message))
	res, err := t.client.Post(url, headers, message)
	if err != nil && res == nil {
		return nil, err
	}
	resBytes, jsonerr := json.Marshal(res)
	if err != nil {
		return resBytes, err
	}
	return resBytes, jsonerr
}

func (t *HTTP) getUrl(direction string, channel string) string {
	protocol := "http"
	if t.isTLS.Load().(bool) {
		protocol = "https"
	}
	path := filepath.Join(yggdrasil.PathPrefix, channel, t.clientID, direction)

	return fmt.Sprintf("%s://%s/%s", protocol, t.server, path)
}
