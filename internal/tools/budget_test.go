package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// BudgetSuite validates the output-budget contract: bounded responses,
// signaled degradation (never silent truncation), folded TOCs, local
// sub-TOCs and the header hygiene rules. All against a MockLawClient —
// the BCN API is never reached. The synthetic meganorm fixture exercises
// the shapes real meganorms show: containers over the listing limit,
// textual article labels (BIS/TRANSITORIO), wide-gap section names,
// over-budget whole norm and over-budget individual section.
type BudgetSuite struct {
	suite.Suite
	ctx       context.Context
	lawClient *bcn.MockLawClient
	session   *mcp.ClientSession
}

func TestBudgetSuite(t *testing.T) {
	suite.Run(t, new(BudgetSuite))
}

func (s *BudgetSuite) SetupTest() {
	s.ctx = context.Background()
	s.lawClient = bcn.NewMockLawClient(s.T())
	s.session = newTestClient(s.T(), s.ctx, s.lawClient)
}

func (s *BudgetSuite) callTool(name string, args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
}

// megaNorma builds the synthetic meganorm fixture: exceeds the output
// budget as a whole AND in one individual section (TÍTULO GIGANTE, whose
// five article children together exceed the budget), has a container over
// the listing limit (TÍTULO I, 25 articles), a normally-listed container
// (TÍTULO II, 3 párrafos × 5 articles), textual article labels (BIS,
// TRANSITORIO), wide-gap section names and 15 vinculaciones.
func megaNorma() bcn.NormaFull {
	giant := func(id int64, n int) (bcn.EstructuraPart, bcn.HtmlBlock) {
		part := bcn.EstructuraPart{N: fmt.Sprintf("Sección gigante %d", id), I: id, T: 1}
		block := bcn.HtmlBlock{I: id, SectionName: part.N}
		for i := range n {
			artID := id*100 + int64(i)
			art := bcn.EstructuraPart{N: fmt.Sprintf("Artículo %d", i+1), I: artID, T: 6}
			part.H = append(part.H, art)
			block.H = append(block.H, bcn.HtmlBlock{
				I: artID, SectionName: art.N,
				Markdown: strings.Repeat("Cuerpo extenso del artículo. ", 900),
			})
		}
		return part, block
	}

	norma := bcn.NormaFull{
		Metadatos: bcn.Metadatos{
			TituloNorma:      "MEGANORMA SINTÉTICA PARA PRESUPUESTO",
			TiposNumeros:     []bcn.TipoNumero{{Numero: "1", Descripcion: "Ley"}},
			Organismos:       []string{"MINISTERIO DE PRUEBA"},
			Fuente:           "Diario Oficial",
			NumeroFuente:     "1",
			FechaPublicacion: "2026-01-01",
			Resumenes:        []string{"Resumen de la meganorma."},
		},
	}

	// Encabezado (leaf).
	norma.Estructura = append(norma.Estructura, bcn.EstructuraPart{N: "Encabezado", I: 1})
	norma.Html = append(norma.Html, bcn.HtmlBlock{I: 1, SectionName: "Encabezado", Markdown: "Encabezado."})

	// TÍTULO I: 25 articles (> maxListedChildren → collapses to a range).
	t1 := bcn.EstructuraPart{N: "TÍTULO I", I: 2, T: 1}
	t1Block := bcn.HtmlBlock{I: 2, SectionName: "TÍTULO I"}
	for i := range 25 {
		name := fmt.Sprintf("Artículo %d", i+1)
		if i == 12 {
			name = "Artículo 13 BIS" // textual labels exist
		}
		art := bcn.EstructuraPart{N: name, I: int64(100 + i), T: 6}
		t1.H = append(t1.H, art)
		t1Block.H = append(t1Block.H, bcn.HtmlBlock{I: int64(100 + i), SectionName: name, Markdown: "Cuerpo corto."})
	}
	norma.Estructura = append(norma.Estructura, t1)
	norma.Html = append(norma.Html, t1Block)

	// TÍTULO II (wide-gap name): 3 párrafos × 5 articles (listed normally).
	t2 := bcn.EstructuraPart{N: "TÍTULO II      DISPOSICIONES VARIAS", I: 3, T: 1}
	t2Block := bcn.HtmlBlock{I: 3, SectionName: t2.N}
	for p := range 3 {
		par := bcn.EstructuraPart{N: fmt.Sprintf("§ %d", p+1), I: int64(30 + p), T: 4}
		parBlock := bcn.HtmlBlock{I: int64(30 + p), SectionName: par.N}
		for a := range 5 {
			art := bcn.EstructuraPart{N: fmt.Sprintf("Artículo %d TRANSITORIO", p*5+a+1), I: int64(300 + p*10 + a), T: 6}
			par.H = append(par.H, art)
			parBlock.H = append(parBlock.H, bcn.HtmlBlock{I: art.I, SectionName: art.N, Markdown: "Cuerpo corto."})
		}
		t2.H = append(t2.H, par)
		t2Block.H = append(t2Block.H, parBlock)
	}
	norma.Estructura = append(norma.Estructura, t2)
	norma.Html = append(norma.Html, t2Block)

	// TÍTULO GIGANTE: five article children together exceed the budget.
	tg, tgBlock := giant(50, 5)
	tg.N = "TÍTULO GIGANTE"
	tgBlock.SectionName = tg.N
	norma.Estructura = append(norma.Estructura, tg)
	norma.Html = append(norma.Html, tgBlock)

	// 15 vinculaciones for the related-norms cap.
	for i := range 15 {
		norma.Metadatos.Vinculaciones = append(norma.Metadatos.Vinculaciones,
			bcn.Vinculacion{Text: fmt.Sprintf("Norma vinculada %d", i+1)})
	}
	return norma
}

