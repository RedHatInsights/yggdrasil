package main

import (
	"fmt"
	"io/ioutil"
	"os"
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

		in := make(chan yggdrasil.Assignment)
		out := make(chan yggdrasil.Assignment)
		died := make(chan int64)

		// ProcessManager goroutine
		go func() {
			m := yggdrasil.NewProcessManager(died)

			logger := log.New(os.Stderr, fmt.Sprintf("%v[manager_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			p := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
			fileInfos, localErr := ioutil.ReadDir(p)
			if localErr != nil {
				err = localErr
				return
			}

			for _, info := range fileInfos {
				if strings.HasSuffix(info.Name(), "worker") {
					logger.Tracef("found worker: %v", info.Name())
					_, localErr := m.StartWorker(filepath.Join(p, info.Name()))
					if localErr != nil {
						logger.Tracef("worker failed to start: %v", localErr)
						err = localErr
						return
					}
				}
			}
			go m.ReapWorkers()
		}()

		// Dispatcher goroutine
		go func() {
			d := yggdrasil.NewDispatcher(in, out, died)

			logger := log.New(os.Stderr, fmt.Sprintf("%v[dispatcher_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			if localErr := d.ListenAndServe(); localErr != nil {
				logger.Trace(localErr)
				err = localErr
				return
			}
		}()

		// SignalRouter goroutine
		go func() {
			r, localErr := yggdrasil.NewSignalRouter(c.StringSlice("broker"), data, out, in)
			if localErr != nil {
				err = localErr
				return
			}

			logger := log.New(os.Stderr, fmt.Sprintf("%v[mqtt_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			if localError := r.Connect(); localError != nil {
				err = localError
				return
			}

			facts, localErr := yggdrasil.GetCanonicalFacts()
			if localErr != nil {
				err = localErr
				return
			}
			if localErr := r.Publish(facts); err != nil {
				err = localErr
				return
			}

			if localErr := r.Subscribe(); localErr != nil {
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

// TODO
// Manager, manages the lifecycle of worker processes. Spawns them. Monitors for
// when they crash. Tells Dispatcher when they crash (or removes them from a
// WorkerProcessRegister)

// Dispatcher receives messages from MQTT broker and routes them to workers,
// if they are work signals, or handles them if they are error or control signals.
