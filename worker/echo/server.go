package main

import (
	"context"
	"time"

	"git.sr.ht/~spc/go-log"
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

func (s *echoServer) Start(ctx context.Context, a *pb.Assignment) (*pb.StartResponse, error) {
	go func() {
		log.Tracef("starting assignment: %#v", a)
		message := string(a.GetData())
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

		// Create a complete assignment to send back to the dispatcher.
		completeAssignment := &pb.Assignment{
			Id:       a.GetId(),
			Data:     a.GetData(),
			Complete: !a.GetComplete(),
		}

		// Call "Finish"
		if _, err := c.Finish(ctx, completeAssignment); err != nil {
			log.Fatal(err)
		}
	}()

	// Respond to the start request that the work was accepted.
	return &pb.StartResponse{
		Accepted: true,
	}, nil
}
