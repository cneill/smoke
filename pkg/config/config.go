package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	Providers *Providers `json:"providers"`
	MCP       *MCP       `json:"mcp"`
}

func (c *Config) OK() error {
	if err := c.MCP.OK(); err != nil {
		return fmt.Errorf("error with MCP config: %w", c.MCP.OK())
	}

	return nil
}

type Providers struct {
	AnthropicKey string `json:"anthropic_key"`
	OpenAIKey    string `json:"openai_key"`
	XAIKey       string `json:"xai_key"`
}

type MCP struct {
	Servers []*MCPServer `json:"servers"`
}

func (m *MCP) OK() error {
	if m == nil {
		return nil
	}

	for i, server := range m.Servers {
		if err := server.OK(); err != nil {
			return fmt.Errorf("error with server at index %d: %w", i, err)
		}
	}

	return nil
}

type MCPServer struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled bool     `json:"enabled"`
	// Type?
	// Directory?
}

func (m *MCPServer) OK() error {
	switch {
	case m.Name == "":
		return fmt.Errorf("missing name")
	case m.Command == "":
		return fmt.Errorf("missing command")
	}

	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Providers: &Providers{},
		MCP: &MCP{
			Servers: []*MCPServer{
				{
					Name:    "gopls",
					Command: "gopls",
					Args:    []string{"mcp"},
					Enabled: true,
				},
			},
		},
	}
}

func LoadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config file path: %w", err)
	}

	var configFile *os.File

	if _, err := os.Stat(configPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("failed to stat config file at path %q: %w", configPath, err)
		}

		file, err := createDefaultConfigFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create default config file at path %q: %w", configPath, err)
		}

		configFile = file
	} else {
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file at path %q: %w", configPath, err)
		}

		configFile = file
	}

	defer configFile.Close()

	contents, err := io.ReadAll(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file at path %q: %w", configPath, err)
	}

	config := &Config{}
	if err := json.Unmarshal(contents, config); err != nil {
		return nil, fmt.Errorf("failed to parse config contents from config file at path %q: %w", configPath, err)
	}

	if err := config.OK(); err != nil {
		return nil, fmt.Errorf("error with contents of config file at path %q: %w", configPath, err)
	}

	return config, nil
}

func getConfigPath() (string, error) {
	var result string

	home, _ := os.UserHomeDir()

	// $XDG_CONFIG_HOME/smoke/config.json OR $HOME/.config/smoke/config.json
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		result = filepath.Join(xdgConfig, "smoke", "config.json")
	} else if home != "" {
		result = filepath.Join(home, ".config", "smoke", "config.json")
	}

	if result == "" {
		return "", fmt.Errorf("could not construct config file path from $XDG_CONFIG_HOME or $HOME")
	}

	return result, nil
}

func createDefaultConfigFile(configPath string) (*os.File, error) {
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("failed to stat config directory: %w", err)
		}

		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	defaultBytes, err := json.MarshalIndent(DefaultConfig(), "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default contents: %w", err)
	}

	if _, err := file.Write(defaultBytes); err != nil {
		return nil, fmt.Errorf("failed to write default config bytes: %w", err)
	}

	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file cursor after writing default values: %w", err)
	}

	return file, nil
}
