package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/redhatinsights/yggdrasil/internal/config"
	"github.com/redhatinsights/yggdrasil/internal/http"
	"github.com/redhatinsights/yggdrasil/internal/transport"
	"github.com/redhatinsights/yggdrasil/internal/work"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	"github.com/rjeczalik/notify"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

var (
	UserAgent = yggdrasil.LongName + "/" + yggdrasil.Version
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
			Name:  config.FlagNameLogLevel,
			Value: "info",
			Usage: "Set the logging output level to `LEVEL`",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameCertFile,
			Usage: "Use `FILE` as the client certificate",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameKeyFile,
			Usage: "Use `FILE` as the client's private key",
		}),
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
			Name:   config.FlagNameCaRoot,
			Hidden: true,
			Usage:  "Use `FILE` as the root CA",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:   config.FlagNamePathPrefix,
			Value:  yggdrasil.DefaultPathPrefix,
			Hidden: true,
			Usage:  "Use `PREFIX` as the transport layer path name prefix",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameProtocol,
			Usage: "Transmit data remotely using `PROTOCOL` ('mqtt' or 'http')",
			Value: "mqtt",
		}),
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
			Name:  config.FlagNameServer,
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
			Name:  config.FlagNameDataHost,
			Usage: "Force all HTTP traffic over `HOST`",
			Value: yggdrasil.DefaultDataHost,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameClientID,
			Usage: "Use `VALUE` as the client ID when connecting",
		}),
		altsrc.NewPathFlag(&cli.PathFlag{
			Name:      config.FlagNameCanonicalFacts,
			Usage:     "Read canonical facts from `FILE`",
			TakesFile: true,
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

		config.DefaultConfig = config.Config{
			LogLevel:       c.String(config.FlagNameLogLevel),
			ClientID:       c.String(config.FlagNameClientID),
			Server:         c.StringSlice(config.FlagNameServer),
			CertFile:       c.String(config.FlagNameCertFile),
			KeyFile:        c.String(config.FlagNameKeyFile),
			CARoot:         c.StringSlice(config.FlagNameCaRoot),
			PathPrefix:     c.String(config.FlagNamePathPrefix),
			Protocol:       c.String(config.FlagNameProtocol),
			DataHost:       c.String(config.FlagNameDataHost),
			CanonicalFacts: c.String(config.FlagNameCanonicalFacts),
		}

		tlsConfig, err := config.DefaultConfig.CreateTLSConfig()
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot create TLS config: %w", err), 1)
		}

		TlSEvents, err := config.DefaultConfig.WatcherUpdate()
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot start watching for certificate changes: %w", err), 1)
		}

		// Set up a channel to receive the TERM or INT signal over and clean up
		// before quitting.
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

		// Set up logging
		level, err := log.ParseLevel(config.DefaultConfig.LogLevel)
		if err != nil {
			return cli.Exit(err, 1)
		}
		log.SetLevel(level)
		log.SetPrefix(fmt.Sprintf("[%v] ", app.Name))
		if log.CurrentLevel() >= log.LevelDebug {
			log.SetFlags(log.LstdFlags | log.Llongfile)
		}

		log.Infof("starting %v version %v", app.Name, app.Version)

		clientIDFile := filepath.Join(yggdrasil.LocalstateDir, "lib", yggdrasil.LongName, "client-id")
		if config.DefaultConfig.CertFile != "" {
			CN, err := parseCertCN(config.DefaultConfig.CertFile)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot parse certificate: %w", err), 1)
			}
			if err := setClientID([]byte(CN), clientIDFile); err != nil {
				return cli.Exit(fmt.Errorf("cannot set client-id to CN: %w", err), 1)
			}
		}

		if config.DefaultConfig.ClientID == "" {
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
			config.DefaultConfig.ClientID = string(clientID)
		}

		httpClient := http.NewHTTPClient(tlsConfig, UserAgent)

		// Create Dispatcher service
		dispatcher := work.NewDispatcher(httpClient)

		var transporter transport.Transporter
		switch config.DefaultConfig.Protocol {
		case "mqtt":
			var err error
			transporter, err = transport.NewMQTTTransport(config.DefaultConfig.ClientID, config.DefaultConfig.Server, tlsConfig)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot create MQTT transport: %w", err), 1)
			}
		case "http":
			var err error
			transporter, err = transport.NewHTTPTransport(config.DefaultConfig.ClientID, config.DefaultConfig.Server[0], tlsConfig, UserAgent, time.Second*5)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot create HTTP transport: %w", err), 1)
			}
		default:
			return cli.Exit(fmt.Errorf("unsupported transport protocol: %v", config.DefaultConfig.Protocol), 1)
		}
		client := NewClient(dispatcher, transporter)
		if err := client.Connect(); err != nil {
			return cli.Exit(fmt.Errorf("cannot connect client: %w", err), 1)
		}

		// Start a goroutine that receives values on the 'TLSEvents' channel and
		// reloads the transporter and HTTP client TLS configurations.
		// Depending on the transporter implementation, this may result in
		// active client disconnections and reconnections.
		go func() {
			// Can be that there are no files to watch
			if TlSEvents == nil {
				log.Info("no TLSconfig, disabling TLS watcher update.")
				return
			}

			for cfg := range TlSEvents {
				log.Debug("reloading transport TLS configuration")
				err := transporter.ReloadTLSConfig(cfg)
				if err != nil {
					log.Errorf("cannot update transporter TLS config: %v", err)
					continue
				}
				log.Info("transport TLS configuration reloaded")

				log.Debug("setting dispatcher HTTP client")
				httpClient := http.NewHTTPClient(cfg, UserAgent)
				dispatcher.HTTPClient = httpClient
				log.Info("dispatcher HTTP client updated")
			}
		}()

		// Publish connection-status in a goroutine
		go func() {
			msg, err := client.ConnectionStatus()
			if err != nil {
				log.Fatalf("cannot get connection status: %v", err)
			}
			if _, _, _, err := client.SendConnectionStatusMessage(msg); err != nil {
				log.Errorf("cannot send connection status message: %v", err)
			}
		}()

		// Start a goroutine watching for changes to the CanonicalFacts file and
		// publish a new connection-status message if the file changes.
		if config.DefaultConfig.CanonicalFacts != "" {
			go func() {
				c := make(chan notify.EventInfo, 1)
				if err := notify.Watch(config.DefaultConfig.CanonicalFacts, c, notify.InCloseWrite); err != nil {
					log.Infof("cannot start watching '%v': %v", config.DefaultConfig.CanonicalFacts, err)
					return
				}
				defer notify.Stop(c)

				for e := range c {
					switch e.Event() {
					case notify.InCloseWrite:
						go func() {
							msg, err := client.ConnectionStatus()
							if err != nil {
								log.Fatalf("cannot get connection status: %v", err)
							}
							if _, _, _, err := client.SendConnectionStatusMessage(msg); err != nil {
								log.Errorf("cannot send connection status message: %v", err)
							}
						}()
					}
				}
			}()
		}

		// Start a goroutine that receives values on the 'dispatchers' channel
		// and publishes "connection-status" messages to MQTT.
		var prevDispatchersHash atomic.Value
		go func() {
			for dispatchers := range dispatcher.Dispatchers {
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
						log.Fatalf("cannot get connection status: %v", err)
					}
					if _, _, _, err = client.SendConnectionStatusMessage(msg); err != nil {
						log.Errorf("cannot send connection status: %v", err)
					}
				}()
			}
		}()

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
							log.Fatalf("cannot get connection status: %v", err)
						}
						if _, _, _, err = client.SendConnectionStatusMessage(msg); err != nil {
							log.Errorf("cannot send connection status: %v", err)
						}
					}()
				}
			}
		}()

		watchdogDuration, err := daemon.SdWatchdogEnabled(false)
		if err != nil {
			log.Errorf("cannot get watchdog duration: %v", err)
		}
		go func() {
			for {
				if _, err := daemon.SdNotify(false, daemon.SdNotifyWatchdog); err != nil {
					log.Errorf("cannot call sd_notify: %v", err)
				}
				time.Sleep(watchdogDuration / 2)
			}
		}()

		if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
			log.Errorf("cannot call sd_notify: %v", err)
		}

		<-quit

		return nil
	}
	app.EnableBashCompletion = true
	app.BashComplete = BashComplete

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
