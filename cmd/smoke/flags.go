package main

import (
	"strings"

	"github.com/urfave/cli/v3"
)

const (
	FlagDir          = "dir"
	FlagDebug        = "debug"
	FlagMaxTokens    = "max-tokens"
	FlagModel        = "model"
	FlagSessionName  = "session"
	FlagProvider     = "provider"
	FlagTemperature  = "temperature"
	FlagOpenAIKey    = "openai-api-key"
	FlagAnthropicKey = "anthropic-api-key"
	FlagXAIKey       = "xai-api-key"

	EnvDir          = "SMOKE_DIRECTORY"
	EnvDebug        = "SMOKE_DEBUG"
	EnvMaxTokens    = "SMOKE_MAX_TOKENS"
	EnvModel        = "SMOKE_MODEL"
	EnvSessionName  = "SMOKE_SESSION"
	EnvProvider     = "SMOKE_PROVIDER"
	EnvTemperature  = "SMOKE_TEMPERATURE"
	EnvOpenAIKey    = "OPENAI_API_KEY"
	EnvAnthropicKey = "ANTHROPIC_API_KEY"
	EnvXAIKey       = "XAI_API_KEY"
)

func flags() []cli.Flag {
	flags := []cli.Flag{}
	flags = append(flags, localConfigFlags()...)
	flags = append(flags, llmConfigFlags()...)
	flags = append(flags, providerFlags()...)

	return flags
}

func localConfigFlags() []cli.Flag {
	category := "Local Configuration"

	return []cli.Flag{
		&cli.StringFlag{
			Name:     FlagDir,
			Usage:    "The `DIRECTORY` where your project lives.",
			Category: category,
			Aliases:  []string{"d"},
			Required: true,
			Sources:  cli.EnvVars(EnvDir),
		},
		&cli.BoolFlag{
			Name:     FlagDebug,
			Usage:    "Enable debug logging.",
			Category: category,
			Aliases:  []string{"D"},
			Sources:  cli.EnvVars(EnvDebug),
		},
		&cli.StringFlag{
			Name:     FlagSessionName,
			Usage:    "The `NAME` of the session, which will be used to derive the log file and plan file names",
			Category: category,
			Aliases:  []string{"s"},
			Sources:  cli.EnvVars(EnvSessionName),
			Value:    "session",
		},
	}
}

func llmConfigFlags() []cli.Flag {
	category := "LLM Configuration"
	providers := getProviders()

	return []cli.Flag{
		&cli.StringFlag{
			Name:     FlagModel,
			Usage:    "The provider's model to use, or an alias for it",
			Category: category,
			Aliases:  []string{"m"},
			Sources:  cli.EnvVars(EnvModel),
		},
		&cli.StringFlag{
			Name:     FlagProvider,
			Usage:    "One of the following: " + strings.Join(providers.names(), ", "),
			Category: category,
			Aliases:  []string{"p"},
			Sources:  cli.EnvVars(EnvProvider),
			Required: true,
		},
		&cli.Int64Flag{
			Name:     FlagMaxTokens,
			Usage:    "The max tokens to return in any given response",
			Category: category,
			Aliases:  []string{"t"},
			Sources:  cli.EnvVars(EnvMaxTokens),
			Value:    8192,
		},
		&cli.Float64Flag{
			Name:     FlagTemperature,
			Usage:    "The temperature value to use with the model",
			Category: category,
			Aliases:  []string{"T"},
			Sources:  cli.EnvVars(EnvTemperature),
			Value:    1.0,
		},
	}
}

func providerFlags() []cli.Flag {
	category := "Providers"

	return []cli.Flag{
		&cli.StringFlag{
			Name:     FlagOpenAIKey,
			Category: category,
			Usage:    "The API key for OpenAI",
			Sources:  cli.EnvVars(EnvOpenAIKey),
		},
		&cli.StringFlag{
			Name:     FlagAnthropicKey,
			Category: category,
			Usage:    "The API key for Anthropic",
			Sources:  cli.EnvVars(EnvAnthropicKey),
		},
		&cli.StringFlag{
			Name:     FlagXAIKey,
			Category: category,
			Usage:    "The API key for xAI",
			Sources:  cli.EnvVars(EnvXAIKey),
		},
	}
}
