package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	var err error
	go func() {
		// Read the payload bytes from the work message.
		var data bytes.Buffer
		for _, d := range r.GetData() {
			if _, localErr := data.Write(d); localErr != nil {
				err = localErr
				return
			}
		}

		// Unmarshal the data into a message.
		var m message
		if localErr := json.Unmarshal(data.Bytes(), &m); localErr != nil {
			err = localErr
			return
		}

		// Create and execute the command.
		cmd := exec.Command(m.Cmd, m.Args...)
		output, localErr := cmd.Output()
		if localErr != nil {
			err = localErr
			return
		}

		// Dial the Manager and call "Finish"
		conn, localErr := grpc.Dial(yggdDispatcherSocketAddr, grpc.WithInsecure())
		if localErr != nil {
			err = localErr
			return
		}
		defer conn.Close()

		c := pb.NewManagerClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		var work pb.Work
		work.Id = s.id
		work.Data = append(work.Data, output)

		if _, localErr := c.Finish(ctx, &work); localErr != nil {
			err = localErr
			return
		}

		// Free up for more work.
		s.busy = false
		s.id = ""
	}()
	if err != nil {
		return nil, err
	}

	// Respond to the start request that work was accepted.
	return &pb.StartResponse{
		Accepted: true,
	}, nil
}
