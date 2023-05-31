package transport

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil/internal/config"
	internalhttp "github.com/redhatinsights/yggdrasil/internal/http"
)

// HTTPResponse is a data structure representing an HTTP response received from
// an HTTP request sent through the transport.
type HTTPResponse struct {
	StatusCode int
	Body       json.RawMessage
	Metadata   map[string]string
}

// HTTP is a Transporter that sends and receives data and control
// messages by sending HTTP requests to a URL.
type HTTP struct {
	clientID        string
	client          *internalhttp.Client
	server          string
	dataHandler     RxHandlerFunc
	pollingInterval time.Duration
	disconnected    atomic.Value
	userAgent       string
	isTLS           atomic.Value
	events          chan TransporterEvent
	eventHandler    EventHandlerFunc
}

func NewHTTPTransport(clientID string, server string, tlsConfig *tls.Config, userAgent string, pollingInterval time.Duration) (*HTTP, error) {
	disconnected := atomic.Value{}
	disconnected.Store(false)
	isTls := atomic.Value{}
	isTls.Store(tlsConfig != nil)
	return &HTTP{
		clientID:        clientID,
		client:          internalhttp.NewHTTPClient(tlsConfig.Clone(), userAgent),
		pollingInterval: pollingInterval,
		disconnected:    disconnected,
		server:          server,
		userAgent:       userAgent,
		isTLS:           isTls,
		events:          make(chan TransporterEvent),
	}, nil
}

func (t *HTTP) Connect() error {
	t.disconnected.Store(false)

	go func() {
		for event := range t.events {
			if t.eventHandler == nil {
				continue
			}
			t.eventHandler(event)
		}
	}()

	go func() {
		for {
			if t.disconnected.Load().(bool) {
				return
			}
			resp, err := t.client.Get(t.getUrl("in", "control"))
			if err != nil {
				log.Tracef("cannot get HTTP request: %v", err)
			}
			if resp != nil {
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Errorf("cannot read response body: %v", err)
					continue
				}
				if t.dataHandler != nil {
					metadata := make(map[string]interface{})
					for k, v := range resp.Header {
						metadata[k] = v
					}
					_ = t.dataHandler("control", metadata, data)
				}
				resp.Body.Close()
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

			if resp != nil {
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Errorf("cannot read response body: %v", err)
					continue
				}
				if t.dataHandler != nil {
					metadata := make(map[string]interface{})
					for k, v := range resp.Header {
						metadata[k] = v
					}
					_ = t.dataHandler("data", metadata, data)
				}
				resp.Body.Close()
			}
			time.Sleep(t.pollingInterval)
		}
	}()

	t.events <- TransporterEventConnected

	return nil
}

// ReloadTLSConfig creates a new HTTP client with the provided TLS config.
func (t *HTTP) ReloadTLSConfig(tlsConfig *tls.Config) error {
	*t.client = *internalhttp.NewHTTPClient(tlsConfig, t.userAgent)
	t.isTLS.Store(tlsConfig != nil)
	return nil
}

func (t *HTTP) Disconnect(quiesce uint) {
	time.Sleep(time.Millisecond * time.Duration(quiesce))
	t.disconnected.Store(true)
	t.events <- TransporterEventDisconnected
}

func (t *HTTP) Tx(addr string, metadata map[string]string, data []byte) (responseCode int, responseMetadata map[string]string, responseData []byte, err error) {
	if t.disconnected.Load().(bool) {
		return TxResponseErr, nil, nil, fmt.Errorf("cannot perform Tx: transport is disconnected")
	}
	url := t.getUrl("out", addr)
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	resp, err := t.client.Post(url, headers, data)
	if err != nil && resp == nil {
		return TxResponseErr, nil, nil, fmt.Errorf("cannot perform HTTP request: %w", err)
	}

	responseCode = resp.StatusCode
	responseMetadata = make(map[string]string)
	for k, v := range resp.Header {
		responseMetadata[k] = strings.Join(v, ";")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TxResponseErr, nil, nil, fmt.Errorf("cannot read HTTP response body: %w", err)
	}
	defer resp.Body.Close()

	responseData = body

	if resp.StatusCode >= 400 {
		err = fmt.Errorf("%v", http.StatusText(resp.StatusCode))
	}

	return
}

func (t *HTTP) SetRxHandler(f RxHandlerFunc) error {
	t.dataHandler = f
	return nil
}

func (t *HTTP) SetEventHandler(f EventHandlerFunc) error {
	t.eventHandler = f
	return nil
}

func (t *HTTP) getUrl(direction string, channel string) string {
	protocol := "http"
	if t.isTLS.Load().(bool) {
		protocol = "https"
	}
	path := filepath.Join(config.DefaultConfig.PathPrefix, channel, t.clientID, direction)

	return fmt.Sprintf("%s://%s/%s", protocol, t.server, path)
}
