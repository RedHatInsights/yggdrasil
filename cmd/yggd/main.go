package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/redhatinsights/yggdrasil/internal/config"
	"github.com/redhatinsights/yggdrasil/internal/constants"
	"github.com/redhatinsights/yggdrasil/internal/http"
	"github.com/redhatinsights/yggdrasil/internal/messagejournal"
	"github.com/redhatinsights/yggdrasil/internal/transport"
	"github.com/redhatinsights/yggdrasil/internal/work"

	"github.com/redhatinsights/yggdrasil"
	"github.com/rjeczalik/notify"
	"github.com/subpop/go-log"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

var (
	UserAgent = "yggd/" + constants.Version
)

// generateDocumentation tries to generate documentation for yggd.
// It can generate man page or markdown documentation for
// CLI options, arguments and subcommands.
func generateDocumentation(c *cli.Context) error {
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

// setupDefaultConfig sets up default configuration for yggd according to
// CLI flags and arguments.
func setupDefaultConfig(c *cli.Context) {
	config.DefaultConfig = config.Config{
		LogLevel:                 c.String(config.FlagNameLogLevel),
		ClientID:                 c.String(config.FlagNameClientID),
		Server:                   c.StringSlice(config.FlagNameServer),
		CertFile:                 c.String(config.FlagNameCertFile),
		KeyFile:                  c.String(config.FlagNameKeyFile),
		CARoot:                   c.StringSlice(config.FlagNameCaRoot),
		PathPrefix:               c.String(config.FlagNamePathPrefix),
		Protocol:                 c.String(config.FlagNameProtocol),
		DataHost:                 c.String(config.FlagNameDataHost),
		FactsFile:                c.String(config.FlagNameFactsFile),
		HTTPRetries:              c.Int(config.FlagNameHTTPRetries),
		HTTPTimeout:              c.Duration(config.FlagNameHTTPTimeout),
		MQTTConnectRetry:         c.Bool(config.FlagNameMQTTConnectRetry),
		MQTTConnectRetryInterval: c.Duration(config.FlagNameMQTTConnectRetryInterval),
		MQTTAutoReconnect:        c.Bool(config.FlagNameMQTTAutoReconnect),
		MQTTReconnectDelay:       c.Duration(config.FlagNameMQTTReconnectDelay),
		MQTTConnectTimeout:       c.Duration(config.FlagNameMQTTConnectTimeout),
		MQTTPublishTimeout:       c.Duration(config.FlagNameMQTTPublishTimeout),
		MessageJournal:           c.String(config.FlagNameMessageJournal),
	}
}

// setupLogging sets up logging for yggd
func setupLogging(c *cli.Context) error {
	level, err := log.ParseLevel(config.DefaultConfig.LogLevel)
	if err != nil {
		return cli.Exit(err, 1)
	}
	log.SetLevel(level)
	log.SetPrefix(fmt.Sprintf("[%v] ", c.App.Name))
	if log.CurrentLevel() >= log.LevelDebug {
		log.SetFlags(log.LstdFlags | log.Llongfile)
	}
	return nil
}

// setupClientID tries to create client ID for yggd. It tries to load
// client ID from certificate CN, if certificate is provided. If not, it
// tries to load client ID from file. If client ID is not found in file,
// it generates new client ID and saves it to file.
func setupClientID() error {
	clientIDFile := filepath.Join(constants.StateDir, "client-id")
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
	return nil
}

// setupFactsFile creates a canonical facts file if it doesn’t exist
func setupFactsFile() error {
	if config.DefaultConfig.FactsFile == "" {
		return nil
	}

	_, err := os.Stat(config.DefaultConfig.FactsFile)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("cannot stat file '%v': %v", config.DefaultConfig.FactsFile, err)
	}

	dir := filepath.Dir(config.DefaultConfig.FactsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create directory '%v': %v", dir, err)
	}

	facts := make(map[string]interface{})
	data, _ := json.Marshal(facts)
	if err := os.WriteFile(config.DefaultConfig.FactsFile, data, 0644); err != nil {
		return fmt.Errorf("cannot create facts file '%v': %v", config.DefaultConfig.FactsFile, err)
	}

	log.Infof("facts file not found, created '%v'", config.DefaultConfig.FactsFile)
	return nil
}

