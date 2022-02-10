package main

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

	// CaRoot is a path to full chain certificate file to optionally include in
	// the TLS configration's CA root list.
	CaRoot string

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
