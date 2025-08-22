package llms

import "fmt"

type Config struct {
	APIKey      string
	MaxTokens   int64
	Model       string
	Provider    LLMType
	Temperature float64
}

func (c *Config) OK() error {
	switch {
	case c.APIKey == "":
		return fmt.Errorf("missing api key")
	case c.MaxTokens <= 0:
		return fmt.Errorf("max tokens must be >0")
	case c.Model == "":
		return fmt.Errorf("missing model")
	case c.Provider == "":
		return fmt.Errorf("missing provider")
		// don't validate temperature here, ranges vary by provider
	}

	return nil
}
