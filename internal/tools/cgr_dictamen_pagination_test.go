package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
	"github.com/alvarosdev/lex-chile-mcp/internal/outputbudget"
)

// CgrDictamenPaginationSuite validates the linear pagination of
// get_cgr_dictamen: budget-sized parts, paragraph-safe cuts, range
// signals on every part, whole-document char_count, metadata on part 1
// only, and the argument-error contract. Against a MockCgrClient — the
// CGR API is never reached.
type CgrDictamenPaginationSuite struct {
	suite.Suite
	ctx       context.Context
	cgrClient *cgr.MockCgrClient
	session   *mcp.ClientSession
}

func TestCgrDictamenPaginationSuite(t *testing.T) {
	suite.Run(t, new(CgrDictamenPaginationSuite))
}

func (s *CgrDictamenPaginationSuite) SetupTest() {
	s.ctx = context.Background()
	s.cgrClient = cgr.NewMockCgrClient(s.T())
	s.session = newTestCgrClient(s.T(), s.ctx, s.cgrClient)
}

// multiPartDictamen builds a document ~2.4× the output budget with clean
// paragraph breaks every ~100 runes, so every boundary rounds to a
// paragraph break well within the margin.
func (s *CgrDictamenPaginationSuite) multiPartDictamen() cgr.DictamenFull {
	para := strings.Repeat("Considerando el texto de ejemplo ", 3) // ~100 runes
	doc := strings.TrimSuffix(strings.Repeat(para+"\n\n", int(outputbudget.MaxContentChars/100)+20), "\n\n")
	return cgr.DictamenFull{
		DictamenSummary: cgr.DictamenSummary{
			DictamenID: "E100N26",
			NDictamen:  "N° 100",
			FechaDoc:   "2026-01-15",
			Materia:    "Materia de prueba",
			Caracter:   "Obligatorio",
			URL:        "https://www.contraloria.cl/buscadorpdf/dictamenes/E100N26/html",
			PDFURL:     "https://www.contraloria.cl/buscadorpdf/dictamenes/E100N26/pdf",
		},
		Documento: doc,
		CharCount: len([]rune(doc)),
	}
}

// expectE100 registers the repeatable fixture expectation: pagination is
// deterministic, so any number of calls yields the same slices. Maybe
// (testify 1.12 dropped AnyTimes) allows 0..n calls per test.
func (s *CgrDictamenPaginationSuite) expectE100() {
	s.cgrClient.EXPECT().GetDictamen(mock.Anything, "E100N26").Return(s.multiPartDictamen(), nil).Maybe()
}

func (s *CgrDictamenPaginationSuite) call(args map[string]any) (*mcp.CallToolResult, error) {
	return s.session.CallTool(s.ctx, &mcp.CallToolParams{
		Name:      "get_cgr_dictamen",
		Arguments: args,
	})
}