// setupClient tries to set up new client and transporter
func setupClient(
	dispatcher *work.Dispatcher,
	tlsConfig *tls.Config,
) (*Client, transport.Transporter, error) {
	var transporter transport.Transporter
	switch config.DefaultConfig.Protocol {
	case "mqtt":
		var err error
		transporter, err = transport.NewMQTTTransport(
			config.DefaultConfig.ClientID,
			config.DefaultConfig.Server,
			tlsConfig,
		)
		if err != nil {
			return nil, nil, cli.Exit(fmt.Errorf("cannot create MQTT transport: %w", err), 1)
		}
	case "http":
		var err error
		transporter, err = transport.NewHTTPTransport(
			config.DefaultConfig.ClientID, config.DefaultConfig.Server[0],
			tlsConfig,
			UserAgent,
			time.Second*5,
		)
		if err != nil {
			return nil, nil, cli.Exit(fmt.Errorf("cannot create HTTP transport: %w", err), 1)
		}
	case "none":
		var err error
		transporter, err = transport.NewNoopTransport()
		if err != nil {
			return nil, nil, cli.Exit(fmt.Errorf("cannot create no-op transport: %w", err), 1)
		}
		log.Info(
			"no network protocol specified - no data will be sent or received over the network",
		)
		for _, server := range config.DefaultConfig.Server {
			log.Warnf("no network protocol specified - ignoring server option '%v'", server)
		}
	default:
		return nil, nil, cli.Exit(
			fmt.Errorf("unsupported transport protocol: %v", config.DefaultConfig.Protocol),
			1,
		)
	}
	client := NewClient(dispatcher, transporter)
	if err := client.Connect(); err != nil {
		return nil, nil, cli.Exit(fmt.Errorf("cannot connect client: %w", err), 1)
	}
	return client, transporter, nil
}

// setupMessageJournal tries to set up a message journal database to track
// worker emitted events at the provided path.
func setupMessageJournal(client *Client) error {
	messageJournalPath := config.DefaultConfig.MessageJournal
	if messageJournalPath != "" {
		journalFilePath := filepath.Clean(messageJournalPath)
		journal, err := messagejournal.Open(journalFilePath)
		if err != nil {
			return cli.Exit(
				fmt.Errorf(
					"cannot initialize message journal database at '%v': %w",
					journalFilePath,
					err,
				),
				1,
			)
		}
		client.dispatcher.MessageJournal = journal
		log.Debugf("initialized message journal at '%v'", journalFilePath)
	}
	return nil
}

// setupTLS tries to set up new TLS config and HTTP client
func setupTLS() (*http.Client, *tls.Config, error) {
	tlsConfig, err := config.DefaultConfig.CreateTLSConfig()
	if err != nil {
		return nil, nil, cli.Exit(fmt.Errorf("cannot create TLS config: %w", err), 1)
	}

	httpClient := http.NewHTTPClient(tlsConfig, UserAgent)
	httpClient.Retries = config.DefaultConfig.HTTPRetries
	httpClient.Timeout = config.DefaultConfig.HTTPTimeout

	return httpClient, tlsConfig, nil
}

// publishConnectionStatus tries to publish connection status to server
func publishConnectionStatus(client *Client) {
	msg, err := client.ConnectionStatus()
	if err != nil {
		log.Fatalf("cannot get connection status: %v", err)
	}
	if _, _, _, err := client.SendConnectionStatusMessage(msg); err != nil {
		log.Errorf("cannot send connection status message: %v", err)
	}
}

