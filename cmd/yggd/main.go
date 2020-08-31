package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli"
)

var (
	prefixdir     string
	bindir        string
	sbindir       string
	libexecdir    string
	datadir       string
	datarootdir   string
	mandir        string
	docdir        string
	sysconfdir    string
	localstatedir string
)

func main() {
	app := cli.NewApp()

	app.Flags = []cli.Flag{}

	app.Action = func(c *cli.Context) error {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		<-quit

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
