package yggdrasil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"path/filepath"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/godbus/dbus/v5"
	"golang.org/x/crypto/openpgp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	pb "github.com/redhatinsights/yggdrasil/protocol"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// A Dispatcher routes messages received over an MQTT topic to job controllers,
// depending on the message type.
type Dispatcher struct {
	pb.UnimplementedManagerServer
	facts       CanonicalFacts
	httpClient  HTTPClient
	mqttClient  mqtt.Client
	keyring     openpgp.KeyRing
	workers     map[string]string
	assignments map[string]*pb.WorkAssignment
}

// NewDispatcher cretes a new dispatcher, configured with an appropriate HTTP
// client for reporting results.
func NewDispatcher(brokers []string, armoredPublicKeyData []byte) (*Dispatcher, error) {
	facts, err := GetCanonicalFacts()
	if err != nil {
		return nil, err
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	object := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
	var consumerCertDir string
	if err := object.Call("com.redhat.RHSM1.Config.Get", dbus.Flags(0), "rhsm.consumercertdir", "").Store(&consumerCertDir); err != nil {
		return nil, err
	}

	httpClient, err := NewHTTPClientCertAuth(filepath.Join(consumerCertDir, "cert.pem"), filepath.Join(consumerCertDir, "key.pem"), "")
	if err != nil {
		return nil, err
	}

	opts := mqtt.NewClientOptions()
	for _, broker := range brokers {
		opts.AddBroker(broker)
	}
	mqttClient := mqtt.NewClient(opts)

	var entityList openpgp.KeyRing
	if len(armoredPublicKeyData) > 0 {
		reader := bytes.NewReader(armoredPublicKeyData)
		entityList, err = openpgp.ReadArmoredKeyRing(reader)
		if err != nil {
			return nil, err
		}
	}

	return &Dispatcher{
		facts:       *facts,
		httpClient:  *httpClient,
		mqttClient:  mqttClient,
		keyring:     entityList,
		workers:     make(map[string]string),
		assignments: make(map[string]*pb.WorkAssignment),
	}, nil
}

// Connect connects to the MQTT broker.
func (d *Dispatcher) Connect() error {
	if token := d.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// PublishFacts publishes canonical facts to an MQTT topic.
func (d *Dispatcher) PublishFacts() error {
	data, err := json.Marshal(d.facts)
	if err != nil {
		return err
	}

	if token := d.mqttClient.Publish("/in", byte(0), false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// Subscribe adds a message handler to a host-specific topic.
func (d *Dispatcher) Subscribe() error {
	topic := fmt.Sprintf("/out/%v", d.facts.SubscriptionManagerID)
	if token := d.mqttClient.Subscribe(topic, byte(0), d.messageHandler2); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// ListenAndServe opens a UNIX domain socket, registers a Manager service with
// grpc and accepts incoming connections on the domain socket.
func (d *Dispatcher) ListenAndServe() error {
	socketAddr := "@yggd-dispatcher"

	l, err := net.Listen("unix", socketAddr)
	if err != nil {
		return err
	}

	s := grpc.NewServer()
	pb.RegisterManagerServer(s, d)
	if err := s.Serve(l); err != nil {
		return err
	}
	return nil
}

func (d *Dispatcher) messageHandler2(_ mqtt.Client, msg mqtt.Message) {
	var w pb.WorkAssignment
	if err := proto.Unmarshal(msg.Payload(), &w); err != nil {
		log.Error(err)
		return
	}

	resp, err := d.httpClient.Get(w.PayloadUrl)
	if err != nil {
		log.Error(err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return
	}

	if d.keyring != nil {
		resp, err := d.httpClient.Get(w.PayloadUrl + "/asc")
		if err != nil {
			log.Error(err)
			return
		}

		signedBytes := bytes.NewReader(body)
		_, err = openpgp.CheckArmoredDetachedSignature(d.keyring, signedBytes, resp.Body)
		if err != nil {
			log.Error(err)
			return
		}
	}

	var work pb.Work
	if err := proto.Unmarshal(body, &work); err != nil {
		log.Error(err)
		return
	}

	workerSocketAddr, prs := d.workers[w.GetType()]
	if !prs {
		log.Errorf("no worker registered for type '%v'", w.GetType())
		return
	}

	conn, err := grpc.Dial("unix:"+workerSocketAddr, grpc.WithInsecure())
	if err != nil {
		log.Error(err)
		return
	}
	defer conn.Close()

	c := pb.NewWorkerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.Start(ctx, &work)
	if err != nil {
		log.Error(err)
		return
	}
	if !r.GetAccepted() {
		log.Errorf("work %v rejected by worker %v", work, workerSocketAddr)
		return
	}

	d.assignments[work.GetId()] = &w
}

func (d *Dispatcher) messageHandler(client mqtt.Client, msg mqtt.Message) {
	var message struct {
		Kind string    `json:"kind"`
		URL  string    `json:"url"`
		Sent time.Time `json:"sent"`
	}

	if err := json.Unmarshal(msg.Payload(), &message); err != nil {
		log.Error(err)
		return
	}

	resp, err := d.httpClient.Get(message.URL)
	if err != nil {
		log.Error(err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return
	}
	defer resp.Body.Close()

	if d.keyring != nil {
		resp, err := d.httpClient.Get(message.URL + "/asc")
		if err != nil {
			log.Error(err)
			return
		}

		signedBytes := bytes.NewReader(body)
		_, err = openpgp.CheckArmoredDetachedSignature(d.keyring, signedBytes, resp.Body)
		if err != nil {
			log.Error(err)
			return
		}
	}

	switch message.Kind {
	case "playbook":
		var job Job
		if err := json.Unmarshal(body, &job); err != nil {
			log.Error(err)
			log.Debug(string(body))
			return
		}
		controller := PlaybookJobController{
			job:    job,
			client: &d.httpClient,
			url:    message.URL,
		}
		if err := controller.Start(); err != nil {
			log.Error(err)
			return
		}
	default:
		log.Errorf("unsupported message: %+v", message)
	}
}

// Register implements the "Register" RPC method of the Manager service.
func (d *Dispatcher) Register(ctx context.Context, r *pb.WorkRegistration) (*pb.RegisterResponse, error) {
	if _, ok := d.workers[r.GetType()]; ok {
		return &pb.RegisterResponse{
			Registered: false,
			Reason:     "already registered",
		}, nil
	}

	socketAddr := fmt.Sprintf("@ygg-%v-%v", r.GetType(), randomString(6))
	d.workers[r.GetType()] = socketAddr
	return &pb.RegisterResponse{
		Registered: true,
		Address:    socketAddr,
	}, nil
}

// Finish implements the "Finish" RPC method of the Manager service.
func (d *Dispatcher) Finish(ctx context.Context, r *pb.Work) (*pb.Empty, error) {
	w, prs := d.assignments[r.GetId()]
	if !prs {
		return nil, fmt.Errorf("missing assignment %v", r.GetId())
	}

	var data bytes.Buffer
	for _, d := range r.Data {
		data.Write(d)
	}

	resp, err := d.httpClient.Post(w.GetReturnUrl(), bytes.NewReader(data.Bytes()))
	if err != nil {
		return nil, err
	}
	log.Debugf("%#v", resp)

	delete(d.assignments, r.GetId())

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