// monitorFactsFile tries to monitor facts file for changes
func monitorFactsFile(client *Client) {
	if config.DefaultConfig.FactsFile == "" {
		return
	}
	c := make(chan notify.EventInfo, 1)
	if err := notify.Watch(config.DefaultConfig.FactsFile, c, notify.InCloseWrite); err != nil {
		log.Infof("cannot start watching '%v': %v", config.DefaultConfig.FactsFile, err)
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
}

// monitorTags tries to monitor tags file for changes
func monitorTags(client *Client) {
	c := make(chan notify.EventInfo, 1)

	fp := filepath.Join(constants.ConfigDir, "tags.toml")

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
}

// monitorCertificate tries to monitor certificate file for changes
func monitorCertificate(
	TlSEvents chan *tls.Config,
	transporter transport.Transporter,
	dispatcher *work.Dispatcher,
) {
	// Can be that there are no files to watch
	if TlSEvents == nil {
		log.Info("no TLS configuration, disabling TLS watcher update")
		return
	}

	for cfg := range TlSEvents {
		log.Debug("reloading transport TLS configuration")
		err := transporter.ReloadTLSConfig(cfg)
		if err != nil {
			log.Errorf("cannot update transporter TLS configuration: %v", err)
			continue
		}
		log.Info("transport TLS configuration reloaded")

		log.Debug("setting dispatcher HTTP client")
		httpClient := http.NewHTTPClient(cfg, UserAgent)
		dispatcher.HTTPClient = httpClient
		log.Info("dispatcher HTTP client updated")
	}
}

// systemdWatchDog tries to send sd_notify to systemd.
// More details about sd_notify can be found here:
// https://www.freedesktop.org/software/systemd/man/sd_notify.html
func systemdWatchDog() {
	watchdogDuration, err := daemon.SdWatchdogEnabled(false)
	if err != nil {
		log.Errorf("cannot get watchdog duration: %v", err)
	}
	if watchdogDuration > 0 {
		log.Debug("starting systemd watchdog notification")
		for {
			if _, err := daemon.SdNotify(false, daemon.SdNotifyWatchdog); err != nil {
				log.Errorf("cannot call sd_notify(%v): %v", daemon.SdNotifyWatchdog, err)
			}
			time.Sleep(watchdogDuration / 2)
		}
	}
}

// mainAction is main action of yggd
func mainAction(c *cli.Context) error {

	// First of all set up a channel to receive the TERM or INT signal
	// over and clean up before quitting.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	// Generate documentation, when hidden flag is set
	if c.Bool("generate-man-page") || c.Bool("generate-markdown") {
		err := generateDocumentation(c)
		if err != nil {
			return err
		}
		return nil
	}

	// Setup configuration according to CLI flags and options
	setupDefaultConfig(c)

	// Setup logging according to configuration
	err := setupLogging(c)
	if err != nil {
		return err
	}
	log.Infof("starting %v version %v", c.App.Name, c.App.Version)

	// Tries to create file containing client ID
	err = setupClientID()
	if err != nil {
		return err
	}

	err = setupFactsFile()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot setup facts file: %v", err), 1)
	}

	// Create HTTP client and TLS configuration. HTTP client is used for
	// getting data, when MQTT could not transport too big messages.
	httpClient, tlsConfig, err := setupTLS()
	if err != nil {
		return err
	}

	// Create Dispatcher service
	dispatcher := work.NewDispatcher(httpClient)

	// Create Transporter service (it could be HTTP or MQTT according to configuration)
	// This also starts probably the most important goroutine waiting for messages
	// from the Transporter
	client, transporter, err := setupClient(dispatcher, tlsConfig)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot setup client: %w", err), 1)
	}

	// Create a message journal if a journal path is provided
	// or if it is enabled in the config.
	// This message journal contains a persistent database that
	// tracks events emitted by workers across yggd sessions.
	err = setupMessageJournal(client)
	if err != nil {
		return err
	}

	// Create watcher for certificate changes
	TlSEvents, err := config.DefaultConfig.WatcherUpdate()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot start watching for certificate changes: %w", err), 1)
	}

	// Start a goroutine that receives values on the 'TLSEvents' channel and
	// reloads the transporter and HTTP client TLS configurations.
	// Depending on the transporter implementation, this may result in
	// active client disconnections and reconnections.
	go monitorCertificate(TlSEvents, transporter, dispatcher)

	// Publish connection-status in a goroutine
	go publishConnectionStatus(client)

	// Start a goroutine watching for changes to the facts file and publish a
	// new connection-status message if the file changes.
	go monitorFactsFile(client)

	// Start a goroutine that watches the tags file for write events and
	// publishes connection status messages when the file changes.
	go monitorTags(client)

	// Start a goroutine that sends notifications to systemd
	go systemdWatchDog()

	// Notify systemd that yggd is ready
	var sdState = daemon.SdNotifyReady
	if _, err := daemon.SdNotify(false, sdState); err != nil {
		log.Errorf("cannot call sd_notify(%v): %v", sdState, err)
	}

	// Wait for SIGINT or SIGTERM signal
	<-quit

	// Notify systemd that yggd is stopping
	sdState = daemon.SdNotifyStopping
	if _, err := daemon.SdNotify(false, sdState); err != nil {
		log.Errorf("cannot call sd_notify(%v): %v", sdState, err)
	}

	return nil
}

