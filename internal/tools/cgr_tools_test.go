package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

func newTestCgrClient(t *testing.T, ctx context.Context, cgrClient cgr.CgrClient) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	RegisterCgrTools(server, cgrClient)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect(): %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect(): %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

// SearchCgrSuite
type SearchCgrSuite struct {
	suite.Suite
	ctx       context.Context
	cgrClient *cgr.MockCgrClient
	session   *mcp.ClientSession
}

func TestSearchCgrSuite(t *testing.T) {
	suite.Run(t, new(SearchCgrSuite))
}

func (s *SearchCgrSuite) SetupTest() {
	s.ctx = context.Background()
	s.cgrClient = cgr.NewMockCgrClient(s.T())
	s.session = newTestCgrClient(s.T(), s.ctx, s.cgrClient)
}

func (s *SearchCgrSuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{Name: "search_cgr_dictamenes", Arguments: args})
}

func (s *SearchCgrSuite) callGeneric(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{Name: "search_cgr", Arguments: args})
}

func (s *SearchCgrSuite) TestSearchSuccess() {
	s.cgrClient.EXPECT().Search(mock.Anything, cgr.SearchParams{
		Query: "quillota", ExactSearch: false, Order: "date", Page: 1, Source: "dictamenes",
	}).Return(cgr.SearchResponse{
		Results: []cgr.DictamenSummary{{
			DictamenID: "OF80660N26", NDictamen: "OF80660", FechaDoc: "2026-04-27",
			Materia: "Cursa con alcances", Descriptores: "Facultades CGR", URL: "https://www.contraloria.cl/buscadorpdf/dictamenes/OF80660N26/html", PDFURL: "https://www.contraloria.cl/buscadorpdf/dictamenes/OF80660N26/pdf",
		}},
		Pagination: cgr.Pagination{Total: 312, Page: 1, PageSize: 20, TotalPages: 16, HasMore: true},
	}, nil).Once()

	res, err := s.callTool(map[string]any{"query": "quillota"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "312")
	s.Contains(text, "OF80660N26")
	s.Contains(text, "https://www.contraloria.cl/buscadorpdf/dictamenes/OF80660N26/html")
	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok)
	s.NotNil(sc["results"])
	s.NotNil(sc["pagination"])
}

