package mcp

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	session    *mcp.ClientSession
	underlying *mcp.Tool
	params     tools.Params
}

func toTool(mcpSession *mcp.ClientSession, mcpTool *mcp.Tool) *Tool {
	return &Tool{
		session:    mcpSession,
		underlying: mcpTool,
		params:     paramsFromSchema(mcpTool.InputSchema),
	}
}

func (t *Tool) Name() string             { return t.underlying.Name }
func (t *Tool) Description() string      { return t.underlying.Description }
func (t *Tool) Examples() tools.Examples { return nil }
func (t *Tool) Params() tools.Params     { return t.params }

func (t *Tool) Run(ctx context.Context, args tools.Args) (string, error) {
	params := &mcp.CallToolParams{
		Name:      t.Name(),
		Arguments: args,
	}

	result, err := t.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to call MCP tool %q: %w", t.Name(), err)
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

func (t *Tool) Schema() *jsonschema.Schema {
	return t.underlying.InputSchema
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
