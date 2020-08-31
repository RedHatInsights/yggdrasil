package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
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

	app.Commands = []*cli.Command{}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
