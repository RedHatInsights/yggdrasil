package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redhatinsights/yggdrasil/internal"
	"github.com/redhatinsights/yggdrasil/internal/http"
	"github.com/redhatinsights/yggdrasil/internal/transport"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"github.com/rjeczalik/notify"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"google.golang.org/grpc"
)

var (
	ClientID   = ""
	SocketAddr = ""
	UserAgent  = yggdrasil.LongName + "/" + yggdrasil.Version
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
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  "protocol",
			Usage: "Transmit data remotely using `PROTOCOL` ('mqtt' or 'http')",
			Value: "mqtt",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  "server",
			Usage: "Connect the client to the specified `URI`",
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
		if err := stopWorkers(); err != nil {
			return cli.Exit(fmt.Errorf("cannot stop workers: %w", err), 1)
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
		SocketAddr = c.String("socket-addr")

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
		httpClient := http.NewHTTPClient(tlsConfig, UserAgent)

		// Create gRPC dispatcher service
		d := newDispatcher(httpClient)
		s := grpc.NewServer()
		pb.RegisterDispatcherServer(s, d)

		l, err := net.Listen("unix", SocketAddr)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot listen to socket: %w", err), 1)
		}
		go func() {
			log.Infof("listening on socket: %v", SocketAddr)
			if err := s.Serve(l); err != nil {
				log.Errorf("cannot start server: %v", err)
			}
		}()

		client := Client{
			d: d,
		}

		var transporter transport.Transporter
		switch c.String("protocol") {
		case "mqtt":
			var err error
			transporter, err = transport.NewMQTTTransport(ClientID, c.String("server"), tlsConfig, client.DataReceiveHandlerFunc)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot create MQTT transport: %w", err), 1)
			}
		case "http":
			var err error
			transporter, err = transport.NewHTTPTransport(ClientID, c.String("server"), tlsConfig, UserAgent, time.Second*5, client.DataReceiveHandlerFunc)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot create HTTP transport: %w", err), 1)
			}
		default:
			return cli.Exit(fmt.Errorf("unsupported transport protocol: %v", c.String("protocol")), 1)
		}
		client.t = transporter
		if err := client.Connect(); err != nil {
			return cli.Exit(fmt.Errorf("cannot connect using transport: %w", err), 1)
		}

		go func() {
			msg, err := client.ConnectionStatus()
			if err != nil {
				log.Errorf("cannot get connection status: %v", err)
			}
			if err := client.SendConnectionStatusMessage(msg); err != nil {
				log.Errorf("cannot send connection status message: %v", err)
			}
		}()

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
				go func() {
					msg, err := client.ConnectionStatus()
					if err != nil {
						log.Errorf("cannot get connection status: %v", err)
					}
					if err = client.SendConnectionStatusMessage(msg); err != nil {
						log.Errorf("cannot send connection status: %v", err)
					}
				}()
			}
		}()

		// Start a goroutine that receives yggdrasil.Data values on a 'send'
		// channel and dispatches them to worker processes.
		go d.sendData()

		// Start a goroutine that receives yggdrasil.Data values on a 'recv'
		// channel and publish them to MQTT.
		go client.ReceiveData()

		// Locate and start worker child processes.
		workerPath := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "workers")
		if err := os.MkdirAll(workerPath, 0755); err != nil {
			return cli.Exit(fmt.Errorf("cannot create directory: %w", err), 1)
		}

		fileInfos, err := ioutil.ReadDir(workerPath)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot read contents of directory: %w", err), 1)
		}
		for _, info := range fileInfos {
			log.Debugf("starting worker: %v", info.Name())
			config, err := loadWorkerConfig(filepath.Join(workerPath, info.Name()))
			if err != nil {
				log.Errorf("cannot load worker config: %v", err)
				continue
			}
			go func() {
				if err := startWorker(*config, nil, func(pid int) {
					d.deadWorkers <- pid
				}); err != nil {
					log.Errorf("cannot start worker: %v", err)
					return
				}
			}()
		}
		// Start a goroutine that watches the worker directory for added or
		// deleted files. Any "worker" files it detects are started up.
		go watchWorkerDir(workerPath, d.deadWorkers)

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
					go func() {
						msg, err := client.ConnectionStatus()
						if err != nil {
							log.Errorf("cannot get connection status: %v", err)
						}
						if err = client.SendConnectionStatusMessage(msg); err != nil {
							log.Errorf("cannot send connection status: %v", err)
						}
					}()
				}
			}
		}()

		<-quit

		if err := stopWorkers(); err != nil {
			return cli.Exit(fmt.Errorf("cannot stop workers: %w", err), 1)
		}

		return nil
	}
	app.EnableBashCompletion = true
	app.BashComplete = internal.BashComplete

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
