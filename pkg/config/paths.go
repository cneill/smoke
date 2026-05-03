package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetConfigDirPath() (string, error) {
	var result string

	// $XDG_CONFIG_HOME/smoke OR $HOME/.config/smoke
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		result = filepath.Join(xdgConfig, "smoke")
	} else {
		home, _ := os.UserHomeDir()
		result = filepath.Join(home, ".config", "smoke")
	}

	if result == "" {
		return "", fmt.Errorf("could not construct config dir path from $XDG_CONFIG_HOME or $HOME")
	}

	return result, nil
}

func GetConfigFilePath() (string, error) {
	configDir, err := GetConfigDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "config.json"), nil
}
