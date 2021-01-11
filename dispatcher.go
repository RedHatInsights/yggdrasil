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

	// SignalWorkAssign is emitted when an Assignment is assigned to a worker.
	// The value emitted on the channel is a yggdrasil.Assignment.
	SignalWorkAssign = "work-assign"

	// SignalWorkComplete is emitted when an Assignment is reported as finished
	// by its worker. The value emitted on the channel is a yggdrasil.Assignment.
	SignalWorkComplete = "work-complete"

	// SignalWorkerRegister is emitted when a worker is successfully registered
	// as handling a specified work type. The value emitted on the channel is
	// a yggdrasil.Worker.
	SignalWorkerRegister = "worker-register"

	// SignalWorkerUnregister is emitted when a worker is removed from the
	// handler table. The value emitted on the channel is a yggdrasil.Worker.
	SignalWorkerUnregister = "worker-unregister"
)

// Assignment is a basic data structure that holds data necessary to create
// an Assignment protobuf message.
type Assignment struct {
	ID       string
	Data     []byte
	Complete bool
	Handler  string
	Headers  map[string]string
}

// ToProtobuf converts the assignment into a protobuf Assignment message.
func (a *Assignment) ToProtobuf() *pb.Assignment {
	return &pb.Assignment{
		Id:       a.ID,
		Data:     a.Data,
		Complete: a.Complete,
		Handler:  a.Handler,
		Headers:  a.Headers,
	}
}

// FromProtobuf fills the assignment with values from the given Assignment
// message.
func (a *Assignment) FromProtobuf(p *pb.Assignment) {
	a.ID = p.GetId()
	a.Data = p.GetData()
	a.Complete = p.GetComplete()
	a.Handler = p.GetHandler()
	a.Headers = p.GetHeaders()
}

// Worker holds values associated with an actively registered worker process.
type Worker struct {
	pid        int
	handler    string
	socketAddr string
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
func NewDispatcher() (*Dispatcher, error) {
	d := new(Dispatcher)
	d.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), d), log.Flags(), log.CurrentLevel())

	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"worker": {
				Name: "worker",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "pid"},
					},
					"handler": {
						Name:    "handler",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "handler"},
					},
				},
			},
			"assignment": {
				Name: "assignment",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "ID"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	d.db = db

	return d, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (d *Dispatcher) Connect(name string) <-chan interface{} {
	return d.sig.connect(name, 1)
}

// ListenAndServe opens a UNIX domain socket, registers a Dispatcher service with
// grpc and accepts incoming connections on the domain socket.
func (d *Dispatcher) ListenAndServe() error {
	socketAddr := "@yggd-dispatcher"
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
func (d *Dispatcher) Register(ctx context.Context, r *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	d.logger.Debugf("Register(%v)", r)
	tx := d.db.Txn(true)
	defer tx.Abort()

	if _, err := tx.First("worker", "id", int(r.GetPid())); err != nil {
		return &pb.RegisterResponse{
			Registered: false,
			Reason:     fmt.Sprintf("already registered: %v: %v", r.GetPid(), err),
		}, nil
	}
	if _, err := tx.First("worker", "handler", r.GetHandler()); err != nil {
		return &pb.RegisterResponse{
			Registered: false,
			Reason:     fmt.Sprintf("already registered: %v: %v", r.GetHandler(), err),
		}, nil
	}

	w := Worker{
		pid:        int(r.GetPid()),
		handler:    r.GetHandler(),
		socketAddr: fmt.Sprintf("@ygg-%v-%v", r.GetHandler(), randomString(6)),
	}

	if err := tx.Insert("worker", &w); err != nil {
		return nil, err
	}

	d.sig.emit(SignalWorkerRegister, w)
	d.logger.Debugf("emitted signal \"%v\"", SignalWorkerRegister)
	d.logger.Tracef("emitted value: %#v", w)

	tx.Commit()

	return &pb.RegisterResponse{
		Registered: true,
		Address:    w.socketAddr,
	}, nil
}

// Update implements the "Update" RPC method of the Dispatcher service.
func (d *Dispatcher) Update(ctx context.Context, r *pb.Assignment) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

// Finish implements the "Finish" RPC method of the Dispatcher service.
func (d *Dispatcher) Finish(ctx context.Context, r *pb.Assignment) (*pb.Empty, error) {
	var a Assignment
	a.FromProtobuf(r)

	d.sig.emit(SignalWorkComplete, a)
	d.logger.Debugf("emitted signal \"%v\"", SignalWorkComplete)
	d.logger.Tracef("emitted value: %#v", a)

	return &pb.Empty{}, nil
}

// HandleProcessDieSignal receives values on the channel, looks up the PID in
// its registry of active workers and removes the worker.
func (d *Dispatcher) HandleProcessDieSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			process := e.(Process)
			d.logger.Debug("HandleProcessDieSignal")
			d.logger.Tracef("emitted value: %#v", process)

			tx := d.db.Txn(true)
			defer tx.Abort()

			obj, err := tx.First("worker", "id", process.pid)
			if err != nil {
				d.logger.Error(err)
				return
			}
			if obj == nil {
				d.logger.Errorf("unknown worker with PID %v", process.pid)
				return
			}

			if err := tx.Delete("worker", obj); err != nil {
				d.logger.Error(err)
				return
			}
			worker := obj.(*Worker)

			d.sig.emit(SignalWorkerUnregister, *worker)
			d.logger.Debugf("emitted signal \"%v\"", SignalWorkerUnregister)
			d.logger.Tracef("emitted value: %#v", *worker)

			tx.Commit()
		}()
	}
}

