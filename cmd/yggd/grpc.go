package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/redhatinsights/yggdrasil/internal/http"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

var (
	sendResponseTimeout = 20 * time.Second
)

// DataMessage is similar to Data, but adds a channel where the response will
// be send.
type DataMessage struct {
	// Data structure to be send to the transport.
	Data *yggdrasil.Data

	// Response channel where transport will send the response back when he finished.
	Response chan []byte
}

type worker struct {
	pid             int
	handler         string
	addr            string
	features        map[string]string
	detachedContent bool
}

type dispatcher struct {
	pb.UnimplementedDispatcherServer
	dispatchers chan map[string]map[string]string
	sendQ       chan yggdrasil.Data
	recvQ       chan DataMessage
	deadWorkers chan int
	reg         registry
	pidHandlers map[int]string
	httpClient  *http.Client
}

func newDispatcher(httpClient *http.Client) *dispatcher {
	return &dispatcher{
		dispatchers: make(chan map[string]map[string]string),
		sendQ:       make(chan yggdrasil.Data),
		recvQ:       make(chan DataMessage),
		deadWorkers: make(chan int),
		reg:         registry{},
		pidHandlers: make(map[int]string),
		httpClient:  httpClient,
	}
}

func (d *dispatcher) Register(ctx context.Context, r *pb.RegistrationRequest) (*pb.RegistrationResponse, error) {
	if d.reg.get(r.GetHandler()) != nil {
		log.Errorf("worker failed to register for handler %v", r.GetHandler())
		return &pb.RegistrationResponse{Registered: false}, nil
	}

	w := worker{
		pid:             int(r.GetPid()),
		handler:         r.GetHandler(),
		addr:            fmt.Sprintf("@ygg-%v-%v", r.GetHandler(), randomString(6)),
		features:        r.GetFeatures(),
		detachedContent: r.GetDetachedContent(),
	}

	if err := d.reg.set(r.GetHandler(), &w); err != nil {
		return &pb.RegistrationResponse{Registered: false}, nil
	}
	d.pidHandlers[int(r.GetPid())] = r.GetHandler()

	log.Infof("worker registered: %+v", w)

	d.sendDispatchersMap()

	return &pb.RegistrationResponse{Registered: true, Address: w.addr}, nil
}

func (d *dispatcher) GetConfig(ctx context.Context, _ *pb.Empty) (*pb.Config, error) {
	return &pb.Config{
		LogLevel:   DefaultConfig.LogLevel,
		ClientId:   DefaultConfig.ClientID,
		SocketAddr: DefaultConfig.SocketAddr,
		Server:     DefaultConfig.Server,
		CertFile:   DefaultConfig.CertFile,
		KeyFile:    DefaultConfig.KeyFile,
		CaRoot:     strings.Join(DefaultConfig.CARoot, ";"),
	}, nil
}

func (d *dispatcher) Send(ctx context.Context, r *pb.Data) (*pb.Response, error) {
	data := &yggdrasil.Data{
		Type:       yggdrasil.MessageTypeData,
		MessageID:  r.GetMessageId(),
		ResponseTo: r.GetResponseTo(),
		Version:    1,
		Sent:       time.Now(),
		Directive:  r.GetDirective(),
		Metadata:   r.GetMetadata(),
		Content:    r.GetContent(),
	}

	URL, err := url.Parse(data.Directive)
	if err != nil {
		e := fmt.Errorf("cannot parse message content as URL: %w", err)
		log.Error(e)
		return nil, e
	}

	var res []byte
	if URL.Scheme == "" {
		resC := make(chan []byte, 1)
		d.recvQ <- DataMessage{Data: data, Response: resC}
		select {
		case <-time.After(sendResponseTimeout):
			log.Error("No response from recvQ queue")
		case val := <-resC:
			res = val
		}
	} else {
		if yggdrasil.DataHost != "" {
			URL.Host = yggdrasil.DataHost
		}
		httpRes, err := d.httpClient.Post(URL.String(), data.Metadata, data.Content)
		if err != nil {
			e := fmt.Errorf("cannot post detached message content: %w", err)
			log.Error(e)
			return nil, e
		}
		res, err = json.Marshal(httpRes)
		if err != nil {
			e := fmt.Errorf("cannot marshal post detached content: %w", err)
			log.Error(e)
			return nil, e
		}
	}
	log.Debugf("received message %v", data.MessageID)
	log.Tracef("message: %+v", data.Content)

	return &pb.Response{
		Response: res,
	}, nil
}

