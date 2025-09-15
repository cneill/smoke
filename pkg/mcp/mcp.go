package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Client struct {
	logger  *slog.Logger
	session *mcp.ClientSession
}

func NewClient(ctx context.Context, directory string) (*Client, error) {
	impl := &mcp.Implementation{
		Name:    "smoke",
		Title:   "Smoke",
		Version: "v0.0.1",
	}

	opts := &mcp.ClientOptions{}
	mcpClient := mcp.NewClient(impl, opts)

	// TODO: make this customizable
	cmd := exec.CommandContext(ctx, "gopls", "mcp")
	cmd.Dir = directory

	logger := slog.Default().WithGroup("mcp")
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

	client := &Client{
		logger:  logger,
		session: session,
	}

	return client, nil
}

func (c *Client) Tools(ctx context.Context) (tools.Tools, error) {
	results := tools.Tools{}

	for tool, err := range c.session.Tools(ctx, &mcp.ListToolsParams{}) {
		if err != nil {
			return nil, fmt.Errorf("error with tool: %w", err)
		}

		// fmt.Printf("%s\n----\n%s\n", tool.Name, tool.Description)
		results = append(results, toTool(c.session, tool))
	}

	return results, nil
}

func (c *Client) Close() error {
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
