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
	FlagSessionName  = "session-name"
	FlagProvider     = "provider"
	FlagTemperature  = "temperature"
	FlagOpenAIKey    = "openai-api-key"
	FlagAnthropicKey = "anthropic-api-key"
	FlagXAIKey       = "xai-api-key"

	EnvDir          = "SMOKE_DIRECTORY"
	EnvDebug        = "SMOKE_DEBUG"
	EnvMaxTokens    = "SMOKE_MAX_TOKENS"
	EnvModel        = "SMOKE_MODEL"
	EnvSessionName  = "SMOKE_SESSION_NAME"
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
	return []cli.Flag{
		&cli.StringFlag{
			Name:     FlagDir,
			Usage:    "The `DIRECTORY` where your project lives.",
			Category: "Local config",
			Aliases:  []string{"d"},
			Required: true,
			Sources:  cli.EnvVars(EnvDir),
		},
		&cli.BoolFlag{
			Name:     FlagDebug,
			Usage:    "Enable debug logging.",
			Category: "Local config",
			Aliases:  []string{"D"},
			Sources:  cli.EnvVars(EnvDebug),
		},
		&cli.StringFlag{
			Name: FlagSessionName,
			Usage: "The name of the session, which will be used for the log file name ([name]_log.log) and plan file " +
				"name ([name]_plan.json)",
			Category: "Local config",
			Aliases:  []string{"s"},
			Sources:  cli.EnvVars(EnvSessionName),
			Value:    "session",
		},
	}
}

func llmConfigFlags() []cli.Flag {
	return []cli.Flag{
		&cli.Int64Flag{
			Name:     FlagMaxTokens,
			Usage:    "The max tokens to return in any given response",
			Category: "LLM config",
			Aliases:  []string{"t"},
			Sources:  cli.EnvVars(EnvMaxTokens),
			Value:    8192,
		},
		&cli.Float64Flag{
			Name:     FlagTemperature,
			Usage:    "The temperature value to use with the model",
			Category: "LLM config",
			Aliases:  []string{"T"},
			Sources:  cli.EnvVars(EnvTemperature),
			Value:    1.0,
		},
	}
}

func providerFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     FlagModel,
			Usage:    "The provider's model to use, or an alias for it",
			Category: "Providers",
			Aliases:  []string{"m"},
			Sources:  cli.EnvVars(EnvModel),
		},
		&cli.StringFlag{
			Name:     FlagProvider,
			Usage:    "One of the following: " + strings.Join(getProviders().names(), ", "),
			Category: "Providers",
			Aliases:  []string{"p"},
			Sources:  cli.EnvVars(EnvProvider),
			Required: true,
		},
		&cli.StringFlag{
			Name:     FlagOpenAIKey,
			Category: "Providers",
			Usage:    "The API key for OpenAI",
			Sources:  cli.EnvVars(EnvOpenAIKey),
		},
		&cli.StringFlag{
			Name:     FlagAnthropicKey,
			Category: "Providers",
			Usage:    "The API key for Anthropic",
			Sources:  cli.EnvVars(EnvAnthropicKey),
		},
		&cli.StringFlag{
			Name:     FlagXAIKey,
			Category: "Providers",
			Usage:    "The API key for xAI",
			Sources:  cli.EnvVars(EnvXAIKey),
		},
	}
}
