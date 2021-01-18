package yggdrasil

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"git.sr.ht/~spc/go-log"
	"github.com/godbus/dbus/v5"
	"github.com/hashicorp/go-memdb"
)

const (
	// SignalDataProcess is emitted when a data message is processed and ready
	// for dispatching.
	// The value emitted on the channel is the data message's "MessageID" field
	// in the form of a UUIDv4-formatted string.
	SignalDataProcess = "data-process"

	// SignalDataConsume is emitted when data has been consumed. The value
	// emitted on the channel is the data message's "MessageID" field in the
	// form of a UUIDv4-formatted string.
	SignalDataConsume = "data-consume"
)

// DataProcessor converts data messages and prepares them for dispatch.
type DataProcessor struct {
	logger *log.Logger
	sig    signalEmitter
	db     *memdb.MemDB
}

// NewDataProcessor creates a new data message processor.
func NewDataProcessor(db *memdb.MemDB) (*DataProcessor, error) {
	p := new(DataProcessor)
	p.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), p), log.Flags(), log.CurrentLevel())

	p.db = db

	return p, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (p *DataProcessor) Connect(name string) <-chan interface{} {
	return p.sig.connect(name, 1)
}

// HandleDataRecvSignal receives values on the channel, unpacks the data,
// replaces the content with the contents of the URL if necessary, and emits
// the data on the "data-process" signal.
func (p *DataProcessor) HandleDataRecvSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			var (
				tx  *memdb.Txn
				obj interface{}
				err error
			)

			messageID := e.(string)
			p.logger.Debugf("HandleDataRecvSignal")
			p.logger.Tracef("received value: %#v", messageID)

			tx = p.db.Txn(false)
			obj, err = tx.First(tableNameData, indexNameID, messageID)
			if err != nil {
				p.logger.Error(err)
				return
			}
			dataMessage := obj.(Data)

			tx = p.db.Txn(false)
			obj, err = tx.First(tableNameWorker, indexNameHandler, dataMessage.Directive)
			if err != nil {
				p.logger.Error(err)
				return
			}
			worker := obj.(Worker)

			if worker.detachedPayload {
				conn, err := dbus.SystemBus()
				if err != nil {
					p.logger.Error(err)
					return
				}

				var consumerCertDir string
				if err := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config").Call("com.redhat.RHSM1.Config.Get", dbus.Flags(0), "rhsm.consumercertdir", "").Store(&consumerCertDir); err != nil {
					p.logger.Error(err)
					return
				}

				client, err := NewHTTPClientCertAuth(filepath.Join(consumerCertDir, "cert.pem"), filepath.Join(consumerCertDir, "key.pem"), "")
				if err != nil {
					p.logger.Error(err)
					return
				}

				resp, err := client.Get(string(dataMessage.Content))
				if err != nil {
					p.logger.Error(err)
					return
				}
				defer resp.Body.Close()

				data, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					p.logger.Error(err)
					return
				}

				switch {
				case resp.StatusCode >= 400:
					p.logger.Error(&APIResponseError{Code: resp.StatusCode, body: string(data)})
					return
				}

				dataMessage.Content = data
			}

			tx = p.db.Txn(true)
			if err := tx.Insert(tableNameData, dataMessage); err != nil {
				p.logger.Error(err)
				tx.Abort()
				return
			}
			tx.Commit()

			p.sig.emit(SignalDataProcess, dataMessage.MessageID)
			p.logger.Debugf("emitted signal \"%v\"", SignalDataProcess)
			p.logger.Tracef("emitted value: %#v", dataMessage.MessageID)
		}()
	}
}

// HandleDataReturnSignal receives values on the channel, retrieves the
// originating message, returns the data via HTTP if necessary, removes the
// message from its database and emits the data on the "data-remove" signal.
func (p *DataProcessor) HandleDataReturnSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			var (
				tx  *memdb.Txn
				obj interface{}
				err error
			)

			messageID := e.(string)
			p.logger.Debugf("HandleDataReturnSignal")
			p.logger.Tracef("received value: %#v", messageID)

			tx = p.db.Txn(false)
			obj, err = tx.First(tableNameData, indexNameID, messageID)
			if err != nil {
				p.logger.Error(err)
				return
			}
			dataMessage := obj.(Data)

			tx = p.db.Txn(false)
			obj, err = tx.First(tableNameWorker, indexNameHandler, dataMessage.Directive)
			if err != nil {
				p.logger.Error(err)
				return
			}
			worker := obj.(Worker)

			if worker.detachedPayload {
				conn, err := dbus.SystemBus()
				if err != nil {
					p.logger.Error(err)
					return
				}

				var consumerCertDir string
				if err := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config").Call("com.redhat.RHSM1.Config.Get", dbus.Flags(0), "rhsm.consumercertdir", "").Store(&consumerCertDir); err != nil {
					p.logger.Error(err)
					return
				}

				client, err := NewHTTPClientCertAuth(filepath.Join(consumerCertDir, "cert.pem"), filepath.Join(consumerCertDir, "key.pem"), "")
				if err != nil {
					p.logger.Error(err)
					return
				}

				req, err := http.NewRequest(http.MethodPost, dataMessage.Directive, bytes.NewReader(dataMessage.Content))

				resp, err := client.Do(req)
				if err != nil {
					p.logger.Error(err)
					return
				}
				defer resp.Body.Close()

				data, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					p.logger.Error(err)
					return
				}

				switch {
				case resp.StatusCode >= 400:
					p.logger.Error(&APIResponseError{Code: resp.StatusCode, body: string(data)})
					return
				}
			}

			p.sig.emit(SignalDataConsume, dataMessage.MessageID)
			p.logger.Debugf("emitted signal \"%v\"", SignalDataConsume)
			p.logger.Tracef("emitted value: %#v", dataMessage.MessageID)
		}()
	}
}
