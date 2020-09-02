package main

import (
	"fmt"
	"os"

	yggdrasil "github.com/redhatinsights/yggdrasil/pkg"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Version = yggdrasil.Version

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name: "config",
		},
		&cli.StringFlag{
			Name: "base-url",
		},
		&cli.StringFlag{
			Name: "auth-mode",
		},
		&cli.StringFlag{
			Name: "username",
		},
		&cli.StringFlag{
			Name: "password",
		},
		&cli.StringFlag{
			Name: "cert-file",
		},
		&cli.StringFlag{
			Name: "key-file",
		},
		&cli.StringFlag{
			Name: "ca-root",
		},
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
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
