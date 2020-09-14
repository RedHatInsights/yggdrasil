package yggdrasil

import (
	"os"
	"path/filepath"
)

// ConfigPath returns an appropriate path to a config file, depending on whether
// the process is running as root or non-root. If the created path does not
// exist, an empty string is returned.
func ConfigPath() (string, error) {
	var prefix string
	if os.Getuid() == 0 {
		prefix = SysconfDir
	} else {
		var err error
		prefix, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}

	filePath := filepath.Join(prefix, "yggdrasil", "config.toml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	return filePath, nil
}
