package main

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"
	"time"

	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

type message struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args"`
}

// execServer implements the Worker gRPC service as defined by the yggdrasil
// gRPC protocol. It accepts Work messages, unmarshals the work data into a
// "message" type (which consists of a command and argument array), executes
// the requested command, and finishes the work by calling "Finish" on the
// Manager gRPC service. Any output produced by the executed command is returned
// during the "Finish" call.
type execServer struct {
	pb.UnimplementedWorkerServer
	socketAddr string
	busy       bool
	id         string
	cmd        string
	args       []string
}

func (s *execServer) Start(ctx context.Context, r *pb.Work) (*pb.StartResponse, error) {
	// Reject the Start request if we're already working.
	if s.busy {
		return &pb.StartResponse{
			Accepted: false,
		}, nil
	}

	s.busy = true
	s.id = r.GetId()

	// Work.
	go func() {
		// Unmarshal the data into a message.
		var m message
		if err := json.Unmarshal(r.GetData(), &m); err != nil {
			log.Fatal(err)
		}

		// Create and execute the command.
		cmd := exec.Command(m.Cmd, m.Args...)
		output, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}

		// Dial the Manager and call "Finish"
		conn, err := grpc.Dial(yggdDispatcherSocketAddr, grpc.WithInsecure())
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		c := pb.NewDispatcherClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		var work pb.Work
		work.Id = s.id
		work.Data = output

		if _, err := c.Finish(ctx, &work); err != nil {
			log.Fatal(err)
		}

		// Free up for more work.
		s.busy = false
		s.id = ""
	}()

	// Respond to the start request that work was accepted.
	return &pb.StartResponse{
		Accepted: true,
	}, nil
}
