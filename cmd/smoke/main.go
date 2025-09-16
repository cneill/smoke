package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/config"
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

type App struct {
	config *config.Config
}

func (a *App) validate(ctx *cli.Command) error {
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

func (a *App) setup() error {
	var logFile *os.File

	command := &cli.Command{
		Name:        "smoke",
		Description: "Smoke 'em if you got 'em.",
		Flags:       flags(),
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if err := a.validate(cmd); err != nil {
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

			config, err := config.LoadConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to load config: %w", err)
			}

			a.config = config

			return ctx, nil
		},
		Action: a.run,
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

func (a *App) run(ctx context.Context, cmd *cli.Command) error {
	sessionName := cmd.String(FlagSessionName)
	projectPath := cmd.String(FlagDir)
	llmConfig := a.getLLMConfig(cmd)

	opts := []smoke.OptFunc{
		smoke.WithConfig(a.config),
		smoke.WithDebug(cmd.Bool(FlagDebug)),
		smoke.WithProjectPath(projectPath),
		smoke.WithSessionInfo(sessionName, prompts.SystemJSON()),
		smoke.WithLLMConfig(llmConfig),
	}

	mcpClients, err := a.getMCPClients(ctx, projectPath)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP clients: %w", err)
	}

	for _, client := range mcpClients {
		opts = append(opts, smoke.WithMCPClient(ctx, client))
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

func (a *App) getLLMConfig(cmd *cli.Command) *llms.Config {
	provider := llms.LLMType(cmd.String(FlagProvider))
	modelFlag := cmd.String(FlagModel)

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

	return llmConfig
}

func (a *App) getMCPClients(ctx context.Context, projectPath string) ([]*mcp.CommandClient, error) {
	if a.config == nil || a.config.MCP == nil {
		return nil, nil
	}

	results := []*mcp.CommandClient{}

	for _, serverConfig := range a.config.MCP.Servers {
		// Don't initialize clients for servers the user has disabled
		if !serverConfig.Enabled {
			slog.Debug("MCP server is disabled", "name", serverConfig.Name)
			continue
		}

		opts := &mcp.CommandClientOpts{
			MCPServer: serverConfig,
			Directory: projectPath,
		}

		client, err := mcp.NewCommandClient(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to set up gopls MCP client: %w", err)
		}

		results = append(results, client)
	}

	return results, nil
}

func main() {
	app := &App{}

	if err := app.setup(); err != nil {
		panic(fmt.Errorf("error: %w", err))
	}
}