// TestWithinBudgetMirrorsRenderer checks the budget helper against the
// renderer arithmetic: a small subtree fits, the giant one does not, and
// the per-section sizes equal the whole-subtree char count.
func (s *BudgetSuite) TestWithinBudgetMirrorsRenderer() {
	norma := megaNorma()

	s.True(withinBudget(norma.Html[0].H)) // empty children slice
	s.True(withinBudget([]bcn.HtmlBlock{norma.Html[1]}), "small container fits")
	s.False(withinBudget([]bcn.HtmlBlock{norma.Html[len(norma.Html)-1]}), "giant section must exceed the budget")

	sizes := bcn.ContentSizes(norma.Html)
	for _, block := range norma.Html {
		subtree := []bcn.HtmlBlock{block}
		s.Equal(bcn.ContentCharCount(subtree), sizes[block.I],
			"per-section size must equal the rendered char count of the subtree (id %d)", block.I)
	}
}

// TestFoldedTOCCollapsesOversizedContainer: TÍTULO I (25 articles)
// collapses into one ranged line with textual first–last labels.
func (s *BudgetSuite) TestFoldedTOCCollapsesOversizedContainer() {
	norma := megaNorma()
	flat := bcn.FlattenStructure(norma.Estructura)
	sizes := bcn.ContentSizes(norma.Html)

	var b strings.Builder
	renderFoldedTOC(&b, flat, sizes)
	out := b.String()

	s.Contains(out, "- TÍTULO I | section_id: 2 | ~")
	s.Contains(out, "arts. Artículo 1–Artículo 25 · 25 articles")
	s.NotContains(out, "Artículo 13 BIS ·", "individual articles of an oversized container are not listed")
	s.NotContains(out, "section_id: 113", "collapsed container (TÍTULO I, ids 100-124) lists no article ids")
}

// TestFoldedTOCListsSmallContainers: TÍTULO II (3 párrafos × 5 arts)
// lists every level with section ids and collapsed whitespace names.
func (s *BudgetSuite) TestFoldedTOCListsSmallContainers() {
	norma := megaNorma()
	flat := bcn.FlattenStructure(norma.Estructura)
	sizes := bcn.ContentSizes(norma.Html)

	var b strings.Builder
	renderFoldedTOC(&b, flat, sizes)
	out := b.String()

	s.Contains(out, "- TÍTULO II DISPOSICIONES VARIAS · section_id: 3", "wide-gap names collapse to single spaces")
	s.Contains(out, "- § 1 · section_id: 30")
	s.Contains(out, "- Artículo 1 TRANSITORIO · section_id: 300")
}

