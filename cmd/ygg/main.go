package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/redhatinsights/yggdrasil"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	app, err := yggdrasil.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	app.Commands = []*cli.Command{
		{
			Name: "register",
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
					return err
				}

				if err := activate(); err != nil {
					return err
				}

				return nil
			},
		},
		{
			Name: "unregister",
			Action: func(c *cli.Context) error {
				if err := deactivate(); err != nil {
					return err
				}

				if err := unregister(); err != nil {
					return err
				}

				return nil
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
