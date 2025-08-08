package main

import (
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/log"
	"github.com/cneill/smoke/pkg/models/ui"
	"github.com/urfave/cli/v2"
)

const (
	FlagDir          = "dir"
	FlagDebug        = "debug"
	FlagMaxTokens    = "max-tokens"
	FlagSessionName  = "session-name"
	FlagProvider     = "provider"
	FlagOpenAIKey    = "openai-api-key"
	FlagAnthropicKey = "anthropic-api-key"

	EnvDir          = "SMOKE_DIRECTORY"
	EnvDebug        = "SMOKE_DEBUG"
	EnvMaxTokens    = "SMOKE_MAX_TOKENS"
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
			Name:     FlagSessionName,
			Usage:    "The name of the session",
			Category: "Models",
			Aliases:  []string{"s"},
			EnvVars:  []string{EnvSessionName},
			Value:    "session",
		},
		&cli.StringFlag{
			Name:     FlagProvider,
			Usage:    "Either 'chatGPT' or 'claude'",
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

func run() error {
	var logFile *os.File
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	app := &cli.App{
		Name:        "smoke",
		HelpName:    "smoke",
		Description: "Smoke 'em if you got 'em.",
		Flags:       flags(),
		Action: func(ctx *cli.Context) error {
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

			smokeModel, err := ui.New()
			if err != nil {
				return fmt.Errorf("failed to set up UI: %w", err)
			}

			p := tea.NewProgram(smokeModel)

			if _, err := p.Run(); err != nil {
				return fmt.Errorf("app error: %w", err)
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		return fmt.Errorf("run error: %w", err)
	}

	return nil
}

func main() {
	// f, err := os.Create("trace.out")
	// if err != nil {
	// 	panic(fmt.Errorf("trace file error: %w", err))
	// }
	//
	// defer func() {
	// 	if err := f.Close(); err != nil {
	// 		fmt.Printf("Failed to close trace.out: %v\n", err)
	// 	}
	// }()
	//
	// if err := trace.Start(f); err != nil {
	// 	panic(err)
	// }
	// defer trace.Stop()

	if err := run(); err != nil {
		panic(fmt.Errorf("error: %w", err))
	}
}
