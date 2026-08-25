package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// GetLawHistorySuite validates the get_law_history tool against a
// MockLawClient: the BCN API is never reached.
type GetLawHistorySuite struct {
	suite.Suite
	ctx       context.Context
	lawClient *bcn.MockLawClient
	session   *mcp.ClientSession
}

func TestGetLawHistorySuite(t *testing.T) {
	suite.Run(t, new(GetLawHistorySuite))
}

func (s *GetLawHistorySuite) SetupTest() {
	s.ctx = context.Background()
	s.lawClient = bcn.NewMockLawClient(s.T())
	s.session = newTestClient(s.T(), s.ctx, s.lawClient)
}

func (s *GetLawHistorySuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{
		Name:      "get_law_history",
		Arguments: args,
	})
}

func (s *GetLawHistorySuite) sampleGrupos() []bcn.HistoriaGrupo {
	return []bcn.HistoriaGrupo{
		{
			Titulo:   "Historia de la ley",
			TipoDesc: "Historia de la tramitación y discusión de esta ley",
			TipoCod:  1,
			Hls: []bcn.HistoriaEntrada{{
				Tipo: 1, IDNorma: 1195666, IDNormaHL: 1195666,
				Fecha: "2023-09-06", Descripcion: "Historia de la Ley N° 21.600",
				Bajada: "Crea el Servicio de Biodiversidad", Enlace: "https://www.bcn.cl/historiadelaley/nc/historia-de-la-ley/8203/",
			}},
		},
		{
			Titulo:   "Historias de la ley modificatorias",
			TipoDesc: "Historias de la ley que reformaron o alteraron la presente Ley",
			TipoCod:  3,
			Hls: []bcn.HistoriaEntrada{{
				Tipo: 3, IDNorma: 1195666, IDNormaHL: 1216930,
				Fecha: "2025-09-29", Descripcion: "Historia de la Ley N° 21.770",
				Bajada: "Establece una Ley Marco de Autorizaciones Sectoriales",
				Enlace: "https://www.bcn.cl/historiadelaley/nc/historia-de-la-ley/8424/",
			}},
		},
	}
}

func (s *GetLawHistorySuite) TestHistoryValid() {
	s.lawClient.EXPECT().GetLawHistory(mock.Anything, int64(1195666)).Return(s.sampleGrupos(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1195666})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "## Historia de la ley")
	s.Contains(text, "## Historias de la ley modificatorias")
	s.Contains(text, "Historia de la Ley N° 21.770")
	// The modificatoria's ficha link is built from id_norma_hl (1216930),
	// the record's own norm — never from the related id_norma (1195666,
	// which legitimately appears as the own-history ficha of group 1).
	s.Contains(text, "Ficha: https://www.leychile.cl/Navegar?idNorma=1216930")

	// Structured: typed groups wrapped in an object (MCP requires
	// outputSchema to be an object, never a top-level array).
	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok, "structuredContent expected as object, got %T", res.StructuredContent)
	groups, ok := sc["groups"].([]any)
	s.Require().True(ok, "groups expected as array, got %T", sc["groups"])
	s.Len(groups, 2)
}

func (s *GetLawHistorySuite) TestHistoryEmptyFriendlyMessage() {
	s.lawClient.EXPECT().GetLawHistory(mock.Anything, int64(999999999)).Return([]bcn.HistoriaGrupo{}, nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 999999999})
	s.Require().NoError(err)
	s.False(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "No legislative history found for norm_id 999999999")
}

func (s *GetLawHistorySuite) TestHistoryInvalidNormIDFailsWithoutCallingClient() {
	// No expectations on the mock: the handler must not call GetLawHistory.
	res, err := s.callTool(map[string]any{"norm_id": 0})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "norm_id must be a positive number")
}
