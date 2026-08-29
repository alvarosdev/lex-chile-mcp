package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// GetLawSuite validates the get_law tool against a MockLawClient: the BCN
// API is never reached.
type GetLawSuite struct {
	suite.Suite
	ctx       context.Context
	lawClient *bcn.MockLawClient
	session   *mcp.ClientSession
}

func TestGetLawSuite(t *testing.T) {
	suite.Run(t, new(GetLawSuite))
}

func (s *GetLawSuite) SetupTest() {
	s.ctx = context.Background()
	s.lawClient = bcn.NewMockLawClient(s.T())
	s.session = newTestClient(s.T(), s.ctx, s.lawClient)
}

func (s *GetLawSuite) callTool(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{
		Name:      "get_law",
		Arguments: args,
	})
}

func (s *GetLawSuite) sampleNorma() bcn.NormaFull {
	return bcn.NormaFull{
		Html: []bcn.HtmlBlock{
			{I: 1, T: "<div>LEY N&#xDA;M. 21.600</div>", Markdown: "LEY NÚM. 21.600", SectionName: "Encabezado"},
			{
				I: 2, T: "<div>Título I</div>", Markdown: "Disposiciones generales.", SectionName: "TÍTULO I",
				H: []bcn.HtmlBlock{
					{I: 3, T: "<div>Artículo 1º</div>", Markdown: "Objeto.", SectionName: "Artículo 1º"},
					{I: 4, T: "<div>Artículo 2º</div>", Markdown: "Definiciones.", SectionName: "Artículo 2º"},
				},
			},
		},
		Estructura: []bcn.EstructuraPart{
			{N: "Encabezado", I: 1},
			{N: "TÍTULO I", I: 2, H: []bcn.EstructuraPart{
				{N: "Artículo 1º", I: 3, T: 6},
				{N: "Artículo 2º", I: 4, T: 6},
			}},
		},
		Proyectos: []bcn.Proyecto{{
			Categoria: "Proyecto original",
			Pls: []struct {
				Enlace      string `json:"enlace"`
				Informacion string `json:"informacion"`
				NroBoletin  string `json:"nroBoletin"`
			}{{NroBoletin: "18156-04", Informacion: "Crea el servicio", Enlace: "https://senado.cl/tramitacion"}},
		}},
		Metadatos: bcn.Metadatos{
			TiposNumeros:     []bcn.TipoNumero{{Numero: "21600", Descripcion: "Ley"}},
			Organismos:       []string{"MINISTERIO DEL MEDIO AMBIENTE"},
			TituloNorma:      "CREA EL SERVICIO DE BIODIVERSIDAD",
			Fuente:           "Diario Oficial",
			NumeroFuente:     "43646",
			Materias:         []string{"Biodiversidad"},
			FechaPublicacion: "2023-09-06",
			Vigencia:         bcn.Vigencia{InicioVigencia: "2025-09-29"},
			Vinculaciones:    []bcn.Vinculacion{{Text: "MODIFICACION"}},
			Resumenes:        []string{"La presente ley crea el Servicio."},
		},
	}
}

func (s *GetLawSuite) TestGetLawFullContent() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1195666}).Return(s.sampleNorma(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1195666})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "# CREA EL SERVICIO DE BIODIVERSIDAD")
	s.Contains(text, "Type: Ley 21600 · Source: Diario Oficial N° 43646")
	s.Contains(text, "Derogated: false")
	s.Contains(text, "## Related bills")
	s.Contains(text, "18156-04")
	s.Contains(text, "## Structure")
	s.Contains(text, "- TÍTULO I")
	s.Contains(text, "## Content")
	s.Contains(text, "### Encabezado")
	s.Contains(text, "LEY NÚM. 21.600")

	// Structured content: typed, complete metadata; the markdown body
	// travels in the text view only — never duplicated here.
	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok, "structuredContent expected, got %T", res.StructuredContent)
	metadatos := sc["metadatos"].(map[string]any)
	s.Equal("CREA EL SERVICIO DE BIODIVERSIDAD", metadatos["titulo_norma"])
	_, hasContent := sc["content"]
	s.False(hasContent, "content markdown must never be duplicated in structuredContent")
	s.NotEmpty(sc["estructura"])
}

func (s *GetLawSuite) TestGetLawStructureOnlyOmitsContent() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1195666}).Return(s.sampleNorma(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1195666, "structure_only": true})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "## Structure")
	s.NotContains(text, "## Content")

	// Structured content with structure_only: content field omitted.
	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok, "structuredContent expected, got %T", res.StructuredContent)
	_, hasContent := sc["content"]
	s.False(hasContent, "content must be omitted with structure_only")
	s.NotEmpty(sc["estructura"])
}

func (s *GetLawSuite) TestGetLawInvalidNormIDFailsWithoutCallingClient() {
	// No expectations on the mock: the handler must not call GetNorma.
	res, err := s.callTool(map[string]any{"norm_id": 0})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "norm_id must be a positive number")

	res, err = s.callTool(map[string]any{})
	s.Require().NoError(err)
	s.True(res.IsError)
}

