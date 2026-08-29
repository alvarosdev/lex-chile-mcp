package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// SchemaPortabilitySuite validates the MCP client portability contract of
// every registered tool schema: no "type" keyword may be an ARRAY
// (["null","array"]) — several MCP clients read type as a single string
// and reject the tool or drop the constraint. The multi-type form must be
// rewritten into anyOf branches (same semantics: null ≠ absent).
type SchemaPortabilitySuite struct {
	suite.Suite
	ctx       context.Context
	lawClient *bcn.MockLawClient
	session   *mcp.ClientSession
}

func TestSchemaPortabilitySuite(t *testing.T) {
	suite.Run(t, new(SchemaPortabilitySuite))
}

func (s *SchemaPortabilitySuite) SetupTest() {
	s.ctx = context.Background()
	s.lawClient = bcn.NewMockLawClient(s.T())
	s.session = newTestClient(s.T(), s.ctx, s.lawClient)
}

// typeArrays walks a decoded JSON schema and reports every "type" keyword
// carried as an array, with its JSON path.
func typeArrays(node any, path string, report *[]string) {
	switch v := node.(type) {
	case map[string]any:
		if t, ok := v["type"].([]any); ok && len(t) > 1 {
			*report = append(*report, path+".type")
		}
		for key, child := range v {
			typeArrays(child, path+"."+key, report)
		}
	case []any:
		for _, child := range v {
			typeArrays(child, path, report)
		}
	}
}

func (s *SchemaPortabilitySuite) decode(raw any) any {
	data, err := json.Marshal(raw)
	s.Require().NoError(err)
	var schema any
	s.Require().NoError(json.Unmarshal(data, &schema))
	return schema
}

func (s *SchemaPortabilitySuite) TestEveryRegisteredSchemaAvoidsTypeArrays() {
	list, err := s.session.ListTools(s.ctx, nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(list.Tools)

	var report []string
	for _, tool := range list.Tools {
		for label, raw := range map[string]any{
			"inputSchema":  tool.InputSchema,
			"outputSchema": tool.OutputSchema,
		} {
			if raw == nil {
				continue
			}
			typeArrays(s.decode(raw), tool.Name+"/"+label, &report)
		}
	}
	s.Empty(report, "type arrays must be rewritten into anyOf branches in: %v", report)
}

func (s *SchemaPortabilitySuite) TestMultiTypeBecomesAnyOfBranches() {
	s.lawClient.EXPECT().GetNormaSummary(mock.Anything, bcn.NormaQuery{NormID: 1}).
		Return(bcn.NormaSummary{TituloNorma: "t", Materias: []string{"m"}}, nil).Once()

	res, err := s.session.CallTool(s.ctx, &mcp.CallToolParams{
		Name:      "get_law_summary",
		Arguments: map[string]any{"norm_id": 1},
	})
	s.Require().NoError(err)
	s.False(res.IsError)

	// The output schema travels with the tool list; take the summary's
	// schema from ListTools (not from the call result).
	list, err := s.session.ListTools(s.ctx, nil)
	s.Require().NoError(err)
	var summarySchema any
	for _, tool := range list.Tools {
		if tool.Name == "get_law_summary" {
			summarySchema = s.decode(tool.OutputSchema)
		}
	}
	s.Require().NotNil(summarySchema)
	sc := summarySchema.(map[string]any)
	props := sc["properties"].(map[string]any)

	materias := props["materias"].(map[string]any)
	_, isTypeArray := materias["type"].([]any)
	s.False(isTypeArray, "materias.type must not be an array")
	anyOf, ok := materias["anyOf"].([]any)
	s.Require().True(ok, "materias must carry anyOf branches, got %v", materias)
	s.Len(anyOf, 2)
	s.Equal("null", anyOf[0].(map[string]any)["type"])
	s.Equal("array", anyOf[1].(map[string]any)["type"])

	// Null stays a real value: the property is NOT made optional.
	required, ok := sc["required"].([]any)
	s.Require().True(ok)
	s.Contains(required, "materias")
}
