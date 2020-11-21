package main

import (
	"context"
	"net"
	"time"

	"git.sr.ht/~spc/go-log"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

const yggdDispatcherSocketAddr = "unix:@yggd-dispatcher"

func main() {
	// Dial the yggd-dispatcher, register handling the "exec" type, and get our
	// socket address.
	var socketAddr string
	{
		conn, err := grpc.Dial(yggdDispatcherSocketAddr)
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		c := pb.NewManagerClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		r, err := c.Register(ctx, &pb.WorkRegistration{Type: "exec"})
		if err != nil {
			log.Fatal(err)
		}
		if !r.GetRegistered() {
			log.Fatalf("not registered: %v", r.GetReason())
		}
		socketAddr = r.GetAddress()
	}

	// Listen on the socket address.
	l, err := net.Listen("unix", socketAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Register a worker server with gRPC and start accepting connections.
	s := grpc.NewServer()
	pb.RegisterWorkerServer(s, &execServer{socketAddr: socketAddr})
	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
