package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~spc/go-log"

	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	app := cli.NewApp()

	log.SetFlags(0)
	log.SetPrefix("")

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
					log.Error(err)
				}

				if err := activate(); err != nil {
					log.Error(err)
				}

				return nil
			},
		},
		{
			Name: "unregister",
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
			Name:  "facts",
			Usage: "prints canonical facts about the system",
			Action: func(c *cli.Context) error {
				return fmt.Errorf("not implemented")
			},
		},
		{
			Name:  "status",
			Usage: "reports status of connection and activation",
			Action: func(c *cli.Context) error {
				return fmt.Errorf("not implemented")
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
	}
}
