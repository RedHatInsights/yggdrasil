package main

import (
	"encoding/json"
	"fmt"
	"os"

	internal "github.com/redhatinsights/yggdrasil/internal"
	yggdrasil "github.com/redhatinsights/yggdrasil/pkg"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

func main() {
	app := cli.NewApp()
	app.Version = yggdrasil.Version

	defaultConfigFilePath, err := internal.ConfigPath()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:      "config",
			Value:     defaultConfigFilePath,
			TakesFile: true,
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "base-url",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "auth-mode",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "username",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "password",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "cert-file",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "key-file",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "ca-root",
		}),
	}

	// This BeforeFunc will load flag values from a config file only if the
	// "config" flag value is non-zero.
	app.Before = func(c *cli.Context) error {
		if c.String("config") != "" {
			inputSource, err := altsrc.NewTomlSourceFromFlagFunc("config")(c)
			if err != nil {
				return err
			}
			return altsrc.ApplyInputSourceValues(c, inputSource, app.Flags)
		}
		return nil
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
				client, err := newClient(c.String("base-url"),
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
