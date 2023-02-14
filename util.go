package yggdrasil

import (
	"os"
	"path/filepath"

	"github.com/redhatinsights/yggdrasil/internal/constants"
)

// ConfigPath returns an appropriate path to a config file. If the created path
// does not exist, an empty string is returned.
func ConfigPath() (string, error) {
	filePath := filepath.Join(constants.ConfigDir, "config.toml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	return filePath, nil
}