// TestGetLawDegradesOversizedNormToMap: the whole meganorm exceeds the
// budget — the response is a navigable map with the drill signal, never a
// partial body; counts state the REAL total and the structured map drops
// the global structure (it lives in summary/structure_only).
func (s *BudgetSuite) TestGetLawDegradesOversizedNormToMap() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 42}).Return(megaNorma(), nil).Once()

	res, err := s.callTool("get_law", map[string]any{"norm_id": 42})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	// Signal, not silence: the omission and the drill path are declared.
	s.Contains(text, "Content withheld:")
	s.Contains(text, "exceeds the 100K-char response budget")
	s.Contains(text, "Drill with section_id")
	// The giant body is NOT delivered.
	s.NotContains(text, "Cuerpo extenso", "degraded response must not carry the body")
	// The folded map travels with ids and ranges.
	s.Contains(text, "TÍTULO I | section_id: 2")
	s.Contains(text, "TÍTULO GIGANTE · section_id: 50")

	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok)
	s.Equal(true, sc["degraded"], "degraded flag travels in structured content")
	// REAL total: char_count describes the whole norm, not the map.
	realTotal := float64(bcn.ContentCharCount(megaNorma().Html))
	s.Equal(realTotal, sc["char_count"])
	s.Greater(realTotal, float64(maxContentChars))
	_, hasEstructura := sc["estructura"]
	s.False(hasEstructura, "degraded map: global structure stays in summary/structure_only")
	_, hasContent := sc["content"]
	s.False(hasContent)
}

// TestGetLawDegradesOversizedSectionRecursively: TÍTULO GIGANTE alone
// exceeds the budget — its response degrades to the map of its own
// children with the section's real total.
func (s *BudgetSuite) TestGetLawDegradesOversizedSectionRecursively() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 42}).Return(megaNorma(), nil).Twice()

	res, err := s.callTool("get_law", map[string]any{"norm_id": 42, "section_id": 50})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "Section: TÍTULO GIGANTE")
	s.Contains(text, "Content withheld:")
	s.NotContains(text, "Cuerpo extenso", "oversized section body is not delivered")
	// The local sub-TOC maps the section's children with ids and sizes.
	s.Contains(text, "Artículo 1 · section_id: 5000")

	sc, ok := res.StructuredContent.(map[string]any)
	s.Require().True(ok)
	s.Equal(true, sc["degraded"])
	s.Equal(float64(50), sc["section_id"])
	// REAL scope: the section's total, not the map's size.
	s.Greater(sc["char_count"].(float64), float64(maxContentChars))
	// Estructura is the section subtree (root + children), not the global one.
	estructura := sc["estructura"].([]any)
	s.Len(estructura, 6)

	// A child section that fits arrives WHOLE (never paginated/cut).
	res2, err := s.callTool("get_law", map[string]any{"norm_id": 42, "section_id": 5000})
	s.Require().NoError(err)
	s.False(res2.IsError)
	text2 := res2.Content[0].(*mcp.TextContent).Text
	s.Contains(text2, "Cuerpo extenso", "node that fits is delivered complete")
	sc2 := res2.StructuredContent.(map[string]any)
	_, hasDegraded := sc2["degraded"]
	s.False(hasDegraded, "degraded omitted when false (omitempty)")
}

// TestGetLawSummaryFoldsMegaStructure: the summary of the meganorm stays
// bounded — containers with ranges, no per-article listing of oversized
// containers, sizes present.
func (s *BudgetSuite) TestGetLawSummaryFoldsMegaStructure() {
	norma := megaNorma()
	s.lawClient.EXPECT().GetNormaSummary(mock.Anything, bcn.NormaQuery{NormID: 42}).Return(bcn.NormaSummary{
		TituloNorma:  norma.Metadatos.TituloNorma,
		Estructura:   bcn.FlattenStructure(norma.Estructura),
		CharCount:    bcn.ContentCharCount(norma.Html),
		ArticleCount: norma.CountArticles(),
		SectionSizes: bcn.ContentSizes(norma.Html),
	}, nil).Once()

	res, err := s.callTool("get_law_summary", map[string]any{"norm_id": 42})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "arts. Artículo 1–Artículo 25 · 25 articles")
	s.NotContains(text, "Artículo 13 BIS · section_id")
	s.Contains(text, "TÍTULO GIGANTE · section_id: 50")
	// Bounded map: the folded text stays far under the budget.
	s.Less(len(text), 10_000, "folded meganorm map stays in a few thousand tokens")
}

