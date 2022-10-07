package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"git.sr.ht/~spc/go-log"

	"github.com/redhatinsights/yggdrasil"
	"github.com/urfave/cli/v2"
)

var DeveloperBuild = true

func main() {
	app := cli.NewApp()
	app.Name = yggdrasil.ShortName + "ctl"
	app.Version = yggdrasil.Version
	app.Usage = "control and interact with " + yggdrasil.ShortName + "d"

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:   "generate-man-page",
			Hidden: !DeveloperBuild,
		},
		&cli.BoolFlag{
			Name:   "generate-markdown",
			Hidden: !DeveloperBuild,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:   "generate",
			Usage:  `Generate messages for publishing to client "in" topics.`,
			Hidden: !DeveloperBuild,
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
			Name:   "benchmark",
			Hidden: !DeveloperBuild,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "broker",
					Aliases: []string{"b"},
					Usage:   "set broker to `URI`",
				},
				&cli.StringFlag{
					Name:    "topic-in",
					Aliases: []string{"t"},
					Usage:   "set inbound topic to `STRING`",
				},
				&cli.StringFlag{
					Name:    "topic-out",
					Aliases: []string{"T"},
					Usage:   "set outbound topic to `STRING`",
				},
				&cli.IntFlag{
					Name:    "count",
					Aliases: []string{"c"},
					Usage:   "publish `INT` messages",
					Value:   100,
				},
				&cli.DurationFlag{
					Name:    "message-delay",
					Aliases: []string{"m"},
					Usage:   "wait `DURATION` between publishing each message",
				},
			},
			Action: func(c *cli.Context) error {
				quit := make(chan os.Signal, 1)
				signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

				benchmark(c.String("broker"), c.String("topic-in"), c.String("topic-out"), c.Int("count"), c.Duration("message-delay"))

				<-quit

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

		return nil
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