func (s *SearchCgrSuite) TestSearchInvalidOrder() {
	res, err := s.callTool(map[string]any{"query": "x", "order": "invalid"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "order must be")
}

func (s *SearchCgrSuite) TestSearchClientError() {
	s.cgrClient.EXPECT().Search(mock.Anything, mock.Anything).Return(cgr.SearchResponse{}, errors.New("upstream")).Once()
	res, err := s.callTool(map[string]any{"query": "x"})
	s.Require().NoError(err)
	s.True(res.IsError)
}

func (s *SearchCgrSuite) TestSearch_InvalidSource_NoIO() {
	// invalid source must be rejected without calling client.Search
	res, err := s.callGeneric(map[string]any{"query": "x", "source": "../count/todos"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "invalid source")
	s.cgrClient.AssertNotCalled(s.T(), "Search", mock.Anything, mock.Anything)

	res, err = s.callGeneric(map[string]any{"query": "x", "source": "web%2fadmin"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "invalid source")
	s.cgrClient.AssertNotCalled(s.T(), "Search", mock.Anything, mock.Anything)

	res, err = s.callGeneric(map[string]any{"query": "x", "source": "invalido"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "invalid source")
	s.cgrClient.AssertNotCalled(s.T(), "Search", mock.Anything, mock.Anything)

	// case-insensitive valid source should succeed (mock expectation)
	s.cgrClient.EXPECT().Search(mock.Anything, cgr.SearchParams{Query: "x", Order: "date", Page: 1, Source: "contable"}).Return(cgr.SearchResponse{
		Results: []cgr.DictamenSummary{}, Pagination: cgr.Pagination{Total: 0, Page: 1, PageSize: 20},
	}, nil).Once()
	res, err = s.callGeneric(map[string]any{"query": "x", "source": "Contable"})
	s.Require().NoError(err)
	s.False(res.IsError)
}

func (s *SearchCgrSuite) TestSearch_PageOverflow() {
	res, err := s.callGeneric(map[string]any{"query": "x", "page": 501})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "page must be <= 500")
	s.cgrClient.AssertNotCalled(s.T(), "Search", mock.Anything, mock.Anything)

	res, err = s.callGeneric(map[string]any{"query": "x", "page": 9999})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "page must be <= 500")
	s.cgrClient.AssertNotCalled(s.T(), "Search", mock.Anything, mock.Anything)

	s.cgrClient.EXPECT().Search(mock.Anything, cgr.SearchParams{Query: "x", Order: "date", Page: 1, Source: "dictamenes"}).Return(cgr.SearchResponse{
		Results: []cgr.DictamenSummary{}, Pagination: cgr.Pagination{Total: 0, Page: 1, PageSize: 20},
	}, nil).Once()
	res, err = s.callGeneric(map[string]any{"query": "x", "page": 0})
	s.Require().NoError(err)
	s.False(res.IsError)
}

func (s *SearchCgrSuite) TestSearch_QueryTruncated() {
	long := strings.Repeat("a", 800)
	truncated := strings.Repeat("a", 500)
	s.cgrClient.EXPECT().Search(mock.Anything, cgr.SearchParams{Query: truncated, Order: "date", Page: 1, Source: "dictamenes"}).Return(cgr.SearchResponse{
		Results: []cgr.DictamenSummary{}, Pagination: cgr.Pagination{Total: 0, Page: 1, PageSize: 20},
	}, nil).Once()
	res, err := s.callGeneric(map[string]any{"query": long})
	s.Require().NoError(err)
	s.False(res.IsError)
}

func (s *SearchCgrSuite) TestSearch_SourceEnum() {
	sources := []string{"dictamenes", "instructivos", "contable", "auditoria", "legislacion", "cuentas", "consolidados", "web", "todos"}
	for _, src := range sources {
		s.cgrClient.EXPECT().Search(mock.Anything, cgr.SearchParams{Query: "municipalidad", Order: "date", Page: 1, Source: src}).Return(cgr.SearchResponse{
			Results:    []cgr.DictamenSummary{{DictamenID: "ID", Materia: "m", URL: "https://example.com", PDFURL: "https://example.com/pdf"}},
			Pagination: cgr.Pagination{Total: 1, Page: 1, PageSize: 20, TotalPages: 1, HasMore: false},
		}, nil).Once()
		res, err := s.callGeneric(map[string]any{"query": "municipalidad", "source": src})
		s.Require().NoError(err)
		s.False(res.IsError, "source %s should be valid", src)
	}
}

func (s *SearchCgrSuite) TestSearch_SourceWithTrimAndLower() {
	s.cgrClient.EXPECT().Search(mock.Anything, cgr.SearchParams{Query: "x", Order: "date", Page: 1, Source: "contable"}).Return(cgr.SearchResponse{
		Results: []cgr.DictamenSummary{}, Pagination: cgr.Pagination{Total: 0, Page: 1, PageSize: 20},
	}, nil).Once()
	res, err := s.callGeneric(map[string]any{"query": "x", "source": "  CONTABLE  "})
	s.Require().NoError(err)
	s.False(res.IsError)
}

// GetCgrSuite
type GetCgrSuite struct {
	suite.Suite
	ctx       context.Context
	cgrClient *cgr.MockCgrClient
	session   *mcp.ClientSession
}

func TestGetCgrSuite(t *testing.T) {
	suite.Run(t, new(GetCgrSuite))
}

func (s *GetCgrSuite) SetupTest() {
	s.ctx = context.Background()
	s.cgrClient = cgr.NewMockCgrClient(s.T())
	s.session = newTestCgrClient(s.T(), s.ctx, s.cgrClient)
}

func (s *GetCgrSuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{Name: "get_cgr_dictamen", Arguments: args})
}

func (s *GetCgrSuite) TestGetSuccess() {
	s.cgrClient.EXPECT().GetDictamen(mock.Anything, "E179593N25").Return(cgr.DictamenFull{
		DictamenSummary: cgr.DictamenSummary{
			DictamenID: "E179593N25", NDictamen: "E179593", FechaDoc: "2025-10-22",
			Materia: "Cursa con alcance", URL: "https://www.contraloria.cl/buscadorpdf/dictamenes/E179593N25/html", PDFURL: "https://www.contraloria.cl/buscadorpdf/dictamenes/E179593N25/pdf",
		},
		Documento: "N° E179593  Fecha: 22-10-2025\nContenido",
		CharCount: 50,
	}, nil).Once()

	res, err := s.callTool(map[string]any{"dictamen_id": "E179593N25"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "E179593N25")
	s.Contains(text, "https://www.contraloria.cl/buscadorpdf/dictamenes/E179593N25/html")
	s.Contains(text, "https://www.contraloria.cl/buscadorpdf/dictamenes/E179593N25/pdf")
	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok)
	s.Equal("E179593N25", sc["dictamen_id"])
}

func (s *GetCgrSuite) TestGetMissingID() {
	res, err := s.callTool(map[string]any{})
	s.Require().NoError(err)
	s.True(res.IsError)
}

func (s *GetCgrSuite) TestGetNotFound() {
	s.cgrClient.EXPECT().GetDictamen(mock.Anything, "E999999N99").Return(cgr.DictamenFull{}, cgr.ErrDictamenNotFound).Once()
	res, err := s.callTool(map[string]any{"dictamen_id": "E999999N99"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "not found")
}

// CountCgrSuite
type CountCgrSuite struct {
	suite.Suite
	ctx       context.Context
	cgrClient *cgr.MockCgrClient
	session   *mcp.ClientSession
}

func TestCountCgrSuite(t *testing.T) {
	suite.Run(t, new(CountCgrSuite))
}

func (s *CountCgrSuite) SetupTest() {
	s.ctx = context.Background()
	s.cgrClient = cgr.NewMockCgrClient(s.T())
	s.session = newTestCgrClient(s.T(), s.ctx, s.cgrClient)
}

func (s *CountCgrSuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{Name: "count_cgr_jurisprudencia", Arguments: args})
}

func (s *CountCgrSuite) TestCountSuccess() {
	s.cgrClient.EXPECT().CountJurisprudencia(mock.Anything, "quillota", false).Return(cgr.CountResponse{
		Query: "quillota", Total: 1255,
		Buckets: []cgr.CountBucket{{Type: "dictamenes", Count: 312}, {Type: "auditoria", Count: 690}},
	}, nil).Once()
	res, err := s.callTool(map[string]any{"query": "quillota"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "1255")
	s.Contains(text, "dictamenes 312")
}
