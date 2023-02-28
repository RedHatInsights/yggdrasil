package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"git.sr.ht/~spc/go-log"

	"github.com/godbus/dbus/v5"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/constants"
	"github.com/redhatinsights/yggdrasil/ipc"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "yggctl"
	app.Version = constants.Version
	app.Usage = "control and interact with yggd"

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:   "generate-man-page",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "generate-markdown",
			Hidden: true,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:   "generate",
			Usage:  `Generate messages for publishing to client "in" topics.`,
			Hidden: true,
			Subcommands: []*cli.Command{
				{
					Name:    "data-message",
					Usage:   "Generate a data message.",
					Aliases: []string{"data"},
					Flags: []cli.Flag{
						&cli.IntFlag{
							Name:    "version",
							Aliases: []string{"v"},
							Value:   1,
							Usage:   "set version to `NUM`",
						},
						&cli.StringFlag{
							Name:    "response-to",
							Aliases: []string{"r"},
							Usage:   "reply to message `UUID`",
						},
						&cli.StringFlag{
							Name:    "metadata",
							Aliases: []string{"m"},
							Value:   "{}",
							Usage:   "set metadata to `JSON`",
						},
						&cli.StringFlag{
							Name:     "directive",
							Aliases:  []string{"d"},
							Required: true,
							Usage:    "set directive to `STRING`",
						},
					},
					Action: func(c *cli.Context) error {
						var metadata map[string]string
						if err := json.Unmarshal([]byte(c.String("metadata")), &metadata); err != nil {
							return cli.Exit(fmt.Errorf("cannot unmarshal metadata: %w", err), 1)
						}

						data, err := generateMessage("data", c.String("response-to"), c.String("directive"), c.Args().First(), metadata, c.Int("version"))
						if err != nil {
							return cli.Exit(fmt.Errorf("cannot marshal message: %w", err), 1)
						}

						fmt.Println(string(data))

						return nil
					},
				},
				{
					Name:    "control-message",
					Usage:   "Generate a control message.",
					Aliases: []string{"control"},
					Flags: []cli.Flag{
						&cli.IntFlag{
							Name:    "version",
							Aliases: []string{"v"},
							Value:   1,
							Usage:   "set version to `NUM`",
						},
						&cli.StringFlag{
							Name:    "response-to",
							Aliases: []string{"r"},
							Usage:   "reply to message `UUID`",
						},
						&cli.StringFlag{
							Name:     "type",
							Aliases:  []string{"t"},
							Required: true,
							Usage:    "set message type to `STRING`",
						},
					},
					Action: func(c *cli.Context) error {
						data, err := generateMessage(c.String("type"), c.String("response-to"), "", c.Args().First(), nil, c.Int("version"))
						if err != nil {
							return cli.Exit(fmt.Errorf("cannot marshal message: %w", err), 1)
						}

						fmt.Println(string(data))

						return nil
					},
				},
			},
		},
		{
			Name:  "workers",
			Usage: "Interact with yggdrasil workers",
			Subcommands: []*cli.Command{
				{
					Name:        "list",
					Usage:       "List currently connected workers",
					Description: `The list command prints a list of currently connected workers, along with the workers "features" table.`,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "format",
							Usage: "Print output in `FORMAT` (json, table or text)",
							Value: "text",
						},
					},
					Action: func(c *cli.Context) error {
						var conn *dbus.Conn
						var err error

						if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
							conn, err = dbus.ConnectSessionBus()
						} else {
							conn, err = dbus.ConnectSystemBus()
						}
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
					},
				},
			},
		},
		{
			Name:        "dispatch",
			Usage:       "Dispatch data to a worker locally",
			UsageText:   "yggctl dispatch [command options] FILE",
			Description: "The dispatch command reads FILE and sends its content to a yggdrasil worker running locally. If FILE is -, content is read from stdin.",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "worker",
					Aliases:  []string{"w"},
					Usage:    "Send data to `WORKER`",
					Required: true,
				},
				&cli.StringFlag{
					Name:    "metadata",
					Aliases: []string{"m"},
					Usage:   "Attach `JSON` as metadata to the message",
					Value:   "{}",
				},
			},
			Action: func(c *cli.Context) error {
				var conn *dbus.Conn
				var err error

				if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
					conn, err = dbus.ConnectSessionBus()
				} else {
					conn, err = dbus.ConnectSystemBus()
				}
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
			},
		},
		{
			Name:        "listen",
			Usage:       "Listen to worker event output",
			Description: "The listen command waits for events emitted by the specified worker and prints them to the console.",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "worker",
					Aliases:  []string{"w"},
					Usage:    "Listen for events emitted by `WORKER`",
					Required: true,
				},
			},
			Action: func(ctx *cli.Context) error {
				var conn *dbus.Conn
				var err error

				if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
					conn, err = dbus.ConnectSessionBus()
				} else {
					conn, err = dbus.ConnectSystemBus()
				}
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
							return cli.Exit(fmt.Errorf("cannot cat %T as uint32", s.Body[1]), 1)
						}
						var message string
						if len(s.Body) > 2 {
							message, ok = s.Body[2].(string)
							if !ok {
								return cli.Exit(fmt.Errorf("cannot cast %T as string", s.Body[0]), 1)
							}
						}
						log.Printf("%v: %v: %v", worker, ipc.WorkerEventName(name), message)

					}
				}
				return nil
			},
		},
	}

	app.Action = func(c *cli.Context) error {
		if c.Bool("generate-man-page") || c.Bool("generate-markdown") {
			type GenerationFunc func() (string, error)
			var generationFunc GenerationFunc
			if c.Bool("generate-man-page") {
				generationFunc = c.App.ToMan
			} else if c.Bool("generate-markdown") {
				generationFunc = c.App.ToMarkdown
			}
			data, err := generationFunc()
			if err != nil {
				return err
			}
			fmt.Println(data)
			return nil
		}

		return cli.ShowAppHelp(c)
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generateMessage(messageType, responseTo, directive, content string, metadata map[string]string, version int) ([]byte, error) {
	switch messageType {
	case "data":
		msg, err := generateDataMessage(yggdrasil.MessageType(messageType), responseTo, directive, []byte(content), metadata, version)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return data, nil
	case "command":
		msg, err := generateCommandMessage(yggdrasil.MessageType(messageType), responseTo, version, []byte(content))
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
