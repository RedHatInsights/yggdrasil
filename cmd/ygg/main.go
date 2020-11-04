package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
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

				// TODO: activate yggd

				return nil
			},
		},
		{
			Name: "unregister",
			Action: func(c *cli.Context) error {
				conn, err := dbus.SystemBus()
				if err != nil {
					return err
				}
				defer conn.Close()

				object := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
				var jobPath dbus.ObjectPath
				if err := object.Call("org.freedesktop.systemd1.Manager.StopUnit", dbus.Flags(0), "rhcd.service", "replace").Store(&jobPath); err != nil {
					return err
				}
				if jobPath.IsValid() {
					state, err := conn.Object("org.freedesktop.systedm1", jobPath).GetProperty("State")
					if err != nil {
						return err
					}
					fmt.Println(state)
				}

				var changes interface{}
				if err := object.Call("org.freedesktop.systemd1.Manager.DisableUnitFiles", dbus.Flags(0), []string{"rhcd.service"}, false).Store(&changes); err != nil {
					return err
				}

				fmt.Println(changes)

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
