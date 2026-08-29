package tools

import (
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTool wraps mcp.AddTool, generating the input and output schemas
// with the SDK's own generator (github.com/google/jsonschema-go, the same
// one mcp.AddTool uses internally) and post-processing them for MCP client
// portability before registration — see sanitizeSchemaPortability.
//
// Panics on schema generation errors, matching mcp.AddTool's own behavior
// for schema failures at registration time.
func registerTool[In, Out any](srv *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	input, err := schemaFor[In]()
	if err != nil {
		panic(fmt.Sprintf("registerTool %q: input schema: %v", tool.Name, err))
	}
	tool.InputSchema = input
	output, err := schemaFor[Out]()
	if err != nil {
		panic(fmt.Sprintf("registerTool %q: output schema: %v", tool.Name, err))
	}
	tool.OutputSchema = output
	mcp.AddTool(srv, tool, handler)
}

// schemaFor generates the JSON Schema for T and sanitizes it.
func schemaFor[T any]() (*jsonschema.Schema, error) {
	s, err := jsonschema.ForType(reflect.TypeFor[T](), &jsonschema.ForOptions{})
	if err != nil {
		return nil, err
	}
	sanitizeSchemaPortability(s)
	return s, nil
}

// sanitizeSchemaPortability rewrites multi-type schemas — `"type":
// ["null", "array"]`, legal JSON Schema but read as a single string by
// several MCP clients, which then reject the tool or drop the constraint —
// into `anyOf` branches, each carrying a single `type`. Semantics are
// preserved exactly: `null` means the Go zero (nil slice), NOT an absent
// property, so this is not the same as making the property optional.
// Sibling keywords (description, items…) stay on the node.
func sanitizeSchemaPortability(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Types) > 1 {
		branches := make([]*jsonschema.Schema, 0, len(s.Types))
		for _, t := range s.Types {
			branches = append(branches, &jsonschema.Schema{Type: t})
		}
		s.Types = nil
		s.Type = ""
		s.AnyOf = branches
	}
	for _, p := range s.Properties {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.PatternProperties {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.Defs {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.Definitions {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.DependencySchemas {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.PrefixItems {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.ItemsArray {
		sanitizeSchemaPortability(p)
	}
	sanitizeSchemaPortability(s.Items)
	sanitizeSchemaPortability(s.AdditionalProperties)
	sanitizeSchemaPortability(s.AdditionalItems)
	sanitizeSchemaPortability(s.Contains)
	sanitizeSchemaPortability(s.PropertyNames)
	sanitizeSchemaPortability(s.UnevaluatedItems)
	sanitizeSchemaPortability(s.UnevaluatedProperties)
	sanitizeSchemaPortability(s.Not)
	for _, p := range s.AllOf {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.AnyOf {
		sanitizeSchemaPortability(p)
	}
	for _, p := range s.OneOf {
		sanitizeSchemaPortability(p)
	}
	sanitizeSchemaPortability(s.If)
	sanitizeSchemaPortability(s.Then)
	sanitizeSchemaPortability(s.Else)
}