// TestGetLawRelatedNormsCapped: more than maxRelatedNorms vinculaciones —
// the header caps and signals the total; the structured metadata keeps
// every one.
func (s *BudgetSuite) TestGetLawRelatedNormsCapped() {
	norma := megaNorma()
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 42}).Return(norma, nil).Once()

	res, err := s.callTool("get_law", map[string]any{"norm_id": 42})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "Related norms: 15 total — showing first 10: Norma vinculada 1")
	s.NotContains(text, "Norma vinculada 11;", "visible list stops at the cap")
	s.NotContains(text, "Norma vinculada 15", "cap excludes the rest of the header line")

	sc := res.StructuredContent.(map[string]any)
	metadatos := sc["metadatos"].(map[string]any)
	vinculaciones := metadatos["vinculaciones"].([]any)
	s.Len(vinculaciones, 15, "structured metadata keeps the complete list")
}

// TestCollapseSpaces unit-checks the whitespace hygiene.
func (s *BudgetSuite) TestCollapseSpaces() {
	s.Equal("TÍTULO VII DE LA FILIACIÓN", collapseSpaces("TÍTULO VII      DE LA FILIACIÓN"))
	s.Equal("Artículo 1", collapseSpaces("  Artículo\t1  "))
	s.Equal("", collapseSpaces("   "))
}

// TestSearchLawsViewsAligned: text and structured summaries carry the
// SAME truncation — no complete-summary duplication in structured.
func (s *BudgetSuite) TestSearchLawsViewsAligned() {
	longSummary := strings.Repeat("resumen extenso de la norma. ", 100) // ~3K chars
	s.lawClient.EXPECT().Search(mock.Anything, bcn.SearchParams{Query: "dicom", Page: 1, PageSize: 10}).Return(bcn.SearchResponse{
		Results: []bcn.Norma{{
			IDNorma:          7,
			Norma:            "Ley 1",
			Tipo:             "Ley",
			TituloNorma:      "Ley de prueba",
			FechaPublicacion: "2026-01-01",
			Resumen:          longSummary,
		}},
	}, nil).Once()

	res, err := s.callTool("search_laws", map[string]any{"query": "dicom"})
	s.Require().NoError(err)
	s.False(res.IsError)

	sc := res.StructuredContent.(map[string]any)
	results := sc["results"].([]any)
	first := results[0].(map[string]any)
	structuredSummary := first["summary"].(string)

	s.Equal(truncate(longSummary, 600), structuredSummary, "structured summary matches the text-view truncation")
	s.Len([]rune(structuredSummary), 600+1, "600 runes + ellipsis")
	s.True(strings.HasSuffix(structuredSummary, "…"))
}

// TestCompletenessInvariants walks every response mode that omits content
// (degraded norm, degraded section, section view, summary) and asserts
// the three invariants: declared signal or scope, REAL totals, and the
// retrieval path (section ids) inside the response.
func (s *BudgetSuite) TestCompletenessInvariants() {
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 42}).Return(megaNorma(), nil).Times(3)
	s.lawClient.EXPECT().GetNormaSummary(mock.Anything, bcn.NormaQuery{NormID: 42}).Return(func() bcn.NormaSummary {
		norma := megaNorma()
		return bcn.NormaSummary{
			TituloNorma:  norma.Metadatos.TituloNorma,
			Estructura:   bcn.FlattenStructure(norma.Estructura),
			CharCount:    bcn.ContentCharCount(norma.Html),
			ArticleCount: norma.CountArticles(),
			SectionSizes: bcn.ContentSizes(norma.Html),
		}
	}(), nil).Once()

	checks := []struct {
		name string
		args map[string]any
	}{
		{"degraded whole norm", map[string]any{"norm_id": 42}},
		{"degraded section", map[string]any{"norm_id": 42, "section_id": 50}},
		{"delivered section", map[string]any{"norm_id": 42, "section_id": 3}},
	}
	for _, tc := range checks {
		res, err := s.callTool("get_law", tc.args)
		s.Require().NoError(err, tc.name)
		text := res.Content[0].(*mcp.TextContent).Text
		sc := res.StructuredContent.(map[string]any)

		// Invariant: the size line always states the REAL scope total.
		s.Contains(text, "Size: ", tc.name)
		s.NotEqual(float64(0), sc["char_count"], tc.name)
		// Invariant: the retrieval path (section ids) travels inside.
		s.Contains(text, "section_id: ", tc.name)
	}

	// The summary map carries ids and the real total too.
	res, err := s.callTool("get_law_summary", map[string]any{"norm_id": 42})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "section_id: 2")
	s.Contains(text, "chars ·")
	sc := res.StructuredContent.(map[string]any)
	s.Greater(sc["char_count"].(float64), float64(maxContentChars))
}

