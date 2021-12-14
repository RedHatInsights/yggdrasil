package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/redhatinsights/yggdrasil/internal"
	"github.com/redhatinsights/yggdrasil/internal/mqtt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"github.com/rjeczalik/notify"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"google.golang.org/grpc"
)

var ClientID = ""

type TransportType string

const (
	MQTT TransportType = "mqtt"
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
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
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
		&cli.StringFlag{
			Name:   "socket-addr",
			Usage:  "Force yggd to listen on `SOCKET`",
			Value:  fmt.Sprintf("@yggd-dispatcher-%v", randomString(6)),
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "transport",
			Usage:  "Force yggdrasil to use specific transport",
			Value:  string(MQTT),
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

		// Set TopicPrefix globally if the config option is non-zero
		if c.String("topic-prefix") != "" {
			yggdrasil.TopicPrefix = c.String("topic-prefix")
		}

		// Set DataHost globally if the config option is non-zero
		if c.String("data-host") != "" {
			yggdrasil.DataHost = c.String("data-host")
		}

		// Set up a channel to receive the TERM or INT signal over and clean up
		// before quitting.
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

		// Set up logging
		level, err := log.ParseLevel(c.String("log-level"))
		if err != nil {
			return cli.Exit(err, 1)
		}
		log.SetLevel(level)
		log.SetPrefix(fmt.Sprintf("[%v] ", app.Name))
		if log.CurrentLevel() >= log.LevelDebug {
			log.SetFlags(log.LstdFlags | log.Llongfile)
		}

		log.Infof("starting %v version %v", app.Name, app.Version)

		log.Trace("attempting to kill any orphaned workers")
		if err := killWorkers(); err != nil {
			return cli.Exit(fmt.Errorf("cannot kill workers: %w", err), 1)
		}

		clientIDFile := filepath.Join(yggdrasil.LocalstateDir, yggdrasil.LongName, "client-id")
		if c.String("cert-file") != "" {
			CN, err := parseCertCN(c.String("cert-file"))
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot parse certificate: %w", err), 1)
			}
			if err := setClientID([]byte(CN), clientIDFile); err != nil {
				return cli.Exit(fmt.Errorf("cannot set client-id to CN: %w", err), 1)
			}
		}

		clientID, err := getClientID(clientIDFile)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot get client-id: %w", err), 1)
		}
		if len(clientID) == 0 {
			data, err := createClientID(clientIDFile)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot create client-id: %w", err), 1)
			}
			clientID = data
		}

		ClientID = string(clientID)

		// Read certificates, create a TLS config, and initialize HTTP client
		var certData, keyData []byte
		if c.String("cert-file") != "" && c.String("key-file") != "" {
			var err error
			certData, err = ioutil.ReadFile(c.String("cert-file"))
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot read certificate file: %v", err), 1)
			}
			keyData, err = ioutil.ReadFile(c.String("key-file"))
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot read key file: %w", err), 1)
			}
		}
		rootCAs := make([][]byte, 0)
		for _, file := range c.StringSlice("ca-root") {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot read certificate authority: %v", err), 1)
			}
			rootCAs = append(rootCAs, data)
		}
		tlsConfig, err := newTLSConfig(certData, keyData, rootCAs)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot create TLS config: %w", err), 1)
		}
		initHTTPClient(tlsConfig, fmt.Sprintf("%v/%v", app.Name, app.Version))

		// Create gRPC dispatcher service
		d := newDispatcher()
		s := grpc.NewServer()
		pb.RegisterDispatcherServer(s, d)

		l, err := net.Listen("unix", c.String("socket-addr"))
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot listen to socket: %w", err), 1)
		}
		go func() {
			log.Infof("listening on socket: %v", c.String("socket-addr"))
			if err := s.Serve(l); err != nil {
				log.Errorf("cannot start server: %v", err)
			}
		}()

		controlPlaneTransport, err := createTransport(c, tlsConfig, d)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		err = controlPlaneTransport.Start()
		if err != nil {
			return cli.Exit(err, 1)
		}

		// Start a goroutine that receives values on the 'dispatchers' channel
		// and publishes "connection-status" messages to MQTT.
		var prevDispatchersHash atomic.Value
		go func() {
			for dispatchers := range d.dispatchers {
				data, err := json.Marshal(dispatchers)
				if err != nil {
					log.Errorf("cannot marshal dispatcher map to JSON: %v", err)
					continue
				}

				// Create a checksum of the dispatchers map. If it's identical
				// to the previous checksum, skip publishing a connection-status
				// message.
				sum := fmt.Sprintf("%x", sha256.Sum256(data))
				oldSum := prevDispatchersHash.Load()
				if oldSum != nil {
					if sum == oldSum.(string) {
						continue
					}
				}
				prevDispatchersHash.Store(sum)
				go internal.PublishConnectionStatus(controlPlaneTransport, d.makeDispatchersMap())
			}
		}()

		// Start a goroutine that receives yggdrasil.Data values on a 'send'
		// channel and dispatches them to worker processes.
		go d.sendData()

		// Start a goroutine that receives yggdrasil.Data values on a 'recv'
		// channel and publish them to MQTT.
		go internal.PublishReceivedData(controlPlaneTransport, d.recvQ)

		// Locate and start worker child processes.
		workerPath := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
		if err := os.MkdirAll(workerPath, 0755); err != nil {
			return cli.Exit(fmt.Errorf("cannot create directory: %w", err), 1)
		}

		fileInfos, err := ioutil.ReadDir(workerPath)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot read contents of directory: %w", err), 1)
		}
		configDir := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName)
		env := []string{
			"YGG_SOCKET_ADDR=unix:" + c.String("socket-addr"),
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"YGG_CONFIG_DIR=" + configDir,
			"YGG_LOG_LEVEL=" + level.String(),
			"YGG_CLIENT_ID=" + ClientID,
		}
		for _, info := range fileInfos {
			if strings.HasSuffix(info.Name(), "worker") {
				log.Debugf("starting worker: %v", info.Name())
				go startProcess(filepath.Join(workerPath, info.Name()), env, 0, d.deadWorkers)
			}
		}
		// Start a goroutine that watches the worker directory for added or
		// deleted files. Any "worker" files it detects are started up.
		go watchWorkerDir(workerPath, env, d.deadWorkers)

		// Start a goroutine that receives handler values on a channel and
		// removes the worker registration entry.
		go d.unregisterWorker()

		// Start a goroutine that watches the tags file for write events and
		// publishes connection status messages when the file changes.
		go func() {
			c := make(chan notify.EventInfo, 1)

			fp := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "tags.toml")

			if err := notify.Watch(fp, c, notify.InCloseWrite, notify.InDelete); err != nil {
				log.Infof("cannot start watching '%v': %v", fp, err)
				return
			}
			defer notify.Stop(c)

			for e := range c {
				log.Debugf("received inotify event %v", e.Event())
				switch e.Event() {
				case notify.InCloseWrite, notify.InDelete:
					go internal.PublishConnectionStatus(controlPlaneTransport, d.makeDispatchersMap())
				}
			}
		}()

		<-quit

		if err := killWorkers(); err != nil {
			return cli.Exit(fmt.Errorf("cannot kill workers: %w", err), 1)
		}

		return nil
	}
	app.EnableBashCompletion = true
	app.BashComplete = internal.BashComplete

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func createTransport(c *cli.Context, tlsConfig *tls.Config, d *dispatcher) (internal.Transport, error) {
	dataHandler := createDataHandler(d)
	controlMessageHandler := handleControlMessage

	transportType := TransportType(c.String("transport"))
	switch transportType {
	case MQTT:
		brokers := c.StringSlice("broker")
		return mqtt.NewMQTTTransport(ClientID, brokers, tlsConfig, controlMessageHandler, dataHandler)
	default:
		return nil, fmt.Errorf("unrecognized transport type: %v", transportType)
	}
}

