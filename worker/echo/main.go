package main

import (
	"context"
	"net"
	"os"
	"time"

	"git.sr.ht/~spc/go-log"

	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

const yggdDispatchSocketAddr = "unix:@yggd-dispatcher"

func main() {
	// Dial the dispatcher on its well-known address.
	conn, err := grpc.Dial(yggdDispatchSocketAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a dispatcher client
	c := pb.NewDispatcherClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Register as a handler of the "echo" type.
	r, err := c.Register(ctx, &pb.RegistrationRequest{Handler: "echo", Pid: int64(os.Getpid())})
	if err != nil {
		log.Fatal(err)
	}
	if !r.GetRegistered() {
		log.Fatalf("handler registration failed: %v", err)
	}

	// Listen on the provided socket address.
	l, err := net.Listen("unix", r.GetAddress())
	if err != nil {
		log.Fatal(err)
	}

	// Register as a Worker service with gRPC and start accepting connections.
	s := grpc.NewServer()
	pb.RegisterWorkerServer(s, &echoServer{})
	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