func (s *GetLawSuite) TestGetLawNotFoundSurfacesClearMessage() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 999999999, VersionDate: ""}).Return(bcn.NormaFull{}, bcn.ErrNormaNotFound).Once()

	res, err := s.callTool(map[string]any{"norm_id": 999999999})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "norma not found: norm_id 999999999")
}

func (s *GetLawSuite) TestGetLawGenericErrorSurfaces() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1, VersionDate: ""}).Return(bcn.NormaFull{}, errors.New("circuit breaker open")).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "get law failed")
}

func (s *GetLawSuite) TestGetLawWithVersionDate() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 141599, VersionDate: "2010-01-01"}).
		Return(s.sampleNorma(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 141599, "version_date": "2010-01-01"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "Version: as of 2010-01-01")

	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok)
	s.Equal("2010-01-01", sc["version_date"])
}

func (s *GetLawSuite) TestGetLawInvalidVersionDateFailsWithoutCallingClient() {
	// No expectations on the mock: the handler must not call GetNorma.
	for _, bad := range []string{"2010-13-45", "basura", "01/01/2010"} {
		res, err := s.callTool(map[string]any{"norm_id": 141599, "version_date": bad})
		s.Require().NoError(err)
		s.True(res.IsError)
		s.Contains(res.Content[0].(*mcp.TextContent).Text, "version_date must be a valid date")
	}
}

func (s *GetLawSuite) TestGetLawSectionReturnsOnlySubtree() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1195666}).Return(s.sampleNorma(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1195666, "section_id": 2})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	// The header names the section and the content is limited to its subtree.
	s.Contains(text, "Section: TÍTULO I")
	s.Contains(text, "### TÍTULO I")
	s.Contains(text, "#### Artículo 1º")
	s.Contains(text, "Objeto.")
	// The section view stays lightweight: summary and bills are skipped in
	// the text (they ride along complete in the structured output).
	s.NotContains(text, "Summary:", "section view must not repeat the law summary")
	s.NotContains(text, "## Related bills", "section view must not repeat the related bills")
	// The structure block is the LOCAL sub-TOC of the section's children —
	// the global index does not travel with section responses.
	s.Contains(text, "- Artículo 1º · section_id: 3")
	s.Contains(text, "- Artículo 2º · section_id: 4")
	s.NotContains(text, "- Encabezado", "global index must not travel with section responses")

	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok)
	s.Equal(float64(2), sc["section_id"])
	estructura := sc["estructura"].([]any)
	// Subtree only: TÍTULO I root + its two articles, no global entries.
	s.Len(estructura, 3)
	_, hasContent := sc["content"]
	s.False(hasContent, "content markdown must never be duplicated in structuredContent")
}

func (s *GetLawSuite) TestGetLawSectionNotFoundSuggestsStructureOnly() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1195666}).Return(s.sampleNorma(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1195666, "section_id": 999})
	s.Require().NoError(err)
	s.True(res.IsError)
	msg := res.Content[0].(*mcp.TextContent).Text
	s.Contains(msg, "section not found: section_id 999")
	s.Contains(msg, "structure_only=true")
}

func (s *GetLawSuite) TestGetLawNegativeSectionIDFailsWithoutCallingClient() {
	// No expectations on the mock: the handler must not call GetNorma.
	res, err := s.callTool(map[string]any{"norm_id": 1195666, "section_id": -1})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "section_id must be a positive number")
}

func (s *GetLawSuite) TestGetLawCountsDescribeReturnedContent() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1195666}).Return(s.sampleNorma(), nil).Twice()

	// Whole norm: counts describe the full document.
	full, err := s.callTool(map[string]any{"norm_id": 1195666})
	s.Require().NoError(err)
	fullText := full.Content[0].(*mcp.TextContent).Text
	s.Contains(fullText, "Size: ")
	s.Contains(fullText, "2 articles")
	fullSC := full.StructuredContent.(map[string]any)
	fullChars := fullSC["char_count"].(float64)
	s.Equal(float64(2), fullSC["article_count"])

	// Section: the size is in the text and the counts describe the section —
	// smaller char_count, same article count in this fixture (both articles
	// live inside Título I).
	section, err := s.callTool(map[string]any{"norm_id": 1195666, "section_id": 2})
	s.Require().NoError(err)
	sectionSC := section.StructuredContent.(map[string]any)
	s.Less(sectionSC["char_count"].(float64), fullChars)
	s.Equal(float64(2), sectionSC["article_count"])
}

func (s *GetLawSuite) TestGetLawStructureOnlyWithSectionIDOmitsContentAnyway() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1195666}).Return(s.sampleNorma(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1195666, "section_id": 2, "structure_only": true})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "Section: TÍTULO I", "the header still names the requested scope")
	s.NotContains(text, "## Content", "structure_only wins: no content")
}

func (s *GetLawSuite) TestGetLawSingleArticleUsesSingularCount() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 1195666}).Return(s.sampleNorma(), nil).Once()

	res, err := s.callTool(map[string]any{"norm_id": 1195666, "section_id": 3}) // Artículo 1º (leaf)
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "Section: Artículo 1º")
	s.Contains(text, "1 article", "singular for a single article")
	s.NotContains(text, "1 articles")
}