// DisconnectWorkers sends a RECEIVED_DISCONNECT event message to all registered
// workers.
func (d *dispatcher) DisconnectWorkers() {
	for _, w := range d.reg.all() {
		if err := d.disconnectWorker(w); err != nil {
			log.Errorf("cannot disconnect worker %v: %v", w, err)
		}
	}
}

// disconnectWorker creates and sends a RECEIVED_DISCONNECT event message to
// worker w.
func (d *dispatcher) disconnectWorker(w *worker) error {
	conn, err := grpc.Dial("unix:"+w.addr, grpc.WithInsecure())
	if err != nil {
		log.Errorf("cannot dial socket: %v", err)
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	workerClient := pb.NewWorkerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err = workerClient.NotifyEvent(ctx, &pb.EventNotification{Name: pb.Event_RECEIVED_DISCONNECT})
	if err != nil {
		log.Errorf("cannot disconnect worker %v: %v", w, err)
		return err
	}
	return nil
}

// sendData receives values on a channel and sends the data over gRPC
func (d *dispatcher) sendData() {
	for data := range d.sendQ {
		f := func() {
			w := d.reg.get(data.Directive)

			if w == nil {
				log.Warnf("cannot route message  %v to directive: %v", data.MessageID, data.Directive)
				return
			}

			if w.detachedContent {
				var urlString string
				if err := json.Unmarshal(data.Content, &urlString); err != nil {
					log.Errorf("cannot unmarshal message content: %v", err)
					return
				}
				URL, err := url.Parse(urlString)
				if err != nil {
					log.Errorf("cannot parse message content as URL: %v", err)
					return
				}
				if yggdrasil.DataHost != "" {
					URL.Host = yggdrasil.DataHost
				}

				resp, err := d.httpClient.Get(URL.String())
				if err != nil {
					log.Errorf("cannot get detached message content: %v", err)
					return
				}
				data.Content = resp.Body
			}

			conn, err := grpc.Dial("unix:"+w.addr, grpc.WithInsecure())
			if err != nil {
				log.Errorf("cannot dial socket: %v", err)
				return
			}
			defer conn.Close()

			c := pb.NewWorkerClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			msg := pb.Data{
				MessageId:  data.MessageID,
				ResponseTo: data.ResponseTo,
				Directive:  data.Directive,
				Metadata:   data.Metadata,
				Content:    data.Content,
			}
			_, err = c.Send(ctx, &msg)
			if err != nil {
				log.Errorf("cannot send message %v: %v", data.MessageID, err)
				log.Tracef("message: %+v", data)
				return
			}
			log.Debugf("dispatched message %v to worker %v", msg.MessageId, data.Directive)
		}

		f()
	}
}

func (d *dispatcher) unregisterWorker() {
	for pid := range d.deadWorkers {
		handler := d.pidHandlers[pid]
		delete(d.pidHandlers, pid)
		d.reg.del(handler)
		log.Infof("unregistered worker: %v", handler)

		d.sendDispatchersMap()
	}
}

func (d *dispatcher) makeDispatchersMap() map[string]map[string]string {
	dispatchers := make(map[string]map[string]string)
	for handler, worker := range d.reg.all() {
		dispatchers[handler] = worker.features
	}

	return dispatchers
}

func (d *dispatcher) sendDispatchersMap() {
	d.dispatchers <- d.makeDispatchersMap()
}
