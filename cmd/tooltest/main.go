package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cneill/smoke/pkg/log"
	"github.com/cneill/smoke/pkg/tools"
)

func run() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: %s [project_path] [tool_name] [args]", os.Args[0])
	}

	log.Setup(os.Stderr, slog.LevelDebug)

	absPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		return fmt.Errorf("path error: %w", err)
	}

	toolManager := tools.NewManager(absPath, "test_session")
	toolName := os.Args[2]
	rawArgs := os.Args[3]

	params, err := toolManager.Tools.Params(toolName)
	if err != nil {
		return fmt.Errorf("failed to get params: %w", err)
	}

	args, err := tools.GetArgs([]byte(rawArgs), params)
	if err != nil {
		return fmt.Errorf("failed to get args from input %q: %w", rawArgs, err)
	}

	output, err := toolManager.CallTool(toolName, args)
	if err != nil {
		return fmt.Errorf("failed to call tool: %w", err)
	}

	fmt.Println(output)

	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
