package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"text/tabwriter"

	"github.com/godbus/dbus/v5"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/ipc"
	"github.com/urfave/cli/v2"
)

func generateDataMessageAction(ctx *cli.Context) error {
	var metadata map[string]string
	if err := json.Unmarshal([]byte(ctx.String("metadata")), &metadata); err != nil {
		return cli.Exit(fmt.Errorf("cannot unmarshal metadata: %w", err), 1)
	}

	var err error
	var content []byte
	var reader io.Reader
	if ctx.Args().First() == "-" {
		reader = os.Stdin
	} else {
		reader, err = os.Open(ctx.Args().First())
	}
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot open file for reading: %w", err), 1)
	}
	content, err = io.ReadAll(reader)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot read data: %w", err), 1)
	}

	data, err := generateMessage(
		"data",
		ctx.String("response-to"),
		ctx.String("directive"),
		content,
		metadata,
		ctx.Int("version"),
	)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot marshal message: %w", err), 1)
	}

	fmt.Println(string(data))

	return nil
}

func generateControlMessageAction(ctx *cli.Context) error {
	var err error
	var content []byte
	var reader io.Reader
	if ctx.Args().First() == "-" {
		reader = os.Stdin
	} else {
		reader, err = os.Open(ctx.Args().First())
	}
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot open file for reading: %w", err), 1)
	}
	content, err = io.ReadAll(reader)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot read data: %w", err), 1)
	}
	data, err := generateMessage(
		ctx.String("type"),
		ctx.String("response-to"),
		"",
		content,
		nil,
		ctx.Int("version"),
	)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot marshal message: %w", err), 1)
	}

	fmt.Println(string(data))

	return nil
}

func workersAction(c *cli.Context) error {
	conn, err := connectBus()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot connect to bus: %w", err), 1)
	}

	obj := conn.Object("com.redhat.Yggdrasil1", "/com/redhat/Yggdrasil1")
	var workers map[string]map[string]string
	if err := obj.Call("com.redhat.Yggdrasil1.ListWorkers", dbus.Flags(0)).Store(&workers); err != nil {
		return cli.Exit(fmt.Errorf("cannot list workers: %v", err), 1)
	}

	switch c.String("format") {
	case "json":
		data, err := json.Marshal(workers)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot marshal workers: %v", err), 1)
		}
		fmt.Println(string(data))
	case "table":
		writer := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
		fmt.Fprintf(writer, "WORKER\tFIELD\tVALUE\n")
		for worker, features := range workers {
			for field, value := range features {
				fmt.Fprintf(writer, "%v\t%v\t%v\n", worker, field, value)
			}
			_ = writer.Flush()
		}
	case "text":
		for worker, features := range workers {
			for field, value := range features {
				fmt.Printf("%v - %v: %v\n", worker, field, value)
			}
		}
	default:
		return cli.Exit(fmt.Errorf("unknown format type: %v", c.String("format")), 1)
	}

	return nil
}

func dispatchAction(c *cli.Context) error {
	conn, err := connectBus()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot connect to bus: %w", err), 1)
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(c.String("metadata")), &metadata); err != nil {
		return cli.Exit(fmt.Errorf("cannot unmarshal metadata: %w", err), 1)
	}

	var data []byte
	var r io.Reader
	if c.Args().First() == "-" {
		r = os.Stdin
	} else {
		r, err = os.Open(c.Args().First())
	}
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot open file for reading: %w", err), 1)
	}
	data, err = io.ReadAll(r)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot read data: %w", err), 1)
	}

	id := uuid.New().String()

	obj := conn.Object("com.redhat.Yggdrasil1", "/com/redhat/Yggdrasil1")
	if err := obj.Call("com.redhat.Yggdrasil1.Dispatch", dbus.Flags(0), c.String("worker"), id, metadata, data).Store(); err != nil {
		return cli.Exit(fmt.Errorf("cannot dispatch message: %w", err), 1)
	}

	fmt.Printf("Dispatched message %v to worker %v\n", id, c.String("worker"))

	return nil
}

func listenAction(ctx *cli.Context) error {
	conn, err := connectBus()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot connect to bus: %w", err), 1)
	}

	if err := conn.AddMatchSignal(); err != nil {
		return cli.Exit(fmt.Errorf("cannot add match signal: %w", err), 1)
	}

	signals := make(chan *dbus.Signal)
	conn.Signal(signals)
	for s := range signals {
		switch s.Name {
		case "com.redhat.Yggdrasil1.WorkerEvent":
			worker, ok := s.Body[0].(string)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as string", s.Body[0]), 1)
			}
			name, ok := s.Body[1].(uint32)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as uint32", s.Body[1]), 1)
			}
			messageID, ok := s.Body[2].(string)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as string", s.Body[2]), 1)
			}
			responseTo, ok := s.Body[3].(string)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as string", s.Body[3]), 1)
			}
			data := map[string]string{}
			if len(s.Body) > 4 {
				data, ok = s.Body[4].(map[string]string)
				if !ok {
					return cli.Exit(fmt.Errorf("cannot cast %T as map[string]string", s.Body[4]), 1)
				}
			}
			parsedData, err := json.Marshal(data)
			if err != nil {
				return cli.Exit(fmt.Errorf("unable to parse optional data: %v", data), 1)
			}

			log.Printf(
				"%v: %v: %v: %v: %v",
				worker,
				messageID,
				ipc.WorkerEventName(name),
				responseTo,
				string(parsedData),
			)
		}
	}
	return nil
}

func generateMessage(
	messageType string,
	responseTo string,
	directive string,
	content []byte,
	metadata map[string]string,
	version int,
) ([]byte, error) {
	switch messageType {
	case "data":
		msg, err := generateDataMessage(
			yggdrasil.MessageType(messageType),
			responseTo,
			directive,
			content,
			metadata,
			version,
		)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return data, nil
	case "command":
		msg, err := generateControlMessage(
			yggdrasil.MessageType(messageType),
			responseTo,
			version,
			content,
		)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %v", messageType)
	}
}

func connectBus() (*dbus.Conn, error) {
	var connect func(...dbus.ConnOption) (*dbus.Conn, error)
	var conn *dbus.Conn
	var err error
	var errMsg string
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" && os.Geteuid() > 0 {
		connect = dbus.ConnectSessionBus
		errMsg = "cannot connect to session bus (" + os.Getenv("DBUS_SESSION_BUS_ADDRESS") + "): %w"
	} else {
		connect = dbus.ConnectSystemBus
		errMsg = "cannot connect to system bus: %w"
	}

	conn, err = connect()
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}
	return conn, nil
}
