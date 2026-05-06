package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/log"
	"github.com/cneill/smoke/internal/version"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/models/ui"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/urfave/cli/v3"
)

var ErrInit = errors.New("initialization")

func validate(cmd *cli.Command) error {
	provider := strings.ToLower(cmd.String(FlagProvider))

	details, err := getProviders().details(provider)
	if err != nil {
		return err
	}

	if details.apiKeyFlag != "" && cmd.String(details.apiKeyFlag) == "" {
		return fmt.Errorf("must supply --%s flag or $%s environment variable", details.apiKeyFlag, details.apiKeyEnvVar)
	}

	args := cmd.Args()
	if args.Len() > 1 {
		return fmt.Errorf("must supply one project directory as an argument (or none for CWD)")
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
	provider := strings.ToLower(cmd.String(FlagProvider))

	details, err := getProviders().details(provider)
	if err != nil {
		return nil, err
	}

	model, err := details.getModel(cmd.String(FlagModel))
	if err != nil {
		return nil, fmt.Errorf("failed to select model for provider %q: %w", provider, err)
	}

	llmConfig := &llms.Config{
		APIKey:      cmd.String(details.apiKeyFlag),
		BaseURL:     cmd.String(details.baseURLFlag),
		MaxTokens:   cmd.Int64(FlagMaxTokens),
		Provider:    llms.LLMType(provider),
		Temperature: cmd.Float64(FlagTemperature),
		Model:       model,
	}

	return llmConfig, nil
}

func getSmokeInstance(ctx context.Context, cmd *cli.Command) (*smoke.Smoke, error) {
	args := cmd.Args()

	projectPath := args.First()
	if args.Len() == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}

		projectPath = cwd
	}

	sessionName := cmd.String(FlagSessionName)

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
		smoke.WithSessionName(sessionName),
		smoke.WithLLMConfig(llmConfig),
	}

	smokeInstance, err := smoke.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to set up smoke: %w", err)
	}

	return smokeInstance, nil
}

func run(ctx context.Context, cmd *cli.Command) error {
	if err := validate(cmd); err != nil {
		return fmt.Errorf("%w: flag validation error: %w", ErrInit, err)
	}

	logFile, err := setupLogFile(cmd)
	if err != nil {
		return fmt.Errorf("%w: failed to set up log file: %w", ErrInit, err)
	}

	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Printf("failed to close log file: %v\n", err)
		}
	}()

	smokeInstance, err := getSmokeInstance(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%w: failed to set up smoke controller: %w", ErrInit, err)
	}

	// Run the Bubbletea loop
	uiOpts := &ui.Opts{
		Smoke: smokeInstance,
	}

	uiModel, err := ui.New(uiOpts)
	if err != nil {
		return fmt.Errorf("%w: failed to set up UI: %w", ErrInit, err)
	}

	program := tea.NewProgram(uiModel, tea.WithReportFocus(), tea.WithMouseCellMotion())

	// Give Smoke the ability to send messages directly into the bubbletea event loop.
	if _, err := smokeInstance.Update(ctx, smoke.WithTeaEmitter(program.Send)); err != nil {
		return fmt.Errorf("%w: failed to update smoke controller with bubbletea emitter: %w", ErrInit, err)
	}

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

func main() {
	command := &cli.Command{
		Name: "smoke",
		Description: "An agentic coding assistant for the Go programming language that operates within a single git " +
			"repository.",
		Usage:     "A coding assistant for Gophers",
		UsageText: "smoke [global options] [project path]",
		Flags:     flags(),
		Action:    run,
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return fmt.Errorf("%w: %w", ErrInit, err)
		},
		Version: version.String(),
	}

	err := command.Run(context.TODO(), os.Args)
	if err != nil {
		if errors.Is(err, ErrInit) {
			if helpErr := cli.ShowAppHelp(command); helpErr != nil {
				fmt.Printf("error: %v\n", helpErr)
				os.Exit(1)
			}

			fmt.Printf("error: %v\n", err)
			os.Exit(1)
		}

		panic(fmt.Errorf("error: %w", err))
	}
}
