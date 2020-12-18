package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	internal "github.com/redhatinsights/yggdrasil/internal"
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
		&cli.BoolFlag{
			Name:   "generate-man-page",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "generate-markdown",
			Hidden: true,
		},
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
		if c.Bool("generate-man-page") || c.Bool("generate-markdown") {
			type GenerationFunc func() (string, error)
			var generationFunc GenerationFunc
			if c.Bool("generate-man-page") {
				generationFunc = c.App.ToMan
			} else if c.Bool("generate-markdown") {
				generationFunc = c.App.ToMarkdown
			}
			data, err := generationFunc()
			if err != nil {
				return err
			}
			fmt.Println(data)
			return nil
		}
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

		in := make(chan *yggdrasil.Assignment)
		out := make(chan *yggdrasil.Assignment)
		died := make(chan int64)

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

		// ProcessManager goroutine
		go func() {
			m := yggdrasil.NewProcessManager(died)

			logger := log.New(os.Stderr, fmt.Sprintf("%v[manager_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			p := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
			os.MkdirAll(p, 0755)
			fileInfos, localErr := ioutil.ReadDir(p)
			if localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}

			for _, info := range fileInfos {
				if strings.HasSuffix(info.Name(), "worker") {
					logger.Tracef("found worker: %v", info.Name())
					_, localErr := m.StartWorker(filepath.Join(p, info.Name()))
					if localErr != nil {
						logger.Tracef("worker failed to start: %v", localErr)
						err = localErr
						quit <- syscall.SIGTERM
					}
				}
			}
			go m.ReapWorkers()
		}()

		// Dispatcher goroutine
		go func() {
			d, localErr := yggdrasil.NewDispatcher(in, out, died)
			if localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}

			logger := log.New(os.Stderr, fmt.Sprintf("%v[dispatcher_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			if localErr := d.ListenAndServe(); localErr != nil {
				logger.Trace(localErr)
				err = localErr
				quit <- syscall.SIGTERM
			}
		}()

		// SignalRouter goroutine
		go func() {
			r, localErr := yggdrasil.NewSignalRouter(c.StringSlice("broker"), data, out, in)
			if localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}

			logger := log.New(os.Stderr, fmt.Sprintf("%v[mqtt_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			if localError := r.Connect(); localError != nil {
				err = localError
				quit <- syscall.SIGTERM
			}

			facts, localErr := yggdrasil.GetCanonicalFacts()
			if localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}
			data, localErr := json.Marshal(facts)
			if localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}
			if localErr := r.Publish(data); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}

			if localErr := r.Subscribe(); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}
		}()

		<-quit

		if err != nil {
			return cli.NewExitError(err, 1)
		}

		return nil
	}
	app.EnableBashCompletion = true
	app.BashComplete = internal.BashComplete

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
