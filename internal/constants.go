package yggdrasil

import "path/filepath"

// Installation directory prefix and paths. Values are specified by compile-time
// substitution values, and are then set to sane defaults at runtime if the
// value is a zero-value string.
var (
	PrefixDir     string
	BinDir        string
	SbinDir       string
	LibexecDir    string
	DataDir       string
	DatarootDir   string
	ManDir        string
	DocDir        string
	SysconfDir    string
	LocalstateDir string
)

func init() {
	if PrefixDir == "" {
		PrefixDir = "/usr/local"
	}
	if BinDir == "" {
		BinDir = filepath.Join(PrefixDir, "bin")
	}
	if SbinDir == "" {
		SbinDir = filepath.Join(PrefixDir, "sbin")
	}
	if LibexecDir == "" {
		LibexecDir = filepath.Join(PrefixDir, "libexec")
	}
	if DataDir == "" {
		DataDir = filepath.Join(PrefixDir, "share")
	}
	if DatarootDir == "" {
		DatarootDir = filepath.Join(PrefixDir, "share")
	}
	if ManDir == "" {
		ManDir = filepath.Join(PrefixDir, "man")
	}
	if DocDir == "" {
		DocDir = filepath.Join(PrefixDir, "doc")
	}
	if SysconfDir == "" {
		SysconfDir = filepath.Join(PrefixDir, "etc")
	}
	if LocalstateDir == "" {
		LocalstateDir = filepath.Join(LocalstateDir, "var")
	}
}
