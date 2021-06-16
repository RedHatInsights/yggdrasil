package yggdrasil

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/hashicorp/go-memdb"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

const (
	// SignalDispatcherListen is emitted when the dispatcher is active and
	// ready for workers. The value emitted on the channel is a bool.
	SignalDispatcherListen = "dispatcher-listen"

	// SignalDataDispatch is emitted when data is dispatched to a worker.
	// The value emitted on the channel is a data message's "MessageID" field in
	// the form of a UUIDv4-formatted string.
	SignalDataDispatch = "data-dispatch"

	// SignalDataReturn is emitted when data is returned by a worker. The value
	// emitted on the channel is a data message's "MessageID" field in the form
	// of a UUIDv4-formatted string.
	SignalDataReturn = "data-return"

	// SignalWorkerRegister is emitted when a worker is successfully registered
	// as handling a specified work type. The value emitted on the channel is
	// a yggdrasil.Worker.
	SignalWorkerRegister = "worker-register"

	// SignalWorkerUnregister is emitted when a worker is removed from the
	// handler table. The value emitted on the channel is the PID of the process
	// that unregistered.
	SignalWorkerUnregister = "worker-unregister"
)

// Worker holds values associated with an actively registered worker process.
type Worker struct {
	pid             int
	handler         string
	socketAddr      string
	detachedContent bool
	features        map[string]string
}

// Dispatcher implements the gRPC Dispatcher service, handling the sending and
// receiving of work assignments over gRPC to worker processes.
type Dispatcher struct {
	pb.UnimplementedDispatcherServer
	logger *log.Logger
	sig    signalEmitter
	db     *memdb.MemDB
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher(db *memdb.MemDB) (*Dispatcher, error) {
	d := new(Dispatcher)
	d.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), d), log.Flags(), log.CurrentLevel())

	d.db = db

	return d, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (d *Dispatcher) Connect(name string) <-chan interface{} {
	return d.sig.connect(name, 1)
}

// Disconnect removes and closes the channel from the signal table under name
// for the caller.
func (d *Dispatcher) Disconnect(name string, ch <-chan interface{}) {
	d.sig.disconnect(name, ch)
}

// Close closes all channels that have been assigned to signal listeners.
func (d *Dispatcher) Close() {
	d.sig.close()
}

// ListenAndServe opens a UNIX domain socket, registers a Dispatcher service
// with grpc and accepts incoming connections on the domain socket.
func (d *Dispatcher) ListenAndServe(socketAddr string) error {
	d.logger.Debugf("ListenAndServe() -> %v", socketAddr)

	l, err := net.Listen("unix", socketAddr)
	if err != nil {
		return err
	}

	s := grpc.NewServer()
	pb.RegisterDispatcherServer(s, d)

	d.sig.emit(SignalDispatcherListen, true)
	d.logger.Debugf("emitted signal \"%v\"", SignalDispatcherListen)
	d.logger.Tracef("emitted value: %#v", true)

	if err := s.Serve(l); err != nil {
		return err
	}
	return nil
}

// Register implements the "Register" RPC method of the Dispatcher service.
func (d *Dispatcher) Register(ctx context.Context, r *pb.RegistrationRequest) (*pb.RegistrationResponse, error) {
	d.logger.Debugf("Register(%v)", r)
	tx := d.db.Txn(true)
	defer tx.Abort()

	if _, err := tx.First(tableNameWorker, indexNameID, int(r.GetPid())); err != nil {
		return &pb.RegistrationResponse{
			Registered: false,
		}, nil
	}
	if _, err := tx.First(tableNameWorker, indexNameHandler, r.GetHandler()); err != nil {
		return &pb.RegistrationResponse{
			Registered: false,
		}, nil
	}

	w := Worker{
		pid:             int(r.GetPid()),
		handler:         r.GetHandler(),
		socketAddr:      fmt.Sprintf("@ygg-%v-%v", r.GetHandler(), randomString(6)),
		detachedContent: r.GetDetachedContent(),
		features:        r.GetFeatures(),
	}

	if err := tx.Insert(tableNameWorker, w); err != nil {
		return nil, err
	}

	tx.Commit()

	d.sig.emit(SignalWorkerRegister, w)
	d.logger.Debugf("emitted signal \"%v\"", SignalWorkerRegister)
	d.logger.Tracef("emitted value: %#v", w)

	return &pb.RegistrationResponse{
		Registered: true,
		Address:    w.socketAddr,
	}, nil
}

