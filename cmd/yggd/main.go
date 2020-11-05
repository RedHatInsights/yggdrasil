package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	"github.com/urfave/cli/v2"
)

func main() {
	app, err := yggdrasil.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	app.Action = func(c *cli.Context) error {
		level, err := log.ParseLevel(c.String("log-level"))
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		log.SetLevel(level)
		log.SetPrefix(fmt.Sprintf("[%v] ", app.Name))

		dispatcher, err := yggdrasil.NewDispatcher()
		if err != nil {
			return err
		}

		if err := dispatcher.Connect(); err != nil {
			return err
		}

		if err := dispatcher.PublishFacts(); err != nil {
			return err
		}

		if err := dispatcher.Subscribe(); err != nil {
			return err
		}

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		<-quit

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
