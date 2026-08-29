package bcn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// NormSearchSuite validates the pure intra-norm search: two-surface
// matching (names first, then content), determinism, snippet extraction
// and the literal (never regex) query contract.
type NormSearchSuite struct {
	suite.Suite
	norma NormaFull
}

func TestNormSearchSuite(t *testing.T) {
	suite.Run(t, new(NormSearchSuite))
}

func (s *NormSearchSuite) SetupTest() {
	s.norma = NormaFull{
		Html: []HtmlBlock{
			{I: 1, SectionName: "Encabezado", Markdown: "Encabezado de la ley de prueba."},
			{
				I: 2, SectionName: "TÍTULO I      DISPOSICIONES GENERALES",
				H: []HtmlBlock{
					{I: 3, SectionName: "Artículo 1º", Markdown: "Habilita el uso de la sociedad conyugal como regimen patrimonial."},
					{I: 4, SectionName: "Artículo 2º", Markdown: "Definiciones generales de la ley."},
				},
			},
			{
				I: 5, SectionName: "TÍTULO II",
				H: []HtmlBlock{
					{I: 6, SectionName: "Artículo 3º", Markdown: strings.Repeat("Relleno previo. ", 40) + "Aqui habla de la SOCIEDAD CONYUGAL en detalle amplio. " + strings.Repeat("Relleno posterior. ", 40)},
				},
			},
		},
		Estructura: []EstructuraPart{
			{N: "Encabezado", I: 1},
			{N: "TÍTULO I      DISPOSICIONES GENERALES", I: 2, T: 1, H: []EstructuraPart{
				{N: "Artículo 1º", I: 3, T: 6},
				{N: "Artículo 2º", I: 4, T: 6},
			}},
			{N: "TÍTULO II", I: 5, T: 1, H: []EstructuraPart{
				{N: "Artículo 3º", I: 6, T: 6},
			}},
		},
	}
}

func (s *NormSearchSuite) TestNameMatchRanksAboveContentMatch() {
	// "generales" matches the NAME of TÍTULO I and the CONTENT of
	// Artículo 2º: the name match ranks first.
	res := searchNorma(s.norma, "generales")
	s.Require().Len(res.Matches, 2)
	s.Equal(int64(2), res.Matches[0].SectionID, "name match ranks first")
	s.Empty(res.Matches[0].Snippet)
	s.Equal(int64(4), res.Matches[1].SectionID, "content match follows in document order")
	s.NotEmpty(res.Matches[1].Snippet)
}

func (s *NormSearchSuite) TestNodeMatchingBothSurfacesListedOnce() {
	// "DISPOSICIONES GENERALES" matches the name of TÍTULO I and (as
	// content) its whole subtree — the título is a single name match.
	res := searchNorma(s.norma, "DISPOSICIONES GENERALES")
	s.Require().Len(res.Matches, 1)
	s.Equal(int64(2), res.Matches[0].SectionID)
	s.Empty(res.Matches[0].Snippet, "name matches carry no snippet")
}

func (s *NormSearchSuite) TestContainerSectionsMatchBySubtreeContent() {
	// "conyugal" hits no name: the containers whose SUBTREE carries it
	// (TÍTULO I and TÍTULO II) match as content, in document order,
	// interleaved with their matching articles.
	res := searchNorma(s.norma, "conyugal")
	var ids []int64
	for _, m := range res.Matches {
		ids = append(ids, m.SectionID)
	}
	s.Equal([]int64{2, 3, 5, 6}, ids)
}

func (s *NormSearchSuite) TestCaseInsensitiveAndWhitespaceCollapsedQuery() {
	// Wide-gap name matches a single-spaced query, case-insensitively.
	res := searchNorma(s.norma, "título i   disposiciones")
	s.Require().Len(res.Matches, 1)
	s.Equal(int64(2), res.Matches[0].SectionID)
}

func (s *NormSearchSuite) TestContentMatchCarriesSnippetAroundFirstOccurrence() {
	res := searchNorma(s.norma, "sociedad conyugal")

	var withSnippet NormaMatch
	for _, m := range res.Matches {
		if m.SectionID == 6 && m.Snippet != "" {
			withSnippet = m
		}
	}
	s.Require().NotZero(withSnippet.SectionID, "content match on Artículo 3º carries a snippet")
	snip := withSnippet.Snippet
	s.Contains(strings.ToLower(snip), "sociedad conyugal")
	s.LessOrEqual(len([]rune(snip)), snippetMaxHard, "snippet stays bounded")
}

func (s *NormSearchSuite) TestQueryIsLiteralNeverRegex() {
	// "art. 1.*" as regex would match "art. 1º..." broadly; as literal
	// text it must find nothing in this fixture (no such substring).
	res := searchNorma(s.norma, "art. 1.*")
	s.Empty(res.Matches)
	s.Equal(6, res.Walked)
}

func (s *NormSearchSuite) TestNoMatchesReportsWalkedSections() {
	res := searchNorma(s.norma, "inexistente total")
	s.Empty(res.Matches)
	s.Equal(6, res.Walked, "walked counts every structure section")

	res = searchNorma(s.norma, "   ")
	s.Empty(res.Matches, "whitespace-only query is a no-op")
	s.Zero(res.Walked)
}

func (s *NormSearchSuite) TestCountsDescribeTheMatchedSection() {
	res := searchNorma(s.norma, "conyugal")

	var titulo NormaMatch
	for _, m := range res.Matches {
		if m.SectionID == 2 {
			titulo = m
		}
	}
	s.Equal(2, titulo.ArticleCount, "TÍTULO I carries its two articles")
	s.Positive(titulo.CharCount)

	byID := map[int64]NormaMatch{}
	for _, m := range res.Matches {
		byID[m.SectionID] = m
	}
	s.Equal(1, byID[3].ArticleCount, "a leaf article counts itself")
}

func (s *NormSearchSuite) TestDeterministic() {
	a := searchNorma(s.norma, "conyugal")
	b := searchNorma(s.norma, "conyugal")
	s.Equal(a, b)
}

func (s *NormSearchSuite) TestSnippetParagraphRounding() {
	// A short paragraph around the occurrence arrives whole.
	body := "Primer párrafo.\n\nSegundo párrafo con la ocurrencia CLAVE aqui.\n\nTercer párrafo."
	idx := strings.Index(strings.ToLower(body), "clave")
	snip := snippetAround(body, idx, len("clave"))
	// Short body: the window covers it all — snippet is the whole body.
	s.Equal(body, snip)

	// Long paragraphs: the end window lands mid-paragraph and rounds
	// OUTWARD to the paragraph break after it.
	p1 := strings.Repeat("alpha ", 20)
	p2 := strings.Repeat("beta ", 20) + "OCURRENCIA " + strings.Repeat("gamma ", 10)
	p3 := strings.Repeat("delta ", 40)
	long3 := p1 + "\n\n" + p2 + "\n\n" + p3
	idx3 := strings.Index(long3, "OCURRENCIA")
	snip3 := snippetAround(long3, idx3, len("OCURRENCIA"))
	s.True(strings.HasSuffix(snip3, strings.TrimSpace(p2)) || strings.Contains(snip3, "gamma"),
		"the paragraph carrying the occurrence arrives uncut")

	// A long paragraph gets a bounded window, never the whole body.
	long := strings.Repeat("palabra ", 500)
	idx2 := strings.Index(long, "palabra")
	snip2 := snippetAround(long, idx2, len("palabra"))
	s.LessOrEqual(len([]rune(snip2)), snippetMaxHard)
	s.Contains(snip2, "palabra")
}
