package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"github.com/subpop/go-log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
		conn, err := grpc.Dial(
			yggdDispatchSocketAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
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