// beforeAction loads flag values from a config file only if the
// "config" flag value is non-zero.
func beforeAction(c *cli.Context) error {
	filePath := c.String("config")
	if filePath != "" {
		inputSource, err := altsrc.NewTomlSourceFromFile(filePath)
		if err != nil {
			return err
		}
		return altsrc.ApplyInputSourceValues(c, inputSource, c.App.Flags)
	}
	return nil
}

// main is entry point for yggd daemon
func main() {
	app := cli.NewApp()
	app.Name = "yggd"
	app.Version = constants.Version
	app.Usage = "connect the system to the network in order to send and receive messages"

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
			Value:  constants.DefaultPathPrefix,
			Hidden: true,
			Usage:  "Use `PREFIX` as the transport layer path name prefix",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameProtocol,
			Usage: "Transmit data remotely using `PROTOCOL` ('mqtt', 'http' or 'none')",
			Value: "none",
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
			Value: constants.DefaultDataHost,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameClientID,
			Usage: "Use `VALUE` as the client ID when connecting",
		}),
		altsrc.NewPathFlag(&cli.PathFlag{
			Name:      config.FlagNameFactsFile,
			Usage:     "Read facts from `FILE`",
			TakesFile: true,
			Value:     constants.DefaultFactsFile,
		}),
		altsrc.NewIntFlag(&cli.IntFlag{
			Name:   config.FlagNameHTTPRetries,
			Usage:  "Retry HTTP requests `N` times",
			Hidden: true,
		}),
		altsrc.NewDurationFlag(&cli.DurationFlag{
			Name:   config.FlagNameHTTPTimeout,
			Usage:  "Wait for `DURATION` before cancelling an HTTP request",
			Hidden: true,
		}),
		altsrc.NewBoolFlag(&cli.BoolFlag{
			Name:   config.FlagNameMQTTConnectRetry,
			Usage:  "Enable automatic reconnection logic when the client initially connects",
			Value:  false,
			Hidden: true,
		}),
		altsrc.NewDurationFlag(&cli.DurationFlag{
			Name:   config.FlagNameMQTTConnectRetryInterval,
			Usage:  "Sets the time to wait between connection attempts to `DURATION`",
			Value:  30 * time.Second,
			Hidden: true,
		}),
		altsrc.NewBoolFlag(&cli.BoolFlag{
			Name:   config.FlagNameMQTTAutoReconnect,
			Usage:  "Enable automatic reconnection when the client disconnects",
			Value:  true,
			Hidden: true,
		}),
		altsrc.NewDurationFlag(&cli.DurationFlag{
			Name:   config.FlagNameMQTTReconnectDelay,
			Usage:  "Sets the time to wait before attempting to reconnect to `DURATION`",
			Value:  0 * time.Second,
			Hidden: true,
		}),
		altsrc.NewDurationFlag(&cli.DurationFlag{
			Name:   config.FlagNameMQTTConnectTimeout,
			Usage:  "Sets the time to wait before giving up to `DURATION` when connecting to an MQTT broker",
			Value:  30 * time.Second,
			Hidden: true,
		}),
		altsrc.NewDurationFlag(&cli.DurationFlag{
			Name:   config.FlagNameMQTTPublishTimeout,
			Usage:  "Sets the time to wait before giving up to `DURATION` when publishing a message to an MQTT broker",
			Value:  30 * time.Second,
			Hidden: true,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:  config.FlagNameMessageJournal,
			Usage: "Record worker events and messages in the database `FILE`",
		}),
	}

	app.EnableBashCompletion = true
	app.BashComplete = BashComplete

	app.Before = beforeAction
	app.Action = mainAction

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
