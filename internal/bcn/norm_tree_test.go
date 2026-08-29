package bcn

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

// NormTreeSuite validates the tree operations (section slice, article and
// char counts, structure flattening) against the real Ley 21.600 fixture.
type NormTreeSuite struct {
	suite.Suite
	norma NormaFull
}

func TestNormTreeSuite(t *testing.T) {
	suite.Run(t, new(NormTreeSuite))
}

func (s *NormTreeSuite) SetupTest() {
	data, err := os.ReadFile("testdata/norma_full.json")
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(data, &s.norma))
	s.norma.ConvertContent(newConverter())
}

func (s *NormTreeSuite) TestSectionContentReturnsSubtree() {
	tituloID := s.norma.Html[1].I // TÍTULO I, which nests 3 articles
	subtree, ok := s.norma.SectionContent(tituloID)
	s.Require().True(ok)
	s.Require().Len(subtree, 1)
	s.Equal(tituloID, subtree[0].I)
	s.Require().NotEmpty(subtree[0].H, "the subtree must keep the nested articles")

	// A nested article resolves too.
	articleID := subtree[0].H[0].I
	articleSubtree, ok := s.norma.SectionContent(articleID)
	s.Require().True(ok)
	s.Equal(articleID, articleSubtree[0].I)

	_, ok = s.norma.SectionContent(999999999)
	s.False(ok, "unknown section ids must not resolve")
}

func (s *NormTreeSuite) TestCountArticles() {
	total := s.norma.CountArticles()
	s.Positive(total)
	s.Equal(total, s.norma.CountSectionArticles(0), "partID 0 means the whole structure")

	tituloID := s.norma.Html[1].I
	s.Equal(3, s.norma.CountSectionArticles(tituloID), "TÍTULO I carries 3 articles")

	articleID := s.norma.Html[1].H[0].I
	s.Equal(1, s.norma.CountSectionArticles(articleID), "a leaf article counts itself")

	s.Zero(s.norma.CountSectionArticles(999999999))
}

func (s *NormTreeSuite) TestContentCharCount() {
	full := ContentCharCount(s.norma.Html)
	s.Positive(full)

	tituloID := s.norma.Html[1].I
	subtree, ok := s.norma.SectionContent(tituloID)
	s.Require().True(ok)
	s.Less(ContentCharCount(subtree), full, "a section subtree must count less than the whole norm")
}

func (s *NormTreeSuite) TestFlattenStructureKeepsOrderAndDepth() {
	flat := FlattenStructure(s.norma.Estructura)
	s.Require().NotEmpty(flat)
	s.Equal(s.norma.Estructura[0].N, flat[0].Name)
	s.Equal(0, flat[0].Depth)

	// The fixture nests articles under titles: some entry must carry depth > 0.
	hasNested := false
	for _, p := range flat {
		if p.Depth > 0 {
			hasNested = true
			break
		}
	}
	s.True(hasNested, "nested structure entries must be flattened with depth")
}

func (s *NormTreeSuite) TestContentSizesMirrorCharCount() {
	sizes := ContentSizes(s.norma.Html)
	s.Require().NotEmpty(sizes)

	// Every top-level block's subtree size equals the rendered char count
	// of that subtree — the fold renderer depends on this equality.
	for _, block := range s.norma.Html {
		subtree := []HtmlBlock{block}
		s.Equal(ContentCharCount(subtree), sizes[block.I],
			"subtree size for block %d must mirror the renderer count", block.I)
	}

	// Nested articles are mapped too.
	articleID := s.norma.Html[1].H[0].I
	articleSubtree, ok := s.norma.SectionContent(articleID)
	s.Require().True(ok)
	s.Equal(ContentCharCount(articleSubtree), sizes[articleID])

	// Unknown ids are absent from the map.
	_, ok = sizes[999999999]
	s.False(ok)
}

func (s *NormTreeSuite) TestSectionStructureReturnsSubtree() {
	tituloID := s.norma.Estructura[1].I
	subtree, ok := s.norma.SectionStructure(tituloID)
	s.Require().True(ok)
	s.Require().Len(subtree, 1)
	s.Equal(tituloID, subtree[0].I)
	s.Require().NotEmpty(subtree[0].H, "the structure subtree must keep nested parts")

	// The flattened subtree matches the whole flatten restricted to the run.
	full := FlattenStructure(s.norma.Estructura)
	sub := FlattenStructure(subtree)
	s.Equal(sub[0].Name, full[1].Name, "the subtree root is the requested part")
	s.Less(len(sub), len(full))

	_, ok = s.norma.SectionStructure(999999999)
	s.False(ok, "unknown section ids must not resolve")
}
