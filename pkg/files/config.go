package files

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ConfigDirName = "smoke"
	ConfigName    = "config.json"
)

func ConfigDirPath() (string, error) {
	var configDir string

	// $XDG_CONFIG_HOME/smoke OR $HOME/.config/smoke
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		configDir = filepath.Join(xdgConfig, ConfigDirName)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not locate user's home directory: %w", err)
		}

		configDir = filepath.Join(home, ".config", ConfigDirName)
	}

	absDir, err := filepath.Abs(configDir)
	if err != nil {
		return "", fmt.Errorf("got invalid config dir %q: %w", configDir, err)
	}

	return absDir, nil
}

func ConfigFilePath() (string, error) {
	configDir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, ConfigName), nil
}

func PlansDirPath() (string, error) {
	configDir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "plans"), nil
}
