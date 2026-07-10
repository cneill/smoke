package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cneill/smoke/pkg/config"
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
	MCPTool         *mcp.Tool
	MCPSession      *mcp.ClientSession
	MCPServerConfig *config.MCPServer
}

func (t *ToolOpts) OK() error {
	switch {
	case t.MCPTool == nil:
		return fmt.Errorf("missing MCP tool")
	case t.MCPSession == nil:
		return fmt.Errorf("missing MCP session")
	case t.MCPServerConfig == nil:
		return fmt.Errorf("missing MCP server config")
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

	fullName := opts.MCPServerConfig.Name + "_" + opts.MCPTool.Name
	if opts.MCPServerConfig.NoNamespace {
		fullName = opts.MCPTool.Name
	}

	params, err := paramsFromRawSchema(rawSchema)
	if err != nil {
		return nil, fmt.Errorf("invalid tool params schema for tool %q: %w", opts.MCPTool.Name, err)
	}

	tool := &Tool{
		fullName:   fullName,
		clientName: opts.MCPServerConfig.Name,
		toolName:   opts.MCPTool.Name,
		session:    opts.MCPSession,
		underlying: opts.MCPTool,
		schema:     schema,
		params:     params,
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

type rawParamSchema struct {
	Type        any                        `json:"type"`
	Description string                     `json:"description"`
	Items       *rawParamSchema            `json:"items"`
	Enum        []any                      `json:"enum"`
	Properties  map[string]json.RawMessage `json:"properties"`
	Required    []string                   `json:"required"`
}

func paramsFromRawSchema(rawSchema []byte) (tools.Params, error) {
	schema := rawParamSchema{}
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw JSON schema: %w", err)
	}

	return paramsFromRawParamSchema(schema)
}

func paramsFromRawParamSchema(schema rawParamSchema) (tools.Params, error) {
	results := make(tools.Params, 0, len(schema.Properties))

	propertyNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propertyNames = append(propertyNames, name)
	}

	slices.Sort(propertyNames)

	for _, name := range propertyNames {
		property := rawParamSchema{}
		if err := json.Unmarshal(schema.Properties[name], &property); err != nil {
			return nil, fmt.Errorf("failed to unmarshal property %q: %w", name, err)
		}

		param, err := paramFromRawProperty(name, property, slices.Contains(schema.Required, name))
		if err != nil {
			return nil, err
		}

		results = append(results, param)
	}

	return results, nil
}

func paramFromRawProperty(name string, property rawParamSchema, required bool) (tools.Param, error) {
	paramType, nullable := paramTypeFromSchemaType(property.Type)
	param := tools.Param{
		Key:              name,
		Description:      property.Description,
		Type:             paramType,
		Nullable:         nullable,
		Required:         required,
		EnumStringValues: enumStringValues(property.Enum),
	}

	if items := property.Items; items != nil {
		itemType, _ := paramTypeFromSchemaType(items.Type)
		param.ItemType = itemType
	}

	if err := setNestedParams(&param, property); err != nil {
		return tools.Param{}, err
	}

	return param, nil
}

func enumStringValues(enum []any) []string {
	if len(enum) == 0 {
		return nil
	}

	strVals := make([]string, len(enum))
	for enumIdx, enumVal := range enum {
		enumStr, ok := enumVal.(string)
		if !ok {
			return nil
		}

		strVals[enumIdx] = enumStr
	}

	return strVals
}

func setNestedParams(param *tools.Param, property rawParamSchema) error {
	switch {
	case param.Type == tools.ParamTypeObject:
		nestedParams, err := paramsFromRawParamSchema(property)
		if err != nil {
			return fmt.Errorf("failed to parse nested object params for %q: %w", param.Key, err)
		}

		param.NestedParams = nestedParams
	case param.Type == tools.ParamTypeArray && param.ItemType == tools.ParamTypeObject && property.Items != nil:
		nestedParams, err := paramsFromRawParamSchema(*property.Items)
		if err != nil {
			return fmt.Errorf("failed to parse nested array object params for %q: %w", param.Key, err)
		}

		param.NestedParams = nestedParams
	}

	return nil
}

func paramTypeFromSchemaType(schemaType any) (tools.ParamType, bool) {
	switch schemaType := schemaType.(type) {
	case string:
		return tools.ParamType(schemaType), false
	case []any:
		return paramTypeFromSchemaTypeList(schemaType)
	case []string:
		values := make([]any, len(schemaType))
		for i, value := range schemaType {
			values[i] = value
		}

		return paramTypeFromSchemaTypeList(values)
	default:
		return "", false
	}
}

func paramTypeFromSchemaTypeList(schemaTypes []any) (tools.ParamType, bool) {
	nonNullTypes := []tools.ParamType{}
	nullable := false

	for _, schemaType := range schemaTypes {
		strType, ok := schemaType.(string)
		if !ok {
			continue
		}

		paramType := tools.ParamType(strType)
		if paramType == tools.ParamTypeNull {
			nullable = true
			continue
		}

		nonNullTypes = append(nonNullTypes, paramType)
	}

	if len(nonNullTypes) != 1 {
		return "", nullable
	}

	return nonNullTypes[0], nullable
}