func handleControlMessage(cmd yggdrasil.Command, t internal.Transport) {
	log.Tracef("Control message: %v", cmd)
	switch cmd.Content.Command {
	case yggdrasil.CommandNamePing:
		event := yggdrasil.Event{
			Type:       yggdrasil.MessageTypeEvent,
			MessageID:  uuid.New().String(),
			ResponseTo: cmd.MessageID,
			Version:    1,
			Sent:       time.Now(),
			Content:    string(yggdrasil.EventNamePong),
		}

		err := t.SendControl(event)
		if err != nil {
			log.Error(err)
		}
	case yggdrasil.CommandNameDisconnect:
		log.Info("disconnecting...")
		t.Disconnect(500)
	case yggdrasil.CommandNameReconnect:
		log.Info("reconnecting...")
		t.Disconnect(500)
		delay, err := strconv.ParseInt(cmd.Content.Arguments["delay"], 10, 64)
		if err != nil {
			log.Errorf("cannot parse data to int: %v", err)
			return
		}
		time.Sleep(time.Duration(delay) * time.Second)

		if err := t.Start(); err != nil {
			log.Errorf("cannot reconnect to broker: %v", err)
			return
		}
	default:
		log.Warnf("unknown command: %v", cmd.Content.Command)
	}
}

func createDataHandler(d *dispatcher) func(data yggdrasil.Data) {
	return func(data yggdrasil.Data) {
		d.sendQ <- data
	}
}
