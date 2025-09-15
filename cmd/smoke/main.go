package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/llms/chatgpt"
	"github.com/cneill/smoke/pkg/llms/claude"
	"github.com/cneill/smoke/pkg/llms/grok"
	"github.com/cneill/smoke/pkg/log"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/models/ui"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/openai/openai-go/v2"
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
			Name:     FlagSessionName,
			Usage:    "The name of the session",
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
			Usage:    fmt.Sprintf("Either '%s', '%s', or '%s'", llms.LLMTypeChatGPT, llms.LLMTypeClaude, llms.LLMTypeGrok),
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

func validate(ctx *cli.Command) error {
	switch llms.LLMType(ctx.String(FlagProvider)) {
	case llms.LLMTypeChatGPT:
		if ctx.String(FlagOpenAIKey) == "" {
			return fmt.Errorf("must supply %s flag or %s environment variable", FlagOpenAIKey, EnvOpenAIKey)
		}
	case llms.LLMTypeClaude:
		if ctx.String(FlagAnthropicKey) == "" {
			return fmt.Errorf("must supply %s flag or %s environment variable", FlagAnthropicKey, EnvAnthropicKey)
		}
	case llms.LLMTypeGrok:
		if ctx.String(FlagXAIKey) == "" {
			return fmt.Errorf("must supply %s flag or %s environment variable", FlagXAIKey, EnvXAIKey)
		}
	default:
		return fmt.Errorf("unknown model provider: %s", ctx.String(FlagProvider))
	}

	return nil
}

func run() error {
	var logFile *os.File

	command := &cli.Command{
		Name:        "smoke",
		Description: "Smoke 'em if you got 'em.",
		Flags:       flags(),
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if err := validate(cmd); err != nil {
				return nil, fmt.Errorf("flag validation error: %w", err)
			}

			level := slog.LevelInfo
			if cmd.Bool(FlagDebug) {
				level = slog.LevelDebug
			}

			logFileName := cmd.String(FlagSessionName) + "_log.log"
			var err error
			logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return nil, fmt.Errorf("failed to open log file")
			}

			log.Setup(logFile, level)

			return ctx, nil
		},
		Action: action,
		After: func(_ context.Context, _ *cli.Command) error {
			if logFile != nil {
				if err := logFile.Close(); err != nil {
					return fmt.Errorf("failed to close log file %q: %w", logFile.Name(), err)
				}
			}

			return nil
		},
	}

	if err := command.Run(context.TODO(), os.Args); err != nil {
		return fmt.Errorf("run error: %w", err)
	}

	return nil
}

func action(ctx context.Context, cmd *cli.Command) error {
	provider := llms.LLMType(cmd.String(FlagProvider))
	modelFlag := cmd.String(FlagModel)
	sessionName := cmd.String(FlagSessionName)
	projectPath := cmd.String(FlagDir)

	llmConfig := &llms.Config{
		MaxTokens:   cmd.Int64(FlagMaxTokens),
		Provider:    provider,
		Temperature: cmd.Float64(FlagTemperature),
	}

	switch provider {
	case llms.LLMTypeChatGPT:
		llmConfig.APIKey = cmd.String(FlagOpenAIKey)
		llmConfig.Model = chatgpt.GetModel(modelFlag, openai.ChatModelGPT5)
	case llms.LLMTypeClaude:
		llmConfig.APIKey = cmd.String(FlagAnthropicKey)
		llmConfig.Model = string(claude.GetModel(modelFlag, anthropic.ModelClaudeSonnet4_0))
	case llms.LLMTypeGrok:
		llmConfig.APIKey = cmd.String(FlagXAIKey)
		llmConfig.Model = grok.GetModel(modelFlag, "grok-3")
	}

	// TODO: allow for more MCP clients
	mcpClient, err := mcp.NewClient(ctx, projectPath, "gopls", "mcp")
	if err != nil {
		return fmt.Errorf("failed to set up gopls MCP client: %w", err)
	}

	opts := []smoke.OptFunc{
		smoke.WithDebug(cmd.Bool(FlagDebug)),
		smoke.WithProjectPath(projectPath),
		smoke.WithSessionInfo(sessionName, prompts.SystemJSON()),
		smoke.WithLLMConfig(llmConfig),
		smoke.WithMCPClient(ctx, mcpClient),
	}

	smokeInstance, err := smoke.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to set up smoke: %w", err)
	}

	uiOpts := &ui.Opts{
		Smoke: smokeInstance,
	}

	uiModel, err := ui.New(uiOpts)
	if err != nil {
		return fmt.Errorf("failed to set up UI: %w", err)
	}

	p := tea.NewProgram(uiModel, tea.WithReportFocus(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("app error: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		panic(fmt.Errorf("error: %w", err))
	}
}
