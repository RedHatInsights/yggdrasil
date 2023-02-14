package constants

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

var (
	// Version is the version as described by git.
	Version string

	// DefaultPathPrefix is the default vlaue used as a prefix to all transport
	// layer path names in the client.
	DefaultPathPrefix string

	// DefaultDataHost is the default value used to force sending all HTTP
	// traffic to a specific host.
	DefaultDataHost string
)

// Installation directory prefix and paths. Values have hard-coded defaults but
// can be changed at compile time by overriding the variable with an ldflag.
var (
	PrefixDir     string = "/usr/local"
	SysconfDir    string = filepath.Join(PrefixDir, "etc")
	LocalstateDir string = filepath.Join(PrefixDir, "var")

	// ConfigDir is a path to a location where configuration data is assumed to
	// be stored. For non-root users, this is set to $XDG_CONFIG_HOME. Otherwise,
	// it gets set to /etc/yggdrasil.
	ConfigDir string = filepath.Join(SysconfDir, "yggdrasil")

	// StateDir is a path to a location where local state information can be
	// stored. For non-root users, this is set to $XDG_STATE_HOME. Otherwise, it
	// gets set to /var/lib/yggdrasil.
	StateDir string = filepath.Join(LocalstateDir, "lib", "yggdrasil")

	// CacheDir is a path to a location where cache data can be stored. For
	// non-root users, this is set to $XDG_CACHE_HOME. Otherwise, it gets set to
	// /var/cache/yggdrasil.
	CacheDir string = filepath.Join(LocalstateDir, "cache", "yggdrasil")
)

func init() {
	if os.Getuid() > 0 {
		ConfigDir = filepath.Join(xdg.ConfigHome, "yggdrasil")
		StateDir = filepath.Join(xdg.StateHome, "yggdrasil")
		CacheDir = filepath.Join(xdg.CacheHome, "yggdrasil")
	}
}
