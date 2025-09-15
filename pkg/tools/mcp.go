package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPClient struct {
	logger  *slog.Logger
	session *mcp.ClientSession
}

func NewMCPClient(ctx context.Context, directory string) (*MCPClient, error) {
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

	client := &MCPClient{
		logger:  logger,
		session: session,
	}

	return client, nil
}

func (m *MCPClient) Tools(ctx context.Context) (Tools, error) {
	results := Tools{}

	for tool, err := range m.session.Tools(ctx, &mcp.ListToolsParams{}) {
		if err != nil {
			return nil, fmt.Errorf("error with tool: %w", err)
		}

		// fmt.Printf("%s\n----\n%s\n", tool.Name, tool.Description)
		results = append(results, toTool(m.session, tool))
	}

	return results, nil
}

func (m *MCPClient) Close() error {
	if err := m.session.Close(); err != nil {
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

type MCPTool struct {
	session    *mcp.ClientSession
	underlying *mcp.Tool
	params     Params
}

func toTool(mcpSession *mcp.ClientSession, mcpTool *mcp.Tool) *MCPTool {
	return &MCPTool{
		session:    mcpSession,
		underlying: mcpTool,
		params:     paramsFromSchema(mcpTool.InputSchema),
	}
}

func (m *MCPTool) Name() string        { return m.underlying.Name }
func (m *MCPTool) Description() string { return m.underlying.Description }
func (m *MCPTool) Examples() Examples  { return nil }
func (m *MCPTool) Params() Params      { return m.params }

func (m *MCPTool) Run(ctx context.Context, args Args) (string, error) {
	params := &mcp.CallToolParams{
		Name:      m.Name(),
		Arguments: args,
	}

	result, err := m.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to call MCP tool %q: %w", m.Name(), err)
	}

	sb := &strings.Builder{}

	for _, content := range result.Content {
		switch content := content.(type) {
		case *mcp.TextContent:
			if _, err := sb.WriteString(content.Text); err != nil {
				return "", fmt.Errorf("failed to construct output: %w", err)
			}
		default:
			continue
		}
	}

	return sb.String(), nil
}

func (m *MCPTool) Schema() *jsonschema.Schema {
	return m.underlying.InputSchema
}

func paramsFromSchema(schema *jsonschema.Schema) Params {
	results := make(Params, len(schema.Properties))
	idx := 0

	for name, property := range schema.Properties {
		param := Param{
			Key:         name,
			Description: property.Description,
			Type:        ParamType(property.Type), // TODO: match against "enum"?
		}

		if slices.Contains(schema.Required, name) {
			param.Required = true
		}

		if items := property.Items; items != nil {
			if items.Type != "" {
				param.ItemType = ParamType(items.Type) // TODO: match against "enum"?
			}
		}

		if enum := property.Enum; len(enum) > 0 {
			allStrings := true
			strVals := make([]string, len(enum))

			for i, enumVal := range enum {
				if enumStr, ok := enumVal.(string); ok {
					strVals[i] = enumStr
				} else {
					allStrings = false
					break
				}
			}

			if allStrings {
				param.EnumStringValues = strVals
			}
		}

		if param.Type == ParamTypeObject {
			param.NestedParams = paramsFromSchema(property)
		}

		// TODO: handle array of objects

		results[idx] = param

		idx++
	}

	return results
}
