package llms

import "fmt"

type Config struct {
	APIKey      string
	BaseURL     string
	Effort      string
	MaxTokens   int64
	Model       string
	NoStream    bool
	Provider    LLMType
	Temperature float64
}

func (c *Config) OK() error {
	// Not validating temperature here, ranges vary by provider
	switch {
	case c.APIKey == "" && c.BaseURL == "":
		// Currently, a BaseURL indicates that we're using ollama, which does not require an API key
		// TODO: make this more explicit
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
