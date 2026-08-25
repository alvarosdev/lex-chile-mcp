package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// errGeneric is a shared test error for client failures.
var errGeneric = errors.New("upstream failure")

// SearchLawsSuite validates the search_laws tool against a MockLawClient:
// the BCN API is never reached.
type SearchLawsSuite struct {
	suite.Suite
	ctx       context.Context
	lawClient *bcn.MockLawClient
	session   *mcp.ClientSession
}

func TestSearchLawsSuite(t *testing.T) {
	suite.Run(t, new(SearchLawsSuite))
}

func (s *SearchLawsSuite) SetupTest() {
	s.ctx = context.Background()
	s.lawClient = bcn.NewMockLawClient(s.T())
	s.session = newTestClient(s.T(), s.ctx, s.lawClient)
}

func (s *SearchLawsSuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{
		Name:      "search_laws",
		Arguments: args,
	})
}

func (s *SearchLawsSuite) TestSearchSuccess() {
	s.lawClient.EXPECT().Search(mock.Anything, bcn.SearchParams{
		Query: "Ley 21.600", Page: 1, PageSize: 10,
	}).Return(bcn.SearchResponse{
		Results: []bcn.Norma{{
			IDNorma: 1195666, Norma: "Ley 21600", Tipo: "Ley",
			TituloNorma:      "CREA EL SERVICIO DE BIODIVERSIDAD",
			FechaPublicacion: "06-SEP-2023",
			Organismo:        "MINISTERIO DEL MEDIO AMBIENTE",
			Resumen:          "La presente ley crea el Servicio.",
		}},
		Pagination: bcn.Pagination{TotalItems: 140, Page: 1, PageSize: 10, Query: "Ley 21.600"},
	}, nil).Once()

	res, err := s.callTool(map[string]any{"query": "Ley 21.600"})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "page 1 of 14 (140 total results)")
	s.Contains(text, "Ley 21600 | Ley | norm_id: 1195666")
	s.Contains(text, "Summary: La presente ley crea el Servicio.")

	// Structured content: typed, complete data (the wire decodes into a map).
	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok, "structuredContent expected, got %T", res.StructuredContent)
	s.Equal(float64(140), sc["total_items"])
	s.Equal(float64(14), sc["total_pages"])
	s.Equal("Ley 21.600", sc["query"])
	results := sc["results"].([]any)
	s.Require().Len(results, 1)
	first := results[0].(map[string]any)
	s.Equal(float64(1195666), first["norm_id"])
	s.Equal("La presente ley crea el Servicio.", first["summary"])
}

func (s *SearchLawsSuite) TestSearchDefaultsPageAndPageSize() {
	s.lawClient.EXPECT().Search(mock.Anything, bcn.SearchParams{
		Query: "x", Page: 1, PageSize: 10,
	}).Return(bcn.SearchResponse{
		Pagination: bcn.Pagination{TotalItems: 0, Query: "x"},
	}, nil).Once()

	res, err := s.callTool(map[string]any{"query": "x", "page": 0, "page_size": 999})
	s.Require().NoError(err)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "0 total results")
}

func (s *SearchLawsSuite) TestSearchEmptyQueryFailsWithoutCallingClient() {
	// No expectations on the mock: the handler must not call Search.
	_, err := s.callTool(map[string]any{"query": "   "})
	s.Require().NoError(err)
	res, err := s.callTool(map[string]any{"query": ""})
	s.Require().NoError(err)
	s.True(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "query is required")
}

func (s *SearchLawsSuite) TestSearchClientErrorSurfaces() {
	s.lawClient.EXPECT().Search(mock.Anything, bcn.SearchParams{
		Query: "x", Page: 1, PageSize: 10,
	}).Return(bcn.SearchResponse{}, errGeneric).Once()

	res, err := s.callTool(map[string]any{"query": "x"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "search failed")
}

func (s *SearchLawsSuite) TestSearchTruncatesLongSummary() {
	long := strings.Repeat("lorem ipsum ", 200)
	s.lawClient.EXPECT().Search(mock.Anything, mock.Anything).Return(bcn.SearchResponse{
		Results:    []bcn.Norma{{IDNorma: 1, Norma: "Ley 1", Resumen: long}},
		Pagination: bcn.Pagination{TotalItems: 1, Query: "x"},
	}, nil).Once()

	res, err := s.callTool(map[string]any{"query": "x"})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text
	s.NotContains(text, long)
	s.Contains(text, "…")
}