// Send implements the "Send" RPC method of the Dispatcher service.
func (d *Dispatcher) Send(ctx context.Context, r *pb.Data) (*pb.Receipt, error) {
	d.logger.Debug("Send")
	d.logger.Tracef("%#v", r)

	var (
		tx  *memdb.Txn
		obj interface{}
		err error
	)

	if r.GetMessageId() == "" {
		return nil, fmt.Errorf("cannot accept message with empty message-id field")
	}

	tx = d.db.Txn(false)
	obj, err = tx.First(tableNameData, indexNameID, r.GetMessageId())
	if err != nil {
		return nil, err
	}
	if obj != nil {
		return nil, fmt.Errorf("existing message with ID %v found", r.GetMessageId())
	}

	dataMessage := Data{
		Type:       MessageTypeData,
		MessageID:  r.GetMessageId(),
		ResponseTo: r.GetResponseTo(),
		Version:    1,
		Sent:       time.Now(),
		Directive:  r.GetDirective(),
		Metadata:   r.GetMetadata(),
		Content:    r.GetContent(),
	}

	tx = d.db.Txn(true)
	if err := tx.Insert(tableNameData, dataMessage); err != nil {
		tx.Abort()
		return nil, err
	}
	tx.Commit()

	d.sig.emit(SignalDataReturn, dataMessage.MessageID)
	d.logger.Debugf("emitted signal \"%v\"", SignalDataReturn)
	d.logger.Tracef("emitted value: %#v", dataMessage.MessageID)

	return &pb.Receipt{}, nil
}

// HandleProcessDieSignal receives values on the channel, looks up the PID in
// its registry of active workers and removes the worker.
func (d *Dispatcher) HandleProcessDieSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			pid := e.(int)
			d.logger.Debug("HandleProcessDieSignal")
			d.logger.Tracef("emitted value: %#v", pid)

			tx := d.db.Txn(true)
			defer tx.Abort()

			obj, err := tx.First(tableNameWorker, indexNameID, pid)
			if err != nil {
				d.logger.Error(err)
				return
			}
			if obj == nil {
				d.logger.Errorf("unknown worker with PID %v", pid)
				return
			}
			worker := obj.(Worker)

			if err := tx.Delete(tableNameWorker, obj); err != nil {
				d.logger.Error(err)
				return
			}
			if _, err := tx.DeleteAll(tableNameData, indexNameHandler, worker.handler); err != nil {
				d.logger.Error(err)
				return
			}
			tx.Commit()

			d.sig.emit(SignalWorkerUnregister, worker.pid)
			d.logger.Debugf("emitted signal \"%v\"", SignalWorkerUnregister)
			d.logger.Tracef("emitted value: %#v", worker.pid)

		}()
	}
}

// HandleDataProcessSignal receives values on the channel, looks up the
// requested handler in its registry of active workers and dispatches the data
// to the chosen worker.
func (d *Dispatcher) HandleDataProcessSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			var (
				tx  *memdb.Txn
				obj interface{}
				err error
			)

			messageID := e.(string)
			d.logger.Debug("HandleDataProcessSignal")
			d.logger.Tracef("emitted value: %#v", messageID)

			tx = d.db.Txn(false)
			obj, err = tx.First(tableNameData, indexNameID, messageID)
			if err != nil {
				d.logger.Error(err)
				return
			}
			if obj == nil {
				d.logger.Errorf("no data message with ID %v", messageID)
				return
			}
			dataMessage := obj.(Data)

			obj, err = tx.First(tableNameWorker, indexNameHandler, dataMessage.Directive)
			if err != nil {
				d.logger.Error(err)
				return
			}
			if obj == nil {
				d.logger.Errorf("no worker detected for handler %v", dataMessage.Directive)
				return
			}
			worker := obj.(Worker)

			d.logger.Debugf("dialing worker %v", dataMessage.Directive)
			conn, err := grpc.Dial("unix:"+worker.socketAddr, grpc.WithInsecure())
			if err != nil {
				d.logger.Error(err)
				return
			}
			defer conn.Close()

			c := pb.NewWorkerClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			msg := pb.Data{
				MessageId:  dataMessage.MessageID,
				ResponseTo: dataMessage.ResponseTo,
				Directive:  dataMessage.Directive,
				Metadata:   dataMessage.Metadata,
				Content:    dataMessage.Content,
			}
			_, err = c.Send(ctx, &msg)
			if err != nil {
				d.logger.Error(err)
				return
			}

			tx = d.db.Txn(true)
			if err := tx.Delete(tableNameData, dataMessage); err != nil {
				d.logger.Error(err)
				return
			}
			tx.Commit()

			d.sig.emit(SignalDataDispatch, dataMessage.MessageID)
			d.logger.Debugf("emitted signal \"%v\"", SignalDataDispatch)
			d.logger.Tracef("emitted value: %#v", dataMessage.MessageID)
		}()
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := make([]byte, n)
	for i := range data {
		data[i] = letters[rand.Intn(len(letters))]
	}
	return string(data)
}
