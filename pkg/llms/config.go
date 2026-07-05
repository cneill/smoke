package llms

import "fmt"

type Config struct {
	APIKey      string
	BaseURL     string
	Effort      string
	MaxTokens   int64
	Model       string
	ContextSize int64
	NoStream    bool
	Provider    LLMType
	Temperature float64
}

func (c *Config) OK() error {
	// Not validating temperature here, ranges vary by provider
	switch {
	case c.APIKey == "" && c.Provider != LLMTypeOllama: // All other providers require an API key
		return fmt.Errorf("missing api key")
	case c.MaxTokens <= 0:
		return fmt.Errorf("max tokens must be >0")
	case c.Model == "":
		return fmt.Errorf("missing model")
	case c.Provider == "":
		return fmt.Errorf("missing provider")
	}

	return nil
}