// TestFoldedTOCCompactMapForDeepNarrowStructures: structures where no node
// breaks the children limit still exceed the TOC budget — the degradation
// chain must kick in. 80 títulos (compact map with child lines still over
// budget) falls back to top-level lines only; the folded form of a
// mid-size structure stays untouched.
func (s *BudgetSuite) TestFoldedTOCCompactMapForDeepNarrowStructures() {
	build := func(titulos int) bcn.NormaFull {
		norma := bcn.NormaFull{}
		for t := range titulos {
			titulo := bcn.EstructuraPart{N: fmt.Sprintf("TÍTULO %d", t+1), I: int64(1000 + t), T: 1}
			blocks := bcn.HtmlBlock{I: int64(1000 + t), SectionName: titulo.N}
			for p := range 3 {
				par := bcn.EstructuraPart{N: fmt.Sprintf("§ %d", p+1), I: int64(10000 + t*10 + p), T: 4}
				parBlock := bcn.HtmlBlock{I: par.I, SectionName: par.N}
				for a := range 8 {
					art := bcn.EstructuraPart{N: fmt.Sprintf("Artículo %d", p*8+a+1), I: int64(100000 + t*100 + p*10 + a), T: 6}
					par.H = append(par.H, art)
					parBlock.H = append(parBlock.H, bcn.HtmlBlock{I: art.I, SectionName: art.N, Markdown: "Cuerpo."})
				}
				titulo.H = append(titulo.H, par)
				blocks.H = append(blocks.H, parBlock)
			}
			norma.Estructura = append(norma.Estructura, titulo)
			norma.Html = append(norma.Html, blocks)
		}
		return norma
	}

	render := func(norma bcn.NormaFull) string {
		var b strings.Builder
		renderFoldedTOC(&b, bcn.FlattenStructure(norma.Estructura), bcn.ContentSizes(norma.Html))
		return b.String()
	}

	// 80 títulos: 1920 article lines folded → compact map (320 lines,
	// ~23K) still over budget → top-level lines only (~80 lines).
	out := render(build(80))
	s.Less(len(out), maxTOCChars+500, "third degradation level keeps the map bounded")
	s.Contains(out, "TÍTULO 1 | section_id: 1000 | ~")
	s.Contains(out, "arts. Artículo 1–Artículo 24 · 24 articles")
	s.NotContains(out, "§ 1 |", "child lines drop in the top-only fallback")
	s.NotContains(out, "Artículo 5 · section_id:", "individual articles never survive past the fold")

	// 40 títulos: folded explodes (~48K) → compact map fits (~11K):
	// top lines plus one ranged line per direct child.
	out40 := render(build(40))
	s.Contains(out40, "TÍTULO 1 · section_id: 1000")
	s.Contains(out40, "§ 1 | section_id: 10000 | ~")
	s.Contains(out40, "arts. Artículo 1–Artículo 8 · 8 articles")
	s.Less(len(out40), maxTOCChars+500)
}

// TestGetLawRelatedBillsCapped: meganorms in permanent amendment carry
// hundreds of bills — the header caps them with the total signaled and
// the structured output keeps every one (regression for the e2e finding
// on the Código Civil, which lists ~330 bills).
func (s *BudgetSuite) TestGetLawRelatedBillsCapped() {
	norma := megaNorma()
	for i := range 25 {
		norma.Proyectos = append(norma.Proyectos, bcn.Proyecto{
			Pls: []struct {
				Enlace      string `json:"enlace"`
				Informacion string `json:"informacion"`
				NroBoletin  string `json:"nroBoletin"`
			}{{NroBoletin: fmt.Sprintf("1000%d-01", i), Informacion: fmt.Sprintf("Proyecto %d", i)}},
		})
	}
	s.lawClient.EXPECT().GetNorma(mock.Anything, bcn.NormaQuery{NormID: 42}).Return(norma, nil).Once()

	res, err := s.callTool("get_law", map[string]any{"norm_id": 42})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "25 total — showing first 10")
	s.Contains(text, "10000-01 — Proyecto 0")
	s.NotContains(text, "Proyecto 10 —", "visible bill list stops at the cap")
	s.NotContains(text, "Proyecto 24", "cap excludes the rest")

	sc := res.StructuredContent.(map[string]any)
	proyectos := sc["proyectos"].([]any)
	s.Len(proyectos, 25, "structured output keeps every bill")
}
