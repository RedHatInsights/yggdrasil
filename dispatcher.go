package yggdrasil

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"git.sr.ht/~spc/go-log"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-memdb"
	pb "github.com/redhatinsights/yggdrasil/protocol"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Assignment is a basic data structure that holds data necessary to create
// an Assignment protobuf message.
type Assignment struct {
	ID       string
	Data     []byte
	Complete bool
	Handler  string
}

// A Worker is a worker that has registered with the dispatcher.
type Worker struct {
	pid        int64
	handler    string
	socketAddr string
}

// A Dispatcher routes messages received over an MQTT topic to job controllers,
// depending on the message type.
type Dispatcher struct {
	pb.UnimplementedDispatcherServer
	in         chan *Assignment
	out        chan *Assignment
	workerDied <-chan int64
	logger     *log.Logger
	db         *memdb.MemDB
}

// NewDispatcher cretes a new dispatcher, configured with an appropriate HTTP
// client for reporting results.
func NewDispatcher(in, out chan *Assignment, workerDied <-chan int64) (*Dispatcher, error) {
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

	return &Dispatcher{
		in:         in,
		out:        out,
		workerDied: workerDied,
		logger:     log.New(log.Writer(), fmt.Sprintf("%v[dispatcher] ", log.Prefix()), log.Flags(), log.CurrentLevel()),
		db:         db,
	}, nil
}

// ListenAndServe opens a UNIX domain socket, registers a Dispatcher service with
// grpc and accepts incoming connections on the domain socket.
func (d *Dispatcher) ListenAndServe() error {
	go d.reapWorkers()
	go d.receiveAssignments()

	socketAddr := "@yggd-dispatcher"

	l, err := net.Listen("unix", socketAddr)
	if err != nil {
		return err
	}
	d.logger.Tracef("listening on %v", socketAddr)

	s := grpc.NewServer()
	pb.RegisterDispatcherServer(s, d)
	if err := s.Serve(l); err != nil {
		return err
	}
	return nil
}

// Register implements the "Register" RPC method of the Dispatcher service.
func (d *Dispatcher) Register(ctx context.Context, r *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	tx := d.db.Txn(true)
	defer tx.Abort()

	if _, err := tx.First("worker", "id", r.GetPid()); err != nil {
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
		pid:        r.GetPid(),
		handler:    r.GetHandler(),
		socketAddr: fmt.Sprintf("@ygg-%v-%v", r.GetHandler(), randomString(6)),
	}

	if err := tx.Insert("worker", w); err != nil {
		return nil, err
	}
	d.logger.Debugf("worker registered %#v", w)

	tx.Commit()

	return &pb.RegisterResponse{
		Registered: true,
		Address:    w.socketAddr,
	}, nil
}

// Finish implements the "Finish" RPC method of the Dispatcher service.
func (d *Dispatcher) Finish(ctx context.Context, r *pb.Assignment) (*pb.Empty, error) {
	tx := d.db.Txn(true)
	defer tx.Abort()

	obj, err := tx.First("assignment", "id", r.GetId())
	if err != nil {
		return nil, err
	}
	assignment := obj.(*Assignment)

	d.out <- assignment

	if err := tx.Delete("assignment", obj); err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

// reapWorkers receives PIDs on the "workerDied" channel and deletes them from
// the dispatcher worker registry.
func (d *Dispatcher) reapWorkers() {
	reapHandlerFunc := func(pid int64) error {
		d.logger.Tracef("worker died: %v", pid)
		tx := d.db.Txn(true)
		defer tx.Abort()

		w, err := tx.First("worker", "id", pid)
		if err != nil {
			return err
		}
		if err := tx.Delete("worker", w); err != nil {
			return err
		}
		d.logger.Tracef("delete worker %#v", w)

		tx.Commit()

		return nil
	}
	for {
		pid := <-d.workerDied
		if err := reapHandlerFunc(pid); err != nil {
			d.logger.Error(err)
		}
	}
}

// receiveAssignments retrieves assignments off the "in" channel and dispatches
// them to a worker.
func (d *Dispatcher) receiveAssignments() {
	receiveAssignmentsFunc := func(assignment *Assignment) error {
		d.logger.Trace("assignment received")

		tx := d.db.Txn(true)
		defer tx.Abort()

		obj, err := tx.First("worker", "handler", assignment.Handler)
		if err != nil {
			return err
		}
		worker := obj.(Worker)

		d.logger.Tracef("dialing worker %v", assignment.Handler)
		conn, err := grpc.Dial("unix:"+worker.socketAddr, grpc.WithInsecure())
		if err != nil {
			return err
		}
		defer conn.Close()

		c := pb.NewWorkerClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		msg := &pb.Assignment{
			Id:       assignment.ID,
			Data:     assignment.Data,
			Complete: assignment.Complete,
			Handler:  assignment.Handler,
		}
		r, err := c.Start(ctx, msg)
		if err != nil {
			return err
		}
		d.logger.Tracef("dispatched work: %v", assignment.ID)
		if !r.GetAccepted() {
			return fmt.Errorf("work %v rejected by worker %v", assignment.ID, worker.socketAddr)
		}

		if err := tx.Insert("assignment", assignment); err != nil {
			return err
		}

		tx.Commit()

		return nil
	}
	for {
		assignment := <-d.in
		if err := receiveAssignmentsFunc(assignment); err != nil {
			d.logger.Error(err)
		}
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
