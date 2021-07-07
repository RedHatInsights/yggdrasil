package http

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/clients/http"
	"github.com/redhatinsights/yggdrasil/internal/transport"
)

type Transport struct {
	ClientID        string
	HttpClient      *http.Client
	Server          string
	controlHandler  transport.CommandHandler
	dataHandler     transport.DataHandler
	pollingInterval time.Duration
	killSwitch      chan struct{}
}

func NewHTTPTransport(ClientID string, server string, tlsConfig *tls.Config, userAgent string,
	pollingInterval time.Duration, controlHandler transport.CommandHandler,
	dataHandler transport.DataHandler) (*Transport, error) {
	return &Transport{
		Server:          server,
		ClientID:        ClientID,
		HttpClient:      http.NewHTTPClient(tlsConfig, userAgent),
		controlHandler:  controlHandler,
		dataHandler:     dataHandler,
		pollingInterval: pollingInterval,
	}, nil
}

func (t Transport) Start() error {
	t.killSwitch = make(chan struct{})
	go func() {
		for {
			select {
			case <-t.killSwitch:
				return
			default:
				payload, err := t.HttpClient.Get(t.getUrl("in", "control"))
				if err != nil {
					log.Errorf("Error while getting work: %v", err)
				}
				if payload != nil && len(payload) > 0 {
					t.controlHandler(payload, t)
				}
				time.Sleep(t.pollingInterval)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-t.killSwitch:
				return
			default:
				payload, err := t.HttpClient.Get(t.getUrl("in", "data"))
				if err != nil {
					log.Errorf("Error while getting work: %v", err)
				}
				if payload != nil && len(payload) > 0 {
					t.dataHandler(payload)
				}
				time.Sleep(t.pollingInterval)
			}
		}
	}()

	return nil
}

func (t Transport) SendData(data yggdrasil.Data) error {
	return t.send(data, "data")
}

func (t Transport) SendControl(ctrlMsg interface{}) error {
	return t.send(ctrlMsg, "control")
}

func (t Transport) Disconnect(quiesce uint) {
	time.Sleep(time.Millisecond * time.Duration(quiesce))
	t.killSwitch <- struct{}{}
}

func (t Transport) send(message interface{}, channel string) error {
	url := t.getUrl("out", channel)
	dataBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	log.Tracef("Sending %s", string(dataBytes))
	return t.HttpClient.Post(url, headers, dataBytes)
}

func (t Transport) getUrl(direction string, channel string) string {
	return fmt.Sprintf("http://%s/api/k4e-management/v1/%s/%s/%s", t.Server, channel, t.ClientID, direction)
}
