package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"git.sr.ht/~spc/go-log"

	"github.com/redhatinsights/yggdrasil"
	internal "github.com/redhatinsights/yggdrasil/internal"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	app := cli.NewApp()

	log.SetFlags(0)
	log.SetPrefix("")

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
			Name: "connect",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "username",
					Usage: "register with `USERNAME`",
				},
				&cli.StringFlag{
					Name:  "password",
					Usage: "register with `PASSWORD`",
				},
			},
			Action: func(c *cli.Context) error {
				uuid, err := getConsumerUUID()
				if err != nil {
					return err
				}
				if uuid == "" {
					username := c.String("username")
					password := c.String("password")
					if username == "" {
						password = ""
						scanner := bufio.NewScanner(os.Stdin)
						fmt.Print("Username: ")
						scanner.Scan()
						username = strings.TrimSpace(scanner.Text())
					}
					if password == "" {
						fmt.Print("Password: ")
						data, err := terminal.ReadPassword(int(os.Stdin.Fd()))
						if err != nil {
							return err
						}
						password = string(data)
						fmt.Println()
					}

					if err := register(username, password); err != nil {
						log.Error(err)
					}
				}

				if err := activate(); err != nil {
					log.Error(err)
				}

				return nil
			},
		},
		{
			Name: "disconnect",
			Action: func(c *cli.Context) error {
				if err := deactivate(); err != nil {
					log.Error(err)
				}

				if err := unregister(); err != nil {
					log.Error(err)
				}

				return nil
			},
		},
		{
			Name:  "canonical-facts",
			Usage: "prints canonical facts about the system",
			Action: func(c *cli.Context) error {
				facts, err := yggdrasil.GetCanonicalFacts()
				if err != nil {
					return err
				}
				data, err := json.Marshal(facts)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
				return nil
			},
		},
		{
			Name:  "facts",
			Usage: "prints information about the system like architecture",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "format",
					Value: "table",
				},
			},
			Action: func(c *cli.Context) error {
				facts, err := yggdrasil.GetFacts()
				if err != nil {
					return err
				}
				switch c.String("format") {
				case "json":
					data, err := json.Marshal(facts)
					if err != nil {
						return err
					}
					fmt.Print(string(data))
				case "table":
					keys := make([]string, 0, len(facts))
					for k := range facts {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					for _, k := range keys {
						fmt.Fprintf(w, "%v\t%v\n", k, facts[k])
					}
					w.Flush()
				default:
					return fmt.Errorf("unsupported value for '--format': %v", c.String("format"))
				}
				return nil
			},
		},
		{
			Name:  "status",
			Usage: "reports connection status",
			Action: func(c *cli.Context) error {
				s, err := getStatus()
				if err != nil {
					return cli.NewExitError(err, 1)
				}
				fmt.Println(s)

				return nil
			},
		},
	}
	app.EnableBashCompletion = true
	app.BashComplete = internal.BashComplete
	app.Action = func(c *cli.Context) error {
		type GenerationFunc func() (string, error)
		var generationFunc GenerationFunc
		if c.Bool("generate-man-page") {
			generationFunc = c.App.ToMan
		} else if c.Bool("generate-markdown") {
			generationFunc = c.App.ToMarkdown
		} else {
			cli.ShowAppHelpAndExit(c, 0)
		}
		data, err := generationFunc()
		if err != nil {
			return err
		}
		fmt.Println(data)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
	}
}
