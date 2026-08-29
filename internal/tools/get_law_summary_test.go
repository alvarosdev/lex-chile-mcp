package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// GetLawSummarySuite validates the get_law_summary tool against a
// MockLawClient: the BCN API is never reached.
type GetLawSummarySuite struct {
	suite.Suite
	ctx       context.Context
	lawClient *bcn.MockLawClient
	session   *mcp.ClientSession
}

func TestGetLawSummarySuite(t *testing.T) {
	suite.Run(t, new(GetLawSummarySuite))
}

func (s *GetLawSummarySuite) SetupTest() {
	s.ctx = context.Background()
	s.lawClient = bcn.NewMockLawClient(s.T())
	s.session = newTestClient(s.T(), s.ctx, s.lawClient)
}

func (s *GetLawSummarySuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{
		Name:      "get_law_summary",
		Arguments: args,
	})
}

func (s *GetLawSummarySuite) TestSummaryValid() {
	s.lawClient.EXPECT().GetNormaSummary(mock.Anything, bcn.NormaQuery{NormID: 1142880}).Return(bcn.NormaSummary{
		TituloNorma:     "MODIFICA LA LEY N° 19.628, SOBRE PROTECCIÓN DE LA VIDA PRIVADA",
		Fuente:          "Diario Oficial",
		Materias:        []string{"Protección Vida Privada", "Informe sobre Deudas"},
		CategoriasNorma: []string{"Ley \"Educación sin Dicom\""},
		Resumenes:       []string{"La presente ley establece la prohibición de comunicar información."},
		Estructura:      []bcn.StructurePartOut{{Name: "TÍTULO I", ID: 100, Depth: 0}},
		SectionSizes:    map[int64]int{100: 2870},
		CharCount:       3200,
		ArticleCount:    4,
	}, nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1142880})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "# MODIFICA LA LEY N° 19.628")
	s.Contains(text, "Categories: Ley \"Educación sin Dicom\"")
	s.Contains(text, "Summary:")
	s.Contains(text, "prohibición de comunicar información")
	s.Contains(text, "Size: 3.2K chars · 4 articles")
	s.Contains(text, "## Structure", "the summary carries the map of the law (structure with section ids)")
	s.Contains(text, "- TÍTULO I · section_id: 100 · 2.9K")
	s.NotContains(text, "## Content", "summary must never include the norm content")

	// Structured content: typed and complete.
	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok, "structuredContent expected, got %T", res.StructuredContent)
	s.Equal("MODIFICA LA LEY N° 19.628, SOBRE PROTECCIÓN DE LA VIDA PRIVADA", sc["titulo_norma"])
	materias := sc["materias"].([]any)
	s.Len(materias, 2)
	s.Equal(float64(3200), sc["char_count"])
	s.Equal(float64(4), sc["article_count"])
	s.NotEmpty(sc["estructura"])
	_, hasContent := sc["content"]
	s.False(hasContent, "summary structured content must not include the norm content")
}

func (s *GetLawSummarySuite) TestSummaryInvalidNormIDFailsWithoutCallingClient() {
	// No expectations on the mock: the handler must not call GetNormaSummary.
	res, err := s.callTool(map[string]any{"norm_id": 0})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "norm_id must be a positive number")
}

func (s *GetLawSummarySuite) TestSummaryNotFoundSurfacesClearMessage() {
	s.lawClient.EXPECT().GetNormaSummary(mock.Anything, bcn.NormaQuery{NormID: 999999999}).
		Return(bcn.NormaSummary{}, bcn.ErrNormaNotFound).Once()

	res, err := s.callTool(map[string]any{"norm_id": 999999999})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "norma not found: norm_id 999999999")
}

func (s *GetLawSummarySuite) TestSummaryWithVersionDate() {
	s.lawClient.EXPECT().GetNormaSummary(mock.Anything, bcn.NormaQuery{NormID: 141599, VersionDate: "2010-01-01"}).
		Return(bcn.NormaSummary{
			TituloNorma: "SOBRE PROTECCION DE LA VIDA PRIVADA",
			Fuente:      "Diario Oficial",
			Resumenes:   []string{"Resumen de la versión histórica."},
		}, nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 141599, "version_date": "2010-01-01"})
	s.Require().NoError(err)
	s.False(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "Version: as of 2010-01-01")
}

func (s *GetLawSummarySuite) TestSummaryInvalidVersionDateFailsWithoutCallingClient() {
	// No expectations on the mock: the handler must not call GetNormaSummary.
	res, err := s.callTool(map[string]any{"norm_id": 141599, "version_date": "no-es-fecha"})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "version_date must be a valid date")
}
