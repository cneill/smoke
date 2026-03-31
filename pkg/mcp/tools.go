package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	fullName   string
	clientName string
	toolName   string
	session    *mcp.ClientSession
	underlying *mcp.Tool
	schema     *jsonschema.Schema
	params     tools.Params
}

type ToolOpts struct {
	MCPTool       *mcp.Tool
	MCPSession    *mcp.ClientSession
	MCPServerName string
}

func (t *ToolOpts) OK() error {
	switch {
	case t.MCPTool == nil:
		return fmt.Errorf("missing MCP tool")
	case t.MCPSession == nil:
		return fmt.Errorf("missing MCP session")
	case t.MCPServerName == "":
		return fmt.Errorf("missing MCP server name")
	}

	return nil
}

func NewTool(opts *ToolOpts) (*Tool, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error for MCP tool: %w", err)
	}

	rawSchema, err := json.Marshal(opts.MCPTool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("invalid tool JSON schema for tool %q: %w", opts.MCPTool.Name, err)
	}

	schema := &jsonschema.Schema{}
	if err := json.Unmarshal(rawSchema, schema); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}

	tool := &Tool{
		fullName:   opts.MCPServerName + "_" + opts.MCPTool.Name,
		clientName: opts.MCPServerName,
		toolName:   opts.MCPTool.Name,
		session:    opts.MCPSession,
		underlying: opts.MCPTool,
		schema:     schema,
		params:     paramsFromSchema(schema),
	}

	return tool, nil
}

func (t *Tool) Name() string             { return t.fullName }
func (t *Tool) Description() string      { return t.underlying.Description }
func (t *Tool) Examples() tools.Examples { return nil }
func (t *Tool) Params() tools.Params     { return t.params }
func (t *Tool) Source() string           { return t.clientName }

func (t *Tool) Run(ctx context.Context, args tools.Args) (*tools.Output, error) {
	params := &mcp.CallToolParams{
		Name:      t.toolName,
		Arguments: args,
	}

	result, err := t.session.CallTool(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP tool %q: %w", t.Name(), err)
	}

	sb := &strings.Builder{}

	for _, content := range result.Content {
		switch content := content.(type) {
		case *mcp.TextContent:
			if _, err := sb.WriteString(content.Text); err != nil {
				return nil, fmt.Errorf("failed to construct output: %w", err)
			}
		default:
			continue
		}
	}

	return &tools.Output{Text: sb.String()}, nil
}

func (t *Tool) Schema() *jsonschema.Schema {
	return t.schema
}

func paramsFromSchema(schema *jsonschema.Schema) tools.Params {
	results := make(tools.Params, len(schema.Properties))
	idx := 0

	for name, property := range schema.Properties {
		param := tools.Param{
			Key:         name,
			Description: property.Description,
			Type:        tools.ParamType(property.Type), // TODO: match against "enum"?
		}

		if slices.Contains(schema.Required, name) {
			param.Required = true
		}

		if items := property.Items; items != nil {
			if items.Type != "" {
				param.ItemType = tools.ParamType(items.Type) // TODO: match against "enum"?
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

		if param.Type == tools.ParamTypeObject {
			param.NestedParams = paramsFromSchema(property)
		}

		// TODO: handle array of objects

		results[idx] = param

		idx++
	}

	return results
}