func (s *CgrDictamenPaginationSuite) TestShortDocumentPartOneOfOne() {
	doc := "Párrafo único del dictamen de prueba."
	s.cgrClient.EXPECT().GetDictamen(mock.Anything, "E100N26").Return(cgr.DictamenFull{
		DictamenSummary: cgr.DictamenSummary{DictamenID: "E100N26", Materia: "m"},
		Documento:       doc,
		CharCount:       len([]rune(doc)),
	}, nil).Once()

	res, err := s.call(map[string]any{"dictamen_id": "E100N26"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	s.Contains(text, "part 1 of 1")
	s.Contains(text, doc)
	s.Contains(text, "**Materia:** m", "part 1 carries the metadata header")

	sc := res.StructuredContent.(map[string]any)
	s.Equal(float64(1), sc["part"])
	s.Equal(float64(1), sc["total_parts"])
	s.Equal(float64(len([]rune(doc))), sc["char_count"], "char_count is the whole document")
	s.Equal(doc, sc["documento_completo"])
}

func (s *CgrDictamenPaginationSuite) TestMultiPartDocumentPartOneSignalsContinuation() {
	s.expectE100()
	res, err := s.call(map[string]any{"dictamen_id": "E100N26"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text

	doc := s.multiPartDictamen().Documento
	pages := cgr.PaginateStandard(doc)
	total := len([]rune(doc))
	page1 := pages.Range(0)

	s.Contains(text, fmt.Sprintf("**Documento:** part 1 of %d · chars 1–%d of %d · continue with part=2",
		pages.Count(), page1.End, total))
	// Part 1 never shows the tail of the document.
	s.NotContains(text, strings.Repeat("Considerando", 3), "part 1 must not include later parts")
	// The metadata header travels on part 1.
	s.Contains(text, "**Materia:** Materia de prueba")
	s.Contains(text, "**Citación:**")

	sc := res.StructuredContent.(map[string]any)
	s.Equal(float64(1), sc["part"])
	s.Equal(float64(pages.Count()), sc["total_parts"])
	s.Equal(float64(total), sc["char_count"])
}

func (s *CgrDictamenPaginationSuite) TestContinuationPartContinuesWithoutOverlap() {
	s.expectE100()
	// Two consecutive part=2 calls must return the exact same slice —
	// deterministic pagination — and no metadata header.
	res, err := s.call(map[string]any{"dictamen_id": "E100N26", "part": 2})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text

	pages := cgr.PaginateStandard(s.multiPartDictamen().Documento)
	page2 := pages.Range(1)
	doc := s.multiPartDictamen().Documento
	want := string([]rune(doc)[page2.Start:page2.End])

	s.Contains(text, fmt.Sprintf("# Dictamen E100N26 — part 2 of %d", pages.Count()))
	s.Contains(text, want)
	s.NotContains(text, "**Materia:**", "continuation parts skip the metadata header")
	s.NotContains(text, "**Citación:**", "continuation parts skip the citación")
	s.Contains(text, fmt.Sprintf("part 2 of %d · chars %d–%d of %d",
		pages.Count(), page2.Start+1, page2.End, len([]rune(doc))))

	res2, err := s.call(map[string]any{"dictamen_id": "E100N26", "part": 2})
	s.Require().NoError(err)
	s.Equal(text, res2.Content[0].(*mcp.TextContent).Text, "pagination is deterministic")

	// No overlap with part 1 and no loss across the concatenation.
	p1, err := s.call(map[string]any{"dictamen_id": "E100N26"})
	s.Require().NoError(err)
	p1sc := p1.StructuredContent.(map[string]any)
	p2sc := res.StructuredContent.(map[string]any)
	s.Equal(p1sc["char_end"], p2sc["char_start"], "part 2 starts exactly where part 1 ended")
}

func (s *CgrDictamenPaginationSuite) TestLastPartHasNoContinuationSignal() {
	s.expectE100()
	last := cgr.PaginateStandard(s.multiPartDictamen().Documento).Count()
	res, err := s.call(map[string]any{"dictamen_id": "E100N26", "part": last})
	s.Require().NoError(err)
	text := res.Content[0].(*mcp.TextContent).Text
	s.NotContains(text, "continue with part=", "the last part offers no continuation")
}

func (s *CgrDictamenPaginationSuite) TestPartArgumentErrors() {
	// part < 1 fails WITHOUT calling the client.
	res, err := s.call(map[string]any{"dictamen_id": "E100N26", "part": -2})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "part must be 1 or greater")

	// part beyond the total fails AFTER the fetch, stating the range.
	s.expectE100()
	res, err = s.call(map[string]any{"dictamen_id": "E100N26", "part": 999})
	s.Require().NoError(err)
	s.True(res.IsError)
	pages := cgr.PaginateStandard(s.multiPartDictamen().Documento)
	s.Contains(res.Content[0].(*mcp.TextContent).Text,
		fmt.Sprintf("part must be between 1 and %d", pages.Count()))

	// Missing dictamen_id is rejected by the input schema validation
	// before the handler runs (required property).
	res, err = s.call(map[string]any{"part": 1})
	s.Require().NoError(err)
	s.True(res.IsError)
	s.Contains(res.Content[0].(*mcp.TextContent).Text, "dictamen_id")
}

func (s *CgrDictamenPaginationSuite) TestEmptyDocumentStillOnePart() {
	s.cgrClient.EXPECT().GetDictamen(mock.Anything, "E200N26").Return(cgr.DictamenFull{
		DictamenSummary: cgr.DictamenSummary{DictamenID: "E200N26"},
		Documento:       "",
	}, nil).Once()

	res, err := s.call(map[string]any{"dictamen_id": "E200N26"})
	s.Require().NoError(err)
	s.False(res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	s.Contains(text, "Documento sin contenido")
	s.Contains(text, "part 1 of 1")
}

func (s *CgrDictamenPaginationSuite) TestCompletenessInvariantsEveryPart() {
	s.expectE100()
	// Walk EVERY part of the multi-part document and assert the three
	// invariants: range signal present, whole-document char_count, and
	// exact continuation chaining (no gap, no overlap).
	pages := cgr.PaginateStandard(s.multiPartDictamen().Documento)
	total := s.multiPartDictamen().CharCount
	var lastEnd float64
	for p := 1; p <= pages.Count(); p++ {
		res, err := s.call(map[string]any{"dictamen_id": "E100N26", "part": p})
		s.Require().NoError(err, "part %d", p)
		text := res.Content[0].(*mcp.TextContent).Text
		sc := res.StructuredContent.(map[string]any)

		s.Contains(text, fmt.Sprintf("part %d of %d", p, pages.Count()), "range signal on part %d", p)
		s.Equal(float64(total), sc["char_count"], "char_count is the total on part %d", p)

		page := pages.Range(p - 1)
		s.Equal(float64(page.Start), sc["char_start"])
		s.Equal(float64(page.End), sc["char_end"])
		if p > 1 {
			s.Equal(lastEnd, sc["char_start"], "parts chain without gaps")
		}
		lastEnd = float64(page.End)
	}
	s.Equal(len([]rune(s.multiPartDictamen().Documento)), int(lastEnd), "the parts cover the whole document")
}
