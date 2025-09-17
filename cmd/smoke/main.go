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

func setup() *cli.Command {
	return &cli.Command{
		Name: "smoke",
		Description: "An agentic coding assistant primarily focused on the Go programming language. It only works on " +
			"one directory at a time, and requires that directory to contain a .git subdirectory.",
		Usage:   "Smoke 'em if you got 'em.",
		Flags:   flags(),
		Action:  run,
		Version: "v0.0.1", // TODO: dynamic
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	if err := validate(cmd); err != nil {
		return fmt.Errorf("flag validation error: %w", err)
	}

	logFile, err := setupLogFile(cmd)
	if err != nil {
		return fmt.Errorf("failed to set up log file: %w", err)
	}

	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Printf("failed to close log file: %v\n", err)
		}
	}()

	smokeInstance, err := getSmokeInstance(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to set up smoke controller: %w", err)
	}

	// Run the Bubbletea loop
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

func validate(cmd *cli.Command) error {
	provider := cmd.String(FlagProvider)

	details, err := getProviders().details(provider)
	if err != nil {
		return err
	}

	if cmd.String(details.flag) == "" {
		return fmt.Errorf("must supply --%s flag or $%s environment variable", details.flag, details.envVar)
	}

	return nil
}

func setupLogFile(cmd *cli.Command) (*os.File, error) {
	level := slog.LevelInfo
	if cmd.Bool(FlagDebug) {
		level = slog.LevelDebug
	}

	var (
		logFileName = cmd.String(FlagSessionName) + "_log.log"
		err         error
	)

	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	log.Setup(logFile, level)

	return logFile, nil
}

func getLLMConfig(cmd *cli.Command) (*llms.Config, error) {
	provider := cmd.String(FlagProvider)

	details, err := getProviders().details(provider)
	if err != nil {
		return nil, err
	}

	llmConfig := &llms.Config{
		APIKey:      cmd.String(details.flag),
		MaxTokens:   cmd.Int64(FlagMaxTokens),
		Provider:    llms.LLMType(provider),
		Temperature: cmd.Float64(FlagTemperature),
		Model:       details.getModel(cmd.String(FlagModel)),
	}

	return llmConfig, nil
}

func getMCPClients(ctx context.Context, projectPath string, mcpConfigs *config.MCP) ([]*mcp.CommandClient, error) {
	if mcpConfigs == nil {
		return nil, nil
	}

	results := []*mcp.CommandClient{}

	for _, serverConfig := range mcpConfigs.Servers {
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
			return nil, fmt.Errorf("failed to set up MCP client %q: %w", opts.Name, err)
		}

		results = append(results, client)
	}

	return results, nil
}

func getSmokeInstance(ctx context.Context, cmd *cli.Command) (*smoke.Smoke, error) {
	sessionName := cmd.String(FlagSessionName)
	projectPath := cmd.String(FlagDir)

	loadedConfig, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	llmConfig, err := getLLMConfig(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to set up LLM configuration: %w", err)
	}

	opts := []smoke.OptFunc{
		smoke.WithConfig(loadedConfig),
		smoke.WithDebug(cmd.Bool(FlagDebug)),
		smoke.WithProjectPath(projectPath),
		smoke.WithSessionInfo(sessionName, prompts.SystemJSON()),
		smoke.WithLLMConfig(llmConfig),
	}

	if loadedConfig.MCP != nil {
		mcpClients, err := getMCPClients(ctx, projectPath, loadedConfig.MCP)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize MCP clients: %w", err)
		}

		for _, client := range mcpClients {
			opts = append(opts, smoke.WithMCPClient(ctx, client))
		}
	}

	smokeInstance, err := smoke.New(opts...) //nolint:contextcheck
	if err != nil {
		return nil, fmt.Errorf("failed to set up smoke: %w", err)
	}

	return smokeInstance, nil
}

func main() {
	command := setup()

	if err := command.Run(context.TODO(), os.Args); err != nil {
		panic(fmt.Errorf("run error: %w", err))
	}
}
