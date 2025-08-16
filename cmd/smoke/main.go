package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/llms/chatgpt"
	"github.com/cneill/smoke/pkg/llms/claude"
	"github.com/cneill/smoke/pkg/log"
	"github.com/cneill/smoke/pkg/models/ui"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/openai/openai-go/v2"
	"github.com/urfave/cli/v2"
)

const (
	FlagDir          = "dir"
	FlagDebug        = "debug"
	FlagMaxTokens    = "max-tokens"
	FlagModel        = "model"
	FlagSessionName  = "session-name"
	FlagProvider     = "provider"
	FlagOpenAIKey    = "openai-api-key"
	FlagAnthropicKey = "anthropic-api-key"

	EnvDir          = "SMOKE_DIRECTORY"
	EnvDebug        = "SMOKE_DEBUG"
	EnvMaxTokens    = "SMOKE_MAX_TOKENS"
	EnvModel        = "SMOKE_MODEL"
	EnvSessionName  = "SMOKE_SESSION_NAME"
	EnvProvider     = "SMOKE_PROVIDER"
	EnvOpenAIKey    = "OPENAI_API_KEY"
	EnvAnthropicKey = "ANTHROPIC_API_KEY"
)

func flags() []cli.Flag {
	return []cli.Flag{
		&cli.PathFlag{
			Name:     FlagDir,
			Usage:    "The `DIRECTORY` where your project lives.",
			Aliases:  []string{"d"},
			Required: true,
			EnvVars:  []string{EnvDir},
		},
		&cli.BoolFlag{
			Name:    FlagDebug,
			Usage:   "Enable debug logging.",
			Aliases: []string{"D"},
			EnvVars: []string{EnvDebug},
		},
		&cli.Int64Flag{
			Name:     FlagMaxTokens,
			Usage:    "The max tokens to return in any given response",
			Category: "Models",
			Aliases:  []string{"t"},
			EnvVars:  []string{EnvMaxTokens},
			Value:    8192,
		},
		&cli.StringFlag{
			Name:     FlagModel,
			Usage:    "The provider's model to use, or an alias for it",
			Category: "Models",
			Aliases:  []string{"m"},
			EnvVars:  []string{EnvModel},
		},
		&cli.StringFlag{
			Name:     FlagSessionName,
			Usage:    "The name of the session",
			Category: "Models",
			Aliases:  []string{"s"},
			EnvVars:  []string{EnvSessionName},
			Value:    "session",
		},
		&cli.StringFlag{
			Name:     FlagProvider,
			Usage:    fmt.Sprintf("Either '%s' or '%s'", llms.LLMTypeChatGPT, llms.LLMTypeClaude),
			Category: "Models",
			Aliases:  []string{"p"},
			EnvVars:  []string{EnvProvider},
			Required: true,
		},
		&cli.StringFlag{
			Name:     FlagOpenAIKey,
			Category: "Models",
			Usage:    "The API key for OpenAI",
			EnvVars:  []string{EnvOpenAIKey},
		},
		&cli.StringFlag{
			Name:     FlagAnthropicKey,
			Category: "Models",
			Usage:    "The API key for Anthropic",
			EnvVars:  []string{EnvAnthropicKey},
		},
	}
}

func validate(ctx *cli.Context) error {
	switch llms.LLMType(ctx.String(FlagProvider)) {
	case llms.LLMTypeChatGPT:
		if ctx.String(FlagOpenAIKey) == "" {
			return fmt.Errorf("must supply %s flag or %s environment variable", FlagOpenAIKey, EnvOpenAIKey)
		}
	case llms.LLMTypeClaude:
		if ctx.String(FlagAnthropicKey) == "" {
			return fmt.Errorf("must supply %s flag or %s environment variable", FlagAnthropicKey, EnvAnthropicKey)
		}
	default:
		return fmt.Errorf("unknown model provider: %s", ctx.String(FlagProvider))
	}

	return nil
}

func run() error {
	var logFile *os.File

	app := &cli.App{
		Name:        "smoke",
		HelpName:    "smoke",
		Description: "Smoke 'em if you got 'em.",
		Flags:       flags(),
		Before: func(ctx *cli.Context) error {
			if err := validate(ctx); err != nil {
				return fmt.Errorf("flag validation error: %w", err)
			}

			level := slog.LevelInfo
			if ctx.Bool(FlagDebug) {
				level = slog.LevelDebug
			}

			logFileName := ctx.String(FlagSessionName) + "_log.log"
			var err error
			logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return fmt.Errorf("failed to open log file")
			}

			log.Setup(logFile, level)

			return nil
		},
		Action: action,
		After: func(_ *cli.Context) error {
			if logFile != nil {
				if err := logFile.Close(); err != nil {
					return fmt.Errorf("failed to close log file %q: %w", logFile.Name(), err)
				}
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		return fmt.Errorf("run error: %w", err)
	}

	return nil
}

func action(ctx *cli.Context) error {
	provider := llms.LLMType(ctx.String(FlagProvider))
	modelFlag := ctx.String(FlagModel)

	llmConfig := &llms.Config{
		MaxTokens: ctx.Int64(FlagMaxTokens),
		Provider:  provider,
	}

	switch provider {
	case llms.LLMTypeChatGPT:
		llmConfig.APIKey = ctx.String(FlagOpenAIKey)
		llmConfig.Model = chatgpt.GetModel(modelFlag, openai.ChatModelGPT5)
	case llms.LLMTypeClaude:
		llmConfig.APIKey = ctx.String(FlagAnthropicKey)
		llmConfig.Model = string(claude.GetModel(modelFlag, anthropic.ModelClaudeSonnet4_0))
	}

	opts := []smoke.OptFunc{
		smoke.WithDebug(ctx.Bool(FlagDebug)),
		smoke.WithProjectPath(ctx.Path(FlagDir)),
		smoke.WithSessionInfo(ctx.String(FlagSessionName), prompts.SystemJSON()),
		smoke.WithLLMConfig(llmConfig),
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
