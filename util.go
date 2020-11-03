package yggdrasil

import (
	"os"
	"path/filepath"
)

// ConfigPath returns an appropriate path to a config file. If the created path
// does not exist, an empty string is returned.
func ConfigPath() (string, error) {
	prefix, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(prefix, LongName, "config.toml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	return filePath, nil
}
