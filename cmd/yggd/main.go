package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

func main() {
	app := cli.NewApp()
	app.Version = yggdrasil.Version

	defaultConfigFilePath, err := yggdrasil.ConfigPath()
	if err != nil {
		log.Fatal(err)
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:      "config",
			Value:     defaultConfigFilePath,
			TakesFile: true,
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  "log-level",
			Value: "info",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:   "broker-addr",
			Hidden: true,
			Value:  yggdrasil.BrokerAddr,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "public-key",
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

	app.Action = func(c *cli.Context) error {
		level, err := log.ParseLevel(c.String("log-level"))
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		log.SetLevel(level)
		log.SetPrefix(fmt.Sprintf("[%v] ", app.Name))

		var data []byte
		if c.String("public-key") != "" {
			data, err = ioutil.ReadFile(c.String("public-key"))
			if err != nil {
				return cli.NewExitError(err, 1)
			}
		}

		dispatcher, err := yggdrasil.NewDispatcher(c.String("broker-addr"), data)
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
