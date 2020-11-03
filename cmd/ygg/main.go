package main

import (
	"encoding/json"
	"fmt"
	"os"

	yggdrasil "github.com/redhatinsights/yggdrasil"
	internal "github.com/redhatinsights/yggdrasil/internal"
	"github.com/urfave/cli/v2"
)

func main() {
	app, err := internal.NewApp("ygg-exec")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	app.Commands = []*cli.Command{
		{
			Name: "upload",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name: "collector",
				},
				&cli.StringFlag{
					Name: "metadata",
				},
			},
			Action: func(c *cli.Context) error {
				client, err := internal.NewClient(app.Name,
					c.String("base-url"),
					c.String("auth-mode"),
					c.String("username"),
					c.String("password"),
					c.String("cert-file"),
					c.String("key-file"),
					c.String("ca-root"))
				if err != nil {
					return err
				}

				_, err = upload(client, c.Args().First(), c.String("collector"), c.String("metadata"))
				if err != nil {
					switch err {
					case yggdrasil.ErrInvalidContentType:
						return fmt.Errorf("invalid collector: %v", c.String("collector"))
					case yggdrasil.ErrPayloadTooLarge:
						return fmt.Errorf("archive too large: %v", c.Args().First())
					case yggdrasil.ErrUnauthorized:
						switch c.String("auth-mode") {
						case "basic":
							return fmt.Errorf("authentication failed: username/password incorrect")
						case "cert":
							return fmt.Errorf("authentication failed: certificate incorrect")
						default:
							return fmt.Errorf("authentication failed: %w", err)
						}
					default:
						return err
					}
				}
				return nil
			},
		},
		{
			Name:   "canonical-facts",
			Hidden: true,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "format",
					Value: "",
				},
			},
			Action: func(c *cli.Context) error {
				facts, err := yggdrasil.GetCanonicalFacts()
				if err != nil {
					return err
				}

				switch c.String("format") {
				default:
					data, err := json.Marshal(facts)
					if err != nil {
						return err
					}
					fmt.Println(string(data))
				}

				return nil
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
