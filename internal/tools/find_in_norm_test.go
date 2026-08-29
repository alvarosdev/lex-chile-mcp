package tools

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// FindInNormSuite validates the find_in_norm tool against a MockLawClient:
// the BCN API is never reached.
type FindInNormSuite struct {
	suite.Suite
	ctx       context.Context
	lawClient *bcn.MockLawClient
	session   *mcp.ClientSession
}

func TestFindInNormSuite(t *testing.T) {
	suite.Run(t, new(FindInNormSuite))
}

func (s *FindInNormSuite) SetupTest() {
	s.ctx = context.Background()
	s.lawClient = bcn.NewMockLawClient(s.T())
	s.session = newTestClient(s.T(), s.ctx, s.lawClient)
}

func (s *FindInNormSuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{
		Name:      "find_in_norm",
		Arguments: args,
	})
}

func (s *FindInNormSuite) sampleResult() bcn.NormaSearchResult {
	return bcn.NormaSearchResult{
		Walked: 205,
		Matches: []bcn.NormaMatch{
			{Name: "Artículo 1749 (DEL ART. 2)", SectionID: 8721234, CharCount: 1180, ArticleCount: 1},
			{Name: "Título XXVIII DE LA SOCIEDAD CONYUGAL", SectionID: 8721100, CharCount: 32400, ArticleCount: 88,
				Snippet: "…la sociedad conyugal se compone de los bienes…"},
		},
	}
}

func (s *FindInNormSuite) TestFindInNormHappyPath() {
	s.lawClient.EXPECT().SearchNorma(mock.Anything, bcn.NormaQuery{NormID: 172986}, "1749").
		Return(s.sampleResult(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 172986, "query": "1749"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, `Matches for "1749" in norm_id 172986 (205 sections walked)`)
	// Name match line: identifier + size, no snippet.
	s.Contains(text, "- Artículo 1749 (DEL ART. 2) · section_id: 8721234 · 1.2K chars · 1 article")
	// Content match carries an indented snippet.
	s.Contains(text, "- Título XXVIII DE LA SOCIEDAD CONYUGAL · section_id: 8721100 · 32K chars · 88 articles")
	s.Contains(text, "> …la sociedad conyugal se compone de los bienes…")

	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok, "structuredContent expected, got %T", res.StructuredContent)
	results := sc["results"].([]any)
	s.Require().Len(results, 2)
	first := results[0].(map[string]any)
	s.Equal("Artículo 1749 (DEL ART. 2)", first["name"])
	s.Equal(float64(8721234), first["section_id"])
	_, hasSnippet := first["snippet"]
	s.False(hasSnippet, "name matches carry no snippet")
	second := results[1].(map[string]any)
	s.NotEmpty(second["snippet"])
	s.Equal(float64(2), sc["matched_total"])
}

func (s *FindInNormSuite) TestFindInNormArgumentErrorsFailWithoutCallingClient() {
	// No expectations on the mock: the handler must not call SearchNorma.
	cases := []map[string]any{
		{"query": "1749"},               // missing norm_id
		{"norm_id": 0, "query": "1749"}, // non-positive norm_id
		{"norm_id": 42},                 // missing query
		{"norm_id": 42, "query": "   "}, // whitespace-only query
		{"norm_id": 42, "query": "x", "version_date": "2010-13-45"},
		{"norm_id": 42, "query": "x", "version_date": "basura"},
	}
	for _, args := range cases {
		res, err := s.callTool(args)
		s.Require().NoError(err)
		s.True(res.IsError, "args %v must fail", args)
	}
}

func (s *FindInNormSuite) TestFindInNormNotFoundSurfacesClearMessage() {
	s.lawClient.EXPECT().SearchNorma(mock.Anything, bcn.NormaQuery{NormID: 999999999}, "1749").
		Return(bcn.NormaSearchResult{}, bcn.ErrNormaNotFound).Once()

	res, err := s.callTool(map[string]any{"norm_id": 999999999, "query": "1749"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "norma not found: norm_id 999999999")
}

func (s *FindInNormSuite) TestFindInNormGenericErrorSurfaces() {
	s.lawClient.EXPECT().SearchNorma(mock.Anything, bcn.NormaQuery{NormID: 1}, "x").
		Return(bcn.NormaSearchResult{}, errors.New("circuit breaker open")).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1, "query": "x"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "find in norm failed")
}

func (s *FindInNormSuite) TestFindInNormNoMatchesReportsWalkedSections() {
	s.lawClient.EXPECT().SearchNorma(mock.Anything, bcn.NormaQuery{NormID: 42}, "inexistente").
		Return(bcn.NormaSearchResult{Walked: 205}, nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 42, "query": "inexistente"})
	s.Require().NoError(err)
	s.False(res.IsError, "no matches is a valid answer, not an error")
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, `No matches for "inexistente"`)
	s.Contains(text, "walked 205 sections")

	sc := res.StructuredContent.(map[string]any)
	s.Empty(sc["results"])
	s.Equal(float64(0), sc["matched_total"])
}

func (s *FindInNormSuite) TestFindInNormCapsResultsWithSignal() {
	result := bcn.NormaSearchResult{Walked: 300}
	for i := range 25 {
		result.Matches = append(result.Matches, bcn.NormaMatch{
			Name: fmt.Sprintf("Artículo %d", i+1), SectionID: int64(1000 + i),
			CharCount: 100, ArticleCount: 1,
		})
	}
	s.lawClient.EXPECT().SearchNorma(mock.Anything, bcn.NormaQuery{NormID: 42}, "artículo").
		Return(result, nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 42, "query": "artículo"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "- Artículo 1 · section_id: 1000")
	s.Contains(text, "- Artículo 20 · section_id: 1019")
	s.NotContains(text, "- Artículo 21 ·", "only the first 20 are listed")
	s.Contains(text, "Matched 25 sections — showing first 20; narrow the query")

	sc := res.StructuredContent.(map[string]any)
	results := sc["results"].([]any)
	s.Len(results, 20)
	s.Equal(float64(25), sc["matched_total"], "structured carries the REAL total")
}

func (s *FindInNormSuite) TestFindInNormWithVersionDate() {
	s.lawClient.EXPECT().SearchNorma(mock.Anything, bcn.NormaQuery{NormID: 141599, VersionDate: "2010-01-01"}, "plazo").
		Return(s.sampleResult(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 141599, "query": "plazo", "version_date": "2010-01-01"})
	s.Require().NoError(err)
	s.False(res.IsError)
}

func (s *FindInNormSuite) TestFindInNormRegistered() {
	list, err := s.session.ListTools(s.ctx, nil)
	s.Require().NoError(err)
	for _, t := range list.Tools {
		if t.Name == "find_in_norm" {
			s.Contains(t.Description, "search_laws")
			s.Contains(t.Description, "get_law(section_id")
			return
		}
	}
	s.Fail("find_in_norm not registered")
}
