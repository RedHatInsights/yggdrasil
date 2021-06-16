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
	app.Name = yggdrasil.ShortName + "d"
	app.Version = yggdrasil.Version
	app.Usage = "connect the system to " + yggdrasil.Provider

	defaultConfigFilePath, err := yggdrasil.ConfigPath()
	if err != nil {
		log.Fatal(err)
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:      "config",
			Value:     defaultConfigFilePath,
			TakesFile: true,
			Usage:     "Read config values from `FILE`",
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  "log-level",
			Value: "info",
			Usage: "Set the logging output level to `LEVEL`",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  "cert-file",
			Usage: "Use `FILE` as the client certificate",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  "key-file",
			Usage: "Use `FILE` as the client's private key",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:   "ca-root",
			Hidden: true,
			Usage:  "Use `FILE` as the root CA",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:   "topic-prefix",
			Value:  yggdrasil.TopicPrefix,
			Hidden: true,
			Usage:  "Use `PREFIX` as the MQTT topic prefix",
		}),
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
			Name:  "broker",
			Usage: "Connect to the broker specified in `URI`",
		}),
		&cli.BoolFlag{
			Name:   "generate-man-page",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "generate-markdown",
			Hidden: true,
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  "data-host",
			Usage: "Force all HTTP traffic over `HOST`",
			Value: yggdrasil.DataHost,
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
			if err := altsrc.ApplyInputSourceValues(c, inputSource, app.Flags); err != nil {
				return err
			}
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

		for _, f := range []string{"cert-file", "key-file"} {
			if c.String(f) == "" {
				return cli.Exit(fmt.Errorf("required flag '%v' not set", f), 1)
			}
		}

		if c.String("topic-prefix") != "" {
			yggdrasil.TopicPrefix = c.String("topic-prefix")
		}

		level, err := log.ParseLevel(c.String("log-level"))
		if err != nil {
			return cli.Exit(err, 1)
		}
		log.SetLevel(level)
		log.SetPrefix(fmt.Sprintf("[%v] ", app.Name))

		if level >= log.LevelDebug {
			log.SetFlags(log.LstdFlags | log.Llongfile)
		}

		log.Infof("starting %v version %v", app.Name, app.Version)

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

		db, err := yggdrasil.NewDatastore()
		if err != nil {
			return cli.Exit(err, 1)
		}

		dispatcherSocketAddr := fmt.Sprintf("@yggd-dispatcher-%v", randomString(6))

		workerEnv := []string{
			fmt.Sprintf("YGG_SOCKET_ADDR=unix:%v", dispatcherSocketAddr),
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		}
		processManager, err := yggdrasil.NewProcessManager(db, workerEnv)
		if err != nil {
			return cli.Exit(err, 1)
		}

		dispatcher, err := yggdrasil.NewDispatcher(db)
		if err != nil {
			return cli.Exit(err, 1)
		}

		messageRouter, err := yggdrasil.NewMessageRouter(db, c.StringSlice("broker"), c.String("cert-file"), c.String("key-file"))
		if err != nil {
			return cli.Exit(err, 1)
		}

		dataProcessor, err := yggdrasil.NewDataProcessor(db, c.String("cert-file"), c.String("key-file"), c.String("data-host"), c.String("ca-root"))
		if err != nil {
			return cli.Exit(err, 1)
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
		sigTopicSubscribe := messageRouter.Connect(yggdrasil.SignalTopicSubscribe)
		go func(dispatcherListenSig <-chan interface{}, topicSubscribeSig <-chan interface{}) {
			logger := log.New(os.Stderr, fmt.Sprintf("%v[process_manager_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			<-dispatcherListenSig
			<-topicSubscribeSig

			dispatcher.Disconnect(yggdrasil.SignalDispatcherListen, dispatcherListenSig)
			messageRouter.Disconnect(yggdrasil.SignalTopicSubscribe, topicSubscribeSig)

			if localErr := processManager.KillAllOrphans(); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
				return
			}

			p := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
			if localErr := os.MkdirAll(p, 0755); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}
			if localErr := processManager.BootstrapWorkers(p); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}

			if localErr := processManager.WatchForProcesses(p); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
			}
		}(sigDispatcherListen, sigTopicSubscribe)

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
		go func(c <-chan interface{}) {
			logger := log.New(os.Stderr, fmt.Sprintf("%v[message_router_routine] ", log.Prefix()), log.Flags(), log.CurrentLevel())
			logger.Trace("init")

			if localErr := messageRouter.ConnectClient(); localErr != nil {
				err = localErr
				quit <- syscall.SIGTERM
				return
			}

			<-c
			processManager.Disconnect(yggdrasil.SignalProcessBootstrap, c)

			// Connect messageRouter to the dispatcher's "worker-unregister" signal
			go messageRouter.HandleWorkerUnregisterSignal(dispatcher.Connect(yggdrasil.SignalWorkerUnregister))

			// Connect messageRouter to the dispatcher's "worker-register" signal
			go messageRouter.HandleWorkerRegisterSignal(dispatcher.Connect(yggdrasil.SignalWorkerRegister))
		}(processManager.Connect(yggdrasil.SignalProcessBootstrap))

		<-quit

		if err := processManager.KillAllWorkers(); err != nil {
			return cli.Exit(err, 1)
		}

		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
	app.EnableBashCompletion = true
	app.BashComplete = internal.BashComplete

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