// HandleAssignmentCreateSignal receives values on the channel, looks up the
// requested handler in its registry of active workers and dispatches the
// assignment to the chosen worker.
func (d *Dispatcher) HandleAssignmentCreateSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			assignment := e.(Assignment)
			d.logger.Debug("HandleAssignmentCreateSignal")
			d.logger.Tracef("emitted value: %#v", assignment)

			var tx *memdb.Txn

			tx = d.db.Txn(false)
			obj, err := tx.First("worker", "handler", assignment.Handler)
			if err != nil {
				d.logger.Error(err)
				return
			}
			if obj == nil {
				d.logger.Errorf("no worker detected for handler %v", assignment.Handler)
				return
			}
			worker := obj.(*Worker)

			d.logger.Debugf("dialing worker %v", assignment.Handler)
			conn, err := grpc.Dial("unix:"+worker.socketAddr, grpc.WithInsecure())
			if err != nil {
				d.logger.Error(err)
				return
			}
			defer conn.Close()

			c := pb.NewWorkerClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			msg := assignment.ToProtobuf()
			r, err := c.Start(ctx, msg)
			if err != nil {
				d.logger.Error(err)
				return
			}

			tx = d.db.Txn(true)
			if err := tx.Insert("assignment", &assignment); err != nil {
				d.logger.Error(err)
				tx.Abort()
				return
			}
			tx.Commit()

			d.logger.Debugf("dispatched work: %v", assignment.ID)
			if !r.GetAccepted() {
				d.logger.Errorf("work %v rejected by worker %v", assignment.ID, worker.socketAddr)
				return
			}

			d.sig.emit(SignalWorkAssign, assignment)
			d.logger.Debugf("emitted signal \"%v\"", SignalWorkAssign)
			d.logger.Tracef("emitted value: %#v", assignment)
		}()
	}
}

// HandleAssignmentReturnSignal receives values on the channel and removes the
// assignment from its table of pending assignments.
func (d *Dispatcher) HandleAssignmentReturnSignal(c <-chan interface{}) {
	for e := range c {
		func() {
			assignment := e.(Assignment)
			d.logger.Debug("HandleAssignmentReturnSignal")
			d.logger.Tracef("emitted value: %#v", assignment)

			var tx *memdb.Txn

			tx = d.db.Txn(false)
			obj, err := tx.First("assignment", "id", assignment.ID)
			if err != nil {
				d.logger.Error(err)
				return
			}
			d.logger.Tracef("found stored assignment: %#v", obj)

			tx = d.db.Txn(true)
			if err := tx.Delete("assignment", obj); err != nil {
				d.logger.Error(err)
				tx.Abort()
				return
			}
			tx.Commit()

			d.logger.Debugf("deleted complete assignment: %#v", obj)

		}()
	}
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := make([]byte, n)
	for i := range data {
		data[i] = letters[rand.Intn(len(letters))]
	}
	return string(data)
}
