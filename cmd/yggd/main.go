package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
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
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
			Name: "broker",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "public-key",
		}),
	}

	// This BeforeFunc will load flag values from a config file only if the
	// "config" flag value is non-zero.
	app.Before = func(c *cli.Context) error {
		filePath := c.String("config")
		if filePath != "" {
			inputSource, err := altsrc.NewTomlSourceFromFile(filePath)
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

		dispatcher, err := yggdrasil.NewDispatcher(c.StringSlice("broker"), data)
		if err != nil {
			return err
		}

		go func() {
			if localErr := dispatcher.ListenAndServe(); localErr != nil {
				err = localErr
				return
			}

			p := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
			i, localErr := ioutil.ReadDir(p)
			if localErr != nil {
				err = localErr
				return
			}

			for _, info := range i {
				if strings.HasSuffix(info.Name(), "worker") {
					cmd := exec.Command(filepath.Join(p, info.Name()))
					if localErr := cmd.Start(); localErr != nil {
						err = localErr
						return
					}
				}
			}
		}()

		go func() {
			if localError := dispatcher.Connect(); localError != nil {
				err = localError
				return
			}

			if localErr := dispatcher.PublishFacts(); err != nil {
				err = localErr
				return
			}

			if localErr := dispatcher.Subscribe(); localErr != nil {
				err = localErr
				return
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		<-quit

		if err != nil {
			return cli.NewExitError(err, 1)
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
