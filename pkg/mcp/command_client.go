package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CommandClientOpts struct {
	*config.MCPServer

	Directory string
}

func (c *CommandClientOpts) OK() error {
	switch {
	case c.Name == "":
		return fmt.Errorf("missing name")
	case c.Directory == "":
		return fmt.Errorf("missing directory")
	case c.Command == "":
		return fmt.Errorf("missing command")
	}

	return nil
}

type CommandClient struct {
	name    string
	logger  *slog.Logger
	session *mcp.ClientSession
}

func NewCommandClient(ctx context.Context, opts *CommandClientOpts) (*CommandClient, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error for MCP client: %w", err)
	}

	impl := &mcp.Implementation{
		Name:    "smoke",
		Title:   "Smoke",
		Version: "v0.0.1",
	}

	mcpClient := mcp.NewClient(impl, &mcp.ClientOptions{})

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...) //nolint:gosec
	cmd.Dir = opts.Directory

	logger := slog.Default().WithGroup("mcp_" + opts.Name)
	writer := &logHandler{logger: logger}

	transport := &mcp.LoggingTransport{
		Transport: &mcp.CommandTransport{
			Command: cmd,
		},
		Writer: writer,
	}

	sessionOpts := &mcp.ClientSessionOptions{}

	session, err := mcpClient.Connect(ctx, transport, sessionOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to start MCP session: %w", err)
	}

	client := &CommandClient{
		name:    opts.Name,
		logger:  logger,
		session: session,
	}

	return client, nil
}

func (c *CommandClient) Name() string { return c.name }

func (c *CommandClient) Tools(ctx context.Context) (tools.Tools, error) {
	results := tools.Tools{}

	for tool, err := range c.session.Tools(ctx, &mcp.ListToolsParams{}) {
		if err != nil {
			return nil, fmt.Errorf("error with tool: %w", err)
		}

		mcpTool := &Tool{
			name:       c.name + "_" + tool.Name,
			clientName: c.name,
			session:    c.session,
			underlying: tool,
			params:     paramsFromSchema(tool.InputSchema),
		}

		results = append(results, mcpTool)
	}

	return results, nil
}

func (c *CommandClient) Close() error {
	if err := c.session.Close(); err != nil {
		return fmt.Errorf("failed to close client session: %w", err)
	}

	return nil
}

type logHandler struct {
	logger *slog.Logger
}

func (l *logHandler) Write(data []byte) (int, error) {
	l.logger.Debug("mcp message", "message", string(data))

	return len(data), nil
}
