package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/log"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/models/ui"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/urfave/cli/v3"
)

type App struct {
	config *config.Config
}

func (a *App) validate(ctx *cli.Command) error {
	llmType := llms.LLMType(ctx.String(FlagProvider))

	details := providerDetailMappings(llmType)
	if details == nil {
		return fmt.Errorf("unknown model provider")
	}

	if ctx.String(details.flag) == "" {
		return fmt.Errorf("must supply %s flag or %s environment variable", details.flag, details.envVar)
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
	details := providerDetailMappings(provider)

	llmConfig := &llms.Config{
		APIKey:      cmd.String(details.flag),
		MaxTokens:   cmd.Int64(FlagMaxTokens),
		Provider:    provider,
		Temperature: cmd.Float64(FlagTemperature),
		Model:       details.getModel(cmd.String(FlagModel)),
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
