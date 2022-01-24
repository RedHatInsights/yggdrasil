package main

const (
	cliLogLevel      = "log-level"
	cliCertFile      = "cert-file"
	cliKeyFile       = "key-file"
	cliCaRoot        = "ca-root"
	cliServer        = "server"
	cliSocketAddr    = "socket-addr"
	cliClientID      = "client-id"
	cliTopicPrefix   = "topic-prefix"
	cliProtocol      = "protocol"
	cliDataHost      = "data-host"
	cliExcludeWorker = "exclude-worker"
)

// Config struct that holds all yggd config.
type Config struct {
	// LogLevel is the level value used for logging.
	LogLevel string
	// ClientID is the ID of the client
	ClientId string
	// SocketAddr is the socket address where yggd is listening on.
	SocketAddr string
	// Server where yggd connects to.
	Server string
	// Client certificate used by yggd.
	CertFile string
	// Client key certificate used by yggd.
	KeyFile string
	// CaRoot path used by ygdd to verify TLS connections.
	CaRoot string
	// MQTT Topic prefix
	TopicPrefix string
	// Protocol used by yggd to connect upstream, can be HTTP or MQTT
	Protocol string
	// DataHost Host that needs to be used on HTTP request.
	DataHost string
	// ExcludeWorkers contains worker names to be excluded from starting when
	// yggd starts.
	ExcludeWorkers map[string]bool
}
