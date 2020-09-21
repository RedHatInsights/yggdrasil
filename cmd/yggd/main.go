package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"git.sr.ht/~spc/go-log"
	internal "github.com/redhatinsights/yggdrasil/internal"
	"github.com/urfave/cli/v2"
)

func main() {
	app, err := internal.NewApp("yggd")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	app.Flags = append(app.Flags, &cli.StringFlag{
		Name:      "interface-file",
		Hidden:    true,
		TakesFile: true,
		Value:     filepath.Join(internal.DataDir, "dbus-1", "interfaces", "com.redhat.yggdrasil.xml"),
	})

	app.Action = func(c *cli.Context) error {
		level, err := log.ParseLevel(c.String("log-level"))
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		log.SetLevel(level)

		client, err := internal.NewClient(app.Name,
			c.String("base-url"),
			c.String("auth-mode"),
			c.String("username"),
			c.String("password"),
			c.String("cert-file"),
			c.String("key-file"),
			c.String("ca-root"))
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		server, err := internal.NewDBusServer(client, c.String("interface-file"))
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if err := server.Connect(); err != nil {
			return cli.NewExitError(err, 1)
		}
		defer server.Close()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		<-quit

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
