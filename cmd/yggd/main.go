package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "cert-file",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "key-file",
		}),
		&cli.StringFlag{
			Name:   "ca-root",
			Hidden: true,
		},
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
			Name: "broker",
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

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

		db, err := yggdrasil.NewDatastore()
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		dispatcherSocketAddr := fmt.Sprintf("@yggd-dispatcher-%v", randomString(6))

		workerEnv := []string{
			fmt.Sprintf("YGG_SOCKET_ADDR=unix:%v", dispatcherSocketAddr),
		}
		processManager, err := yggdrasil.NewProcessManager(db, workerEnv)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		dispatcher, err := yggdrasil.NewDispatcher(db)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		messageRouter, err := yggdrasil.NewMessageRouter(db, c.StringSlice("broker"), c.String("cert-file"), c.String("key-file"), c.String("ca-root"))
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		dataProcessor, err := yggdrasil.NewDataProcessor(db)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		// Connect dispatcher to the processManager's "process-die" signal
		sigProcessDie := processManager.Connect(yggdrasil.SignalProcessDie)
		go dispatcher.HandleProcessDieSignal(sigProcessDie)

		// Connect dataProcessor to the messageRouter's "data-recv" signal
		sigMessageRecv := messageRouter.Connect(yggdrasil.SignalDataRecv)
		go dataProcessor.HandleDataRecvSignal(sigMessageRecv)

		// Connect dispatcher to the dataProcessor's "data-process" signal
		go dispatcher.HandleDataProcessSignal(dataProcessor.Connect(yggdrasil.SignalDataProcess))

		// Connect dataProcessor to the dispatcher's "data-return" signal
		go dataProcessor.HandleDataReturnSignal(dispatcher.Connect(yggdrasil.SignalDataReturn))

		// Connect messageRouter to the dataProcessor's "data-consume" signal
		go messageRouter.HandleDataConsumeSignal(dataProcessor.Connect(yggdrasil.SignalDataConsume))

		// ProcessManager goroutine
		sigDispatcherListen := dispatcher.Connect(yggdrasil.SignalDispatcherListen)
		go func(c <-chan interface{}) {
			logger := log.New(os.Stderr, fmt.Sprintf("%v[process_manager_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			<-c

			p := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
			os.MkdirAll(p, 0755)
			if localErr := processManager.BootstrapWorkers(p); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}
		}(sigDispatcherListen)

		// Dispatcher goroutine
		go func() {
			logger := log.New(os.Stderr, fmt.Sprintf("%v[dispatcher_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			if localErr := dispatcher.ListenAndServe(dispatcherSocketAddr); localErr != nil {
				logger.Trace(localErr)
				err = localErr
				quit <- syscall.SIGTERM
			}
		}()

		// MessageRouter goroutine
		sigProcessBootstrap := processManager.Connect(yggdrasil.SignalProcessBootstrap)
		go func(c <-chan interface{}) {
			logger := log.New(os.Stderr, fmt.Sprintf("%v[message_router_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			<-c

			if localError := messageRouter.ConnectClient(); localError != nil {
				err = localError
				quit <- syscall.SIGTERM
				return
			}

			if localErr := messageRouter.PublishConnectionStatus(); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
				return
			}

			if localErr := messageRouter.SubscribeAndRoute(); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
				return
			}
		}(sigProcessBootstrap)

		<-quit

		if err := processManager.KillAllWorkers(); err != nil {
			return cli.NewExitError(err, 1)
		}

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
