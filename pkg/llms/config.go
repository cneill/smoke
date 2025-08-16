package llms

import "fmt"

type Config struct {
	MaxTokens int64
	Model     string
	Provider  LLMType
	APIKey    string
}

func (c *Config) OK() error {
	switch {
	case c.MaxTokens <= 0:
		return fmt.Errorf("max tokens must be >0")
	case c.Model == "":
		return fmt.Errorf("missing model")
	case c.Provider == "":
		return fmt.Errorf("missing provider")
	case c.APIKey == "":
		return fmt.Errorf("missing api key")
	}

	return nil
}
