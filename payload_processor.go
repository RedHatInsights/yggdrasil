package yggdrasil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/godbus/dbus/v5"
	"github.com/hashicorp/go-memdb"
)

const (
	// SignalAssignmentCreate is emitted when a "work" message is unmarshaled.
	// The value emitted on the channel is a yggdrasil.Assignment.
	SignalAssignmentCreate = "assignment-create"

	// SignalAssignmentReturn is emitted when a completed "work" message is
	// returned via the work's return URL. The value emitted on the channel is
	// a yggdrasil.Assignment.
	SignalAssignmentReturn = "assignment-return"
)

// PayloadProcessor unmarshals the "Payload" field of a Message and converts or
// dispatches it accordingly.
type PayloadProcessor struct {
	logger *log.Logger
	sig    signalEmitter
	db     *memdb.MemDB
}

// NewPayloadProcessor creates a new payload processor.
func NewPayloadProcessor() (*PayloadProcessor, error) {
	p := new(PayloadProcessor)
	p.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), p), log.Flags(), log.CurrentLevel())

	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"message": {
				Name: "message",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "MessageID"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	p.db = db

	return p, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (p *PayloadProcessor) Connect(name string) <-chan interface{} {
	return p.sig.connect(name, 1)
}

// HandleMessageRecvSignal receives values on the channel, inspects the "Type"
// field and converts the payload to the appropriate type.
func (p *PayloadProcessor) HandleMessageRecvSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			msg := e.(Message)
			p.logger.Debugf("HandleMessageRecvSignal")
			p.logger.Tracef("received value: %#v", msg)

			switch msg.Type {
			case "work":
				var work PayloadWork
				if err := json.Unmarshal([]byte(msg.Payload), &work); err != nil {
					p.logger.Error(err)
					return
				}

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

				resp, err := client.Get(work.PayloadURL)
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

				assignment := Assignment{
					ID:       msg.MessageID,
					Data:     data,
					Complete: false,
					Handler:  work.Handler,
					Headers:  map[string]string{},
				}

				tx := p.db.Txn(true)
				if err := tx.Insert("message", &msg); err != nil {
					p.logger.Error(err)
					return
				}
				tx.Commit()

				p.sig.emit(SignalAssignmentCreate, assignment)
				p.logger.Debugf("emitted signal \"%v\"", SignalAssignmentCreate)
				p.logger.Tracef("emitted value: %#v", assignment)
			default:
				p.logger.Infof("received unhandled message %#v", msg)
			}
		}()
	}
}

// HandleWorkCompleteSignal receives values on the channel, retrieves the
// originating message, inspects the "Type" field and handles the payload
// accordingly.
func (p *PayloadProcessor) HandleWorkCompleteSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			assignment := e.(Assignment)
			p.logger.Debug("HandleWorkCompleteSignal")
			p.logger.Tracef("emitted value: %#v", assignment)

			var tx *memdb.Txn

			tx = p.db.Txn(false)
			obj, err := tx.First("message", "id", assignment.ID)
			if err != nil {
				p.logger.Error(err)
				return
			}
			msg := obj.(*Message)

			switch msg.Type {
			case "work":
				var work PayloadWork
				if err := json.Unmarshal([]byte(msg.Payload), &work); err != nil {
					p.logger.Error(err)
					return
				}

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

				req, err := http.NewRequest(http.MethodPost, work.ReturnURL, bytes.NewReader(assignment.Data))
				if err != nil {
					p.logger.Error(err)
					return
				}

				for k, v := range assignment.Headers {
					req.Header.Add(k, strings.TrimSpace(v))
				}

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

				p.sig.emit(SignalAssignmentReturn, assignment)
				p.logger.Debugf("emitted signal \"%v\"", SignalAssignmentReturn)
				p.logger.Tracef("emitted value: %#v", assignment)
			default:
				p.logger.Infof("unhandled message type: %#v", msg)
			}

			tx = p.db.Txn(true)
			if err := tx.Delete("message", obj); err != nil {
				p.logger.Error(err)
				tx.Abort()
				return
			}
			tx.Commit()
		}()
	}
}
