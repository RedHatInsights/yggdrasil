package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"

	"git.sr.ht/~spc/go-log"
	"github.com/rjeczalik/notify"
)

const (
	cliLogLevel      = "log-level"
	cliCertFile      = "cert-file"
	cliKeyFile       = "key-file"
	cliCaRoot        = "ca-root"
	cliServer        = "server"
	cliSocketAddr    = "socket-addr"
	cliClientID      = "client-id"
	cliPathPrefix    = "path-prefix"
	cliProtocol      = "protocol"
	cliDataHost      = "data-host"
	cliExcludeWorker = "exclude-worker"
)

// Config contains current configuration state for yggdrasil.
type Config struct {
	// LogLevel is the level value used for logging.
	LogLevel string

	// ClientID is a unique identification value for the client over connection
	// transports.
	ClientID string

	// SocketAddr is the socket address on which yggd is listening.
	SocketAddr string

	// Server is a URI to which yggd connects in order to send and receive data.
	Server string

	// CertFile is a path to a public certificate, optionally used along with
	// KeyFile to authenticate connections.
	CertFile string

	// KeyFile is a path to a private certificate, optionally used along with
	// CertFile to authenticate connections.
	KeyFile string

	// CARoot is the list of paths with chain certificate file to optionally
	// include in the TLS configration's CA root list.
	CARoot []string

	// PathPrefix is a value prepended to all path names at the transport layer.
	PathPrefix string

	// Protocol is the protocol used by yggd when connecting to Server. Can be
	// either MQTT or HTTP.
	Protocol string

	// DataHost is a hostname value to interject into all HTTP requests when
	// handling data retrieval for "detachedContent" workers.
	DataHost string

	// ExcludeWorkers contains worker names to be excluded from starting when
	// yggd starts.
	ExcludeWorkers map[string]bool
}

func (conf *Config) CreateTLSConfig() (*tls.Config, error) {
	var certData, keyData []byte
	var err error
	rootCAs := make([][]byte, 0)

	if conf.CertFile != "" && conf.KeyFile != "" {
		certData, err = ioutil.ReadFile(conf.CertFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read cert-file '%v': %v", conf.CertFile, err)
		}

		keyData, err = ioutil.ReadFile(conf.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read key-file '%v': %v", conf.KeyFile, err)
		}
	}

	for _, file := range conf.CARoot {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("cannot read ca-file '%v': ", err)
		}
		rootCAs = append(rootCAs, data)
	}

	tlsConfig, err := newTLSConfig(certData, keyData, rootCAs)
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

func (conf *Config) WatcherUpdate() error {
	c := make(chan notify.EventInfo, 1)
	files := []string{}

	if len(conf.CARoot) > 0 {
		files = append(files, conf.CARoot...)
	}

	if conf.CertFile != "" {
		files = append(files, conf.CertFile)
	}

	if conf.KeyFile != "" {
		files = append(files, conf.KeyFile)
	}

	if len(files) == 0 {
		return nil
	}

	for _, fp := range files {
		if err := notify.Watch(fp, c, notify.InCloseWrite, notify.InDelete); err != nil {
			return fmt.Errorf("cannot start watching '%v': %v", fp, err)
		}
		log.Debugf("Added watcher for '%s'", fp)
	}

	go func() {
		for e := range c {
			log.Debugf("received inotify event %v", e.Event())
			switch e.Event() {
			case notify.InCloseWrite, notify.InDelete:
				_, err := conf.CreateTLSConfig()
				if err != nil {
					log.Errorf("Cannot update TLS config for '%s' on event %v: %v", e.Path(), e.Event(), err)
				}
			}
		}
	}()

	return nil
}
