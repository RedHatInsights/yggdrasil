package main

import (
	"context"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

// echoServer implements the Worker gRPC service as defined by the yggdrasil
// gRPC protocol. It accepts Assignment messages, unmarshals the data into a
// string, and echoes the content back to the Dispatch service by calling the
// "Finish" method.
type echoServer struct {
	pb.UnimplementedWorkerServer
}

// Send implements the "Send" method of the Worker gRPC service.
func (s *echoServer) Send(ctx context.Context, d *pb.Data) (*pb.Receipt, error) {
	go func() {
		log.Tracef("received data: %#v", d)
		message := string(d.GetContent())
		log.Infof("echoing %v", message)

		// Dial the Dispatcher and call "Finish"
		conn, err := grpc.Dial(yggdDispatchSocketAddr, grpc.WithInsecure())
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		// Create a client of the Dispatch service
		c := pb.NewDispatcherClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Create a data message to send back to the dispatcher.
		data := &pb.Data{
			MessageId:  uuid.New().String(),
			ResponseTo: d.GetMessageId(),
			Metadata:   d.GetMetadata(),
			Content:    d.GetContent(),
			Directive:  d.GetDirective(),
		}

		// Call "Send"
		if _, err := c.Send(ctx, data); err != nil {
			log.Error(err)
		}
	}()

	// Respond to the start request that the work was accepted.
	return &pb.Receipt{}, nil
}

// Disconnect implements the "Disconnect" method of the Worker gRPC service.
func (s *echoServer) Disconnect(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.DisconnectResponse, error) {
	log.Infof("received worker disconnect request")
	
	// Respond to the disconnect request that the work was accepted.
	return &pb.DisconnectResponse{}, nil

}
