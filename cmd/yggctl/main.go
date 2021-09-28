package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = yggdrasil.ShortName + "ctl"
	app.Version = yggdrasil.Version
	app.Usage = "control and interact with " + yggdrasil.ShortName + "d"

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
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generateMessage(messageType, responseTo, directive, content string, metadata map[string]string, version int) ([]byte, error) {
	msg := map[string]interface{}{
		"type":        messageType,
		"message_id":  uuid.New().String(),
		"response_id": responseTo,
		"version":     version,
		"sent":        time.Now(),
		"content":     content,
	}

	switch messageType {
	case "data":
		msg["directive"] = directive
		msg["metadata"] = metadata
	case "command":
		break
	default:
		return nil, fmt.Errorf("unsupported message type: %v", messageType)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal message: %w", err)
	}

	return data, nil
}
