package yggdrasil

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"google.golang.org/grpc"

	pb "github.com/redhatinsights/yggdrasil/protocol"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// An Assignment contains data required for a specific job to dispatch to an
// available worker.
type Assignment struct {
	id      string
	handler string
	payload []byte
}

// A Dispatcher routes messages received over an MQTT topic to job controllers,
// depending on the message type.
type Dispatcher struct {
	pb.UnimplementedDispatcherServer
	in                 chan Assignment
	out                chan Assignment
	logger             *log.Logger
	pendingAssignments map[string]*Assignment
	assignmentsLock    sync.RWMutex
	registeredWorkers  map[string]string
	workersLock        sync.RWMutex
}

// NewDispatcher cretes a new dispatcher, configured with an appropriate HTTP
// client for reporting results.
func NewDispatcher(in, out chan Assignment) *Dispatcher {
	return &Dispatcher{
		in:                 in,
		out:                out,
		pendingAssignments: make(map[string]*Assignment),
		registeredWorkers:  make(map[string]string),
		logger:             log.New(log.Writer(), fmt.Sprintf("%v[dispatcher] ", log.Prefix()), log.Flags(), log.CurrentLevel()),
	}
}

// ListenAndServe opens a UNIX domain socket, registers a Dispatcher service with
// grpc and accepts incoming connections on the domain socket.
func (d *Dispatcher) ListenAndServe() error {
	go func() {
		for {
			assignment := <-d.in
			d.logger.Trace("assignment received")
			d.workersLock.RLock()
			workerSocketAddr := d.registeredWorkers[assignment.handler]
			d.workersLock.RUnlock()

			work := pb.Work{
				Id:   assignment.id,
				Data: assignment.payload,
			}

			d.logger.Tracef("dialing worker %v", assignment.handler)
			conn, err := grpc.Dial("unix:"+workerSocketAddr, grpc.WithInsecure())
			if err != nil {
				d.logger.Error(err)
				return
			}
			defer conn.Close()

			c := pb.NewWorkerClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			d.logger.Trace("dispatching work")
			r, err := c.Start(ctx, &work)
			if err != nil {
				d.logger.Error(err)
				return
			}
			if !r.GetAccepted() {
				d.logger.Errorf("work %v rejected by worker %v", work.String(), workerSocketAddr)
				return
			}
			d.assignmentsLock.Lock()
			d.pendingAssignments[assignment.id] = &assignment
			d.assignmentsLock.Unlock()
		}
	}()

	socketAddr := "@yggd-dispatcher"

	l, err := net.Listen("unix", socketAddr)
	if err != nil {
		return err
	}

	s := grpc.NewServer()
	pb.RegisterDispatcherServer(s, d)
	if err := s.Serve(l); err != nil {
		return err
	}
	return nil
}

// Register implements the "Register" RPC method of the Manager service.
func (d *Dispatcher) Register(ctx context.Context, r *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	d.workersLock.Lock()
	defer d.workersLock.Unlock()

	if _, ok := d.registeredWorkers[r.GetHandler()]; ok {
		d.logger.Debugf("worker register rejected: %v", r.GetHandler())
		return &pb.RegisterResponse{
			Registered: false,
			Reason:     "already registered",
		}, nil
	}

	socketAddr := fmt.Sprintf("@ygg-%v-%v", r.GetHandler(), randomString(6))
	d.workersLock.Lock()
	d.registeredWorkers[r.GetHandler()] = socketAddr
	d.workersLock.Unlock()
	d.logger.Debugf("worker register accepted: %v", socketAddr)
	return &pb.RegisterResponse{
		Registered: true,
		Address:    socketAddr,
	}, nil
}

// Finish implements the "Finish" RPC method of the Manager service.
func (d *Dispatcher) Finish(ctx context.Context, r *pb.Work) (*pb.Empty, error) {
	d.assignmentsLock.RLock()
	assignment, prs := d.pendingAssignments[r.GetId()]
	d.assignmentsLock.Unlock()
	if !prs {
		return nil, fmt.Errorf("missing assignment %v", r.GetId())
	}

	d.out <- Assignment{
		id:      assignment.id,
		handler: assignment.handler,
		payload: r.GetData(),
	}

	d.assignmentsLock.Lock()
	delete(d.pendingAssignments, r.GetId())
	d.assignmentsLock.Unlock()

	return &pb.Empty{}, nil
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := make([]byte, n)
	for i := range data {
		data[i] = letters[rand.Intn(len(letters))]
	}
	return string(data)
}
