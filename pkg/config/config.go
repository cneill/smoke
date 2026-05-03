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
	// TODO: expose Providers, use the API keys if provided
	Providers *Providers `json:"-"`
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
	Name string `json:"name"`
	// Command is the name or the full path to the command to run.
	Command string `json:"command"`
	// Args is a list of arguments to pass to the command.
	Args []string `json:"args"`
	// Enabled determines whether we will create a client for this server.
	Enabled bool `json:"enabled"`
	// AllowedTools contains full names or glob patterns for the only tools that will be provided to the LLM. All are
	// allowed by default. Mutually exclusive with DeniedTools.
	AllowedTools []string `json:"allowed_tools"`
	// DeniedTools contains full names or glob patterns for tools that will not be provided to the LLM. All are allowed
	// by default. Mutually exclusive with AllowedTools.
	DeniedTools []string `json:"denied_tools"`
	// PlanTools contains full names or glob patterns for tools that will be allowed in planning mode. All allowed by
	// default.
	PlanTools []string `json:"plan_tools"`
	// Env contains environment variables that will be set for the command's process.
	Env []Env `json:"env"`
	// Type?
	// Directory?
}

type Env struct {
	Var   string `json:"var"`
	Value string `json:"value"`
}

func (m *MCPServer) OK() error {
	switch {
	case m.Name == "":
		return fmt.Errorf("missing name")
	case m.Command == "":
		return fmt.Errorf("missing command")
	case len(m.AllowedTools) > 0 && len(m.DeniedTools) > 0:
		return fmt.Errorf("must specify options in either allowed_tools OR denied_tools")
	}

	// Ensure that we don't have any malformed patterns in the allowed/denied tools globs
	// Ref: https://pkg.go.dev/path/filepath#Match
	patternChecks := map[string][]string{
		"allowed_tools": m.AllowedTools,
		"denied_tools":  m.DeniedTools,
		"plan_tools":    m.PlanTools,
	}

	for name, patterns := range patternChecks {
		for idx, pattern := range patterns {
			if _, err := filepath.Match(pattern, "."); err != nil {
				return fmt.Errorf("pattern at index %d in %s is invalid: %w; see https://pkg.go.dev/path/filepath#Match for syntax", idx, name, err)
			}
		}
	}

	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Providers: &Providers{},
		MCP: &MCP{
			Servers: []*MCPServer{
				{
					Name:         "gopls",
					Command:      "gopls",
					Args:         []string{"mcp"},
					Enabled:      true,
					AllowedTools: []string{},
					DeniedTools:  []string{},
					PlanTools:    []string{},
					Env:          nil,
				},
			},
		},
	}
}

func LoadConfig() (*Config, error) {
	configPath, err := GetConfigFilePath()
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
