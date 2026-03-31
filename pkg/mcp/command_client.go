package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"

	"github.com/cneill/smoke/internal/version"
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
	opts    *CommandClientOpts
}

func NewCommandClient(ctx context.Context, opts *CommandClientOpts) (*CommandClient, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error for MCP client: %w", err)
	}

	impl := &mcp.Implementation{
		Name:    "smoke",
		Title:   "Smoke",
		Version: version.String(),
	}

	mcpClient := mcp.NewClient(impl, &mcp.ClientOptions{})

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...) //nolint:gosec
	cmd.Dir = opts.Directory

	// Provide the environment variables specified in our configuration file
	if len(opts.Env) > 0 {
		cmd.Env = []string{}
		for _, env := range opts.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Var, env.Value))
		}
	}

	// TODO: need this? make it better?
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
		opts:    opts,
	}

	return client, nil
}

func (c *CommandClient) Name() string { return c.name }

func (c *CommandClient) Tools(ctx context.Context) (tools.Tools, error) {
	results := tools.Tools{}

	for serverTool, err := range c.session.Tools(ctx, &mcp.ListToolsParams{}) {
		if err != nil {
			return nil, fmt.Errorf("error with tool: %w", err)
		}

		if !c.toolAllowed(serverTool.Name) {
			continue
		}

		clientTool, err := NewTool(&ToolOpts{
			MCPTool:       serverTool,
			MCPSession:    c.session,
			MCPServerName: c.name,
		})
		if err != nil {
			return nil, fmt.Errorf("invalid tool for server %q: %w", c.name, err)
		}

		results = append(results, clientTool)
	}

	return results, nil
}

func (c *CommandClient) PlanTools(ctx context.Context) (tools.Tools, error) {
	allTools, err := c.Tools(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving tools: %w", err)
	}

	// Allow all tools in planning mode by default if no patterns are specified. Otherwise, check supplied patterns.
	if len(c.opts.PlanTools) == 0 {
		return allTools, nil
	}

	filteredTools := make(tools.Tools, 0, len(allTools))
	for _, tool := range allTools {
		mcpTool := tool.(*Tool) //nolint:forcetypeassert
		for _, pattern := range c.opts.PlanTools {
			if match, _ := filepath.Match(pattern, mcpTool.toolName); match {
				filteredTools = append(filteredTools, tool)
				break
			}
		}
	}

	return filteredTools, nil
}

func (c *CommandClient) Close() error {
	if err := c.session.Close(); err != nil {
		return fmt.Errorf("failed to close client session: %w", err)
	}

	return nil
}

func (c *CommandClient) toolAllowed(toolName string) bool {
	// TODO: revisit this? fail open / closed?
	if c.opts == nil {
		return true
	}

	if len(c.opts.AllowedTools) > 0 {
		ok := false

		for _, pattern := range c.opts.AllowedTools {
			if match, _ := filepath.Match(pattern, toolName); match {
				ok = true
				break
			}
		}

		return ok
	}

	if len(c.opts.DeniedTools) > 0 {
		ok := true

		for _, denied := range c.opts.DeniedTools {
			if match, _ := filepath.Match(denied, toolName); match {
				ok = false
				break
			}
		}

		return ok
	}

	// TODO: revisit this? fail open / closed?
	return true
}

type logHandler struct {
	logger *slog.Logger
}

func (l *logHandler) Write(data []byte) (int, error) {
	l.logger.Debug("mcp message", "message", string(data))

	return len(data), nil
}
