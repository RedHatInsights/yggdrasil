package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/subpop/go-log"

	"github.com/redhatinsights/yggdrasil/internal/constants"
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
			Usage:  `Generate messages for publishing to client "in" topics`,
			Hidden: true,
			Subcommands: []*cli.Command{
				{
					Name:      "data-message",
					Usage:     "Generate a data message",
					UsageText: "yggctl generate data-message [command options] FILE",
					Description: `The generate data-message command reads FILE and prints a JSON object conforming
to yggdrasil's JSON schema for 'data' messages. If FILE is -, content is read
from stdin.`,
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
					Action: generateDataMessageAction,
				},
				{
					Name:      "control-message",
					Usage:     "Generate a control message",
					UsageText: "yggctl generate control-message [command options] FILE",
					Description: `The generate control-message command reads FILE and prints a JSON object conforming
to yggdrasil's JSON schema for 'control' messages. If FILE is -, content is read
from stdin.`,
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
					Action: generateControlMessageAction,
				},
				{
					Name:      "worker-data",
					Usage:     "Generate data files needed for workers to interact with yggd",
					UsageText: "yggctl generate worker-data [command options]",
					Description: `The generate worker-data command creates data files necessary for workers to
communicate properly with yggd.`,
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:    "install",
							Aliases: []string{"i"},
							Usage:   "install data files into system-appropriate directories",
						},
						&cli.PathFlag{
							Name:    "output",
							Aliases: []string{"o"},
							Usage:   "output files to `DIRECTORY`",
						},
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    "generate files using `NAME`",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "program",
							Aliases:  []string{"p"},
							Usage:    "set the worker program to `PATH`",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "user",
							Aliases:  []string{"u"},
							Usage:    "set the worker user to `USER`",
							Required: true,
						},
						&cli.StringFlag{
							Name:    "group",
							Aliases: []string{"g"},
							Usage:   "set the worker group to `GROUP`",
						},
					},
					Before: func(ctx *cli.Context) error {
						if ctx.String("output") == "" && !ctx.Bool("install") {
							return cli.Exit(
								"error: you must specify either --install or --output",
								1,
							)
						}

						if strings.Contains(ctx.String("name"), " -") {
							return cli.Exit("'name' cannot contain spaces or dashes", 1)
						}

						re := regexp.MustCompile("^[a-z][a-z0-9_]{0,31}$")
						if !re.Match([]byte(ctx.String("user"))) {
							return cli.Exit("'user' must be a valid UNIX identifier", 1)
						}
						if ctx.String("group") != "" {
							if !re.Match([]byte(ctx.String("group"))) {
								return cli.Exit("'group' must be a valid UNIX identifier", 1)
							}
						}

						return nil
					},
					Action: generateWorkerDataAction,
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
							Usage: "Print output in `FORMAT` (json or table)",
							Value: "table",
						},
					},
					Action: workersAction,
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
			Action: dispatchAction,
		},
		{
			Name:        "message-journal",
			Usage:       "Show events emitted by workers",
			UsageText:   "yggctl message-journal",
			Description: "The message-journal command retrieves a list of events emitted by workers",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:     "persistent",
					Aliases:  []string{"p"},
					Usage:    "Include events emitted by workers from persistent storage",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "worker",
					Aliases:  []string{"w"},
					Usage:    "Include only events emitted by `WORKER`",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "message-id",
					Aliases:  []string{"m"},
					Usage:    "Include only events emitted for message ID `STRING`",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "since",
					Aliases:  []string{"s"},
					Usage:    "Include only events emitted after `TIMESTAMP` (YYYY-MM-DD HH:MM:SS)",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "until",
					Aliases:  []string{"u"},
					Usage:    "Include only events emitted before `TIMESTAMP` (YYYY-MM-DD HH:MM:SS)",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "format",
					Aliases:  []string{"f"},
					Usage:    "Print output in `FORMAT` (json, table or text)",
					Value:    "table",
					Required: false,
				},
			},
			Action: messageJournalAction,
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
			Action: listenAction,
		},
	}

	app.Action = generateManPage
	app.EnableBashCompletion = true

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generateManPage(c *cli.Context) error {
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
