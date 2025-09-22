package constants

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

var (
	// Version is the version as described by git.
	Version string

	// DefaultPathPrefix is the default value used as a prefix to all transport
	// layer path names in the client.
	DefaultPathPrefix string = "yggdrasil"

	// DefaultDataHost is the default value used to force sending all HTTP
	// traffic to a specific host.
	DefaultDataHost string

	// DefaultFactsFile is the default value used to read facts about the host.
	DefaultFactsFile string
)

// Installation directory prefix and paths. Values have hard-coded defaults but
// can be changed at compile time by overriding the variable with an ldflag.
var (
	PrefixDir     string = filepath.Join("/")
	SysconfDir    string = filepath.Join(PrefixDir, "etc")
	LocalstateDir string = filepath.Join(PrefixDir, "var")
	DataDir       string = filepath.Join(PrefixDir, "usr", "share")
	LibDir        string = filepath.Join(PrefixDir, "usr", "lib")

	// ConfigDir is a path to a location where configuration data is assumed to
	// be stored. For non-root users, this is set to $CONFIGURATION_DIRECTORY or
	// $XDG_CONFIG_HOME/yggdrasil. Otherwise, it gets set to /etc/yggdrasil.
	ConfigDir string = filepath.Join(SysconfDir, "yggdrasil")

	// StateDir is a path to a location where local state information can be
	// stored. For non-root users, this is set to $STATE_DIRECTORY or
	// $XDG_STATE_HOME/yggdrasil. Otherwise, it gets set to /var/lib/yggdrasil.
	StateDir string = filepath.Join(LocalstateDir, "lib", "yggdrasil")

	// CacheDir is a path to a location where cache data can be stored. For
	// non-root users, this is set to $CACHE_DIRECTORY or
	// $XDG_CACHE_HOME/yggdrasil. Otherwise, it gets set to
	// /var/cache/yggdrasil.
	CacheDir string = filepath.Join(LocalstateDir, "cache", "yggdrasil")

	// DBusSystemServicesDir is a path to a location where D-Bus bus-activable
	// system service definition files are stored.
	DBusSystemServicesDir string = filepath.Join(DataDir, "dbus-1", "system-services")

	// DBusPolicyConfigDir is a path to a location where D-Bus policy
	// configuration definition files are stored.
	DBusPolicyConfigDir string = filepath.Join(DataDir, "dbus-1", "system.d")

	// SystemdSystemServicesDir is a path to a location where systemd system
	// service unit files are stored.
	SystemdSystemServicesDir string = filepath.Join(LibDir, "systemd", "system")
)

func init() {
	if os.Getuid() > 0 {
		ConfigDir = lookupEnv("CONFIGURATION_DIRECTORY", filepath.Join(xdg.ConfigHome, "yggdrasil"))
		StateDir = lookupEnv("STATE_DIRECTORY", filepath.Join(xdg.StateHome, "yggdrasil"))
		CacheDir = lookupEnv("CACHE_DIRECTORY", filepath.Join(xdg.CacheHome, "yggdrasil"))
	}
}

// lookupEnv looks up key in the environment. If a value is present, it is
// returned. Otherwise, defaultValue is returned.
func lookupEnv(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
