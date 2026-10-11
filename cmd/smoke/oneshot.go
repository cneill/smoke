package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cneill/smoke/internal/log"
	"github.com/cneill/smoke/internal/markdown"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

const defaultOutputWidth = 80

// readPrompt reads the whole one-shot prompt from r and returns it trimmed of surrounding whitespace.
func readPrompt(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt: %w", err)
	}

	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", errors.New("prompt read from stdin is empty")
	}

	return prompt, nil
}

// runOneShot answers a single prompt piped via stdin with a read-only tool set and prints the final answer to stdout.
func runOneShot(ctx context.Context, cmd *cli.Command) error {
	prompt, err := readPrompt(os.Stdin)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOneShot, err)
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cmd.Bool(FlagDebug) {
		logFile, err := setupLogFile(cmd)
		if err != nil {
			return fmt.Errorf("%w: failed to set up log file: %w", ErrOneShot, err)
		}

		defer func() {
			if err := logFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to close log file: %v\n", err)
			}
		}()
	} else {
		log.Setup(io.Discard, slog.LevelInfo)
	}

	smokeInstance, err := getSmokeInstance(ctx, cmd, smoke.WithOneShot())
	if err != nil {
		return fmt.Errorf("%w: failed to set up smoke controller: %w", ErrOneShot, err)
	}

	req := smoke.OneShotRequest{
		Prompt:   prompt,
		MaxTurns: cmd.Int(FlagMaxTurns),
	}

	if term.IsTerminal(int(os.Stderr.Fd())) {
		req.OnToolCall = func(toolCall llms.ToolCall) {
			fmt.Fprintf(os.Stderr, "-> %s %s\n", toolCall.Name, toolCall.ArgsString())
		}
	}

	final, err := smokeInstance.RunOneShot(ctx, req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOneShot, err)
	}

	output := renderOutput(final.TextContent())
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	if _, err := fmt.Fprint(os.Stdout, output); err != nil {
		return fmt.Errorf("%w: failed to write output: %w", ErrOneShot, err)
	}

	return nil
}

// renderOutput renders Markdown for a terminal on stdout, or returns the raw text when stdout is redirected.
func renderOutput(text string) string {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return text
	}

	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		width = defaultOutputWidth
	}

	renderer, err := markdown.NewTermRenderer(width)
	if err != nil {
		return text
	}

	rendered, err := renderer.Render(text)
	if err != nil {
		return text
	}

	return rendered
}
