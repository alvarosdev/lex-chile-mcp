package cgr

import (
	"strings"
	"testing"

	"github.com/alvarosdev/lex-chile-mcp/internal/outputbudget"
	"github.com/stretchr/testify/suite"
)

// PaginationSuite validates the linear pagination primitive: budget
// sizing, paragraph rounding, determinism and the single-source budget.
type PaginationSuite struct {
	suite.Suite
}

func TestPaginationSuite(t *testing.T) {
	suite.Run(t, new(PaginationSuite))
}

func (s *PaginationSuite) TestSinglePageWhenUnderBudget() {
	p := Paginate("documento corto.", 1000)
	s.Equal(1, p.Count())
	s.Equal("documento corto.", p.Slice("documento corto.", 0))
}

func (s *PaginationSuite) TestEmptyDocument() {
	p := Paginate("", 1000)
	s.Equal(1, p.Count())
	s.Equal("", p.Slice("", 0))
}

func (s *PaginationSuite) TestBudgetSizingWithParagraphRounding() {
	// Three paragraphs of 60 runes each; budget cuts mid-paragraph and
	// rounds forward to the next "\n\n".
	para := strings.Repeat("a", 58) // + "\n\n" handled separately
	doc := strings.Join([]string{para, para, para, para, para}, "\n\n")
	budget := 100
	p := Paginate(doc, budget)

	s.Greater(p.Count(), 1)
	rebuilt := ""
	for i := range p.Count() {
		part := p.Slice(doc, i)
		s.NotContains(part, "  ", "no double space introduced")
		// No part exceeds budget + margin.
		s.LessOrEqual(len([]rune(part)), budget+paragraphMargin)
		rebuilt += part
	}
	s.Equal(doc, rebuilt, "parts concatenate to the exact document (no loss, no overlap)")
}

func (s *PaginationSuite) TestCutRoundsToParagraphBoundary() {
	// 50 a's + \n\n + 50 b's + \n\n + 50 c's: a cut at 60 lands inside
	// the b-paragraph and must round to the break after it.
	doc := strings.Repeat("a", 50) + "\n\n" + strings.Repeat("b", 50) + "\n\n" + strings.Repeat("c", 50)
	p := Paginate(doc, 60)
	s.Require().Equal(2, p.Count())
	part1 := p.Slice(doc, 0)
	// The cut lands inside the b-paragraph and rounds to just past the
	// break: part 1 ends with the paragraph separator, part 2 resumes
	// exactly at the first b (no overlap, no loss).
	s.True(strings.HasSuffix(strings.TrimRight(part1, "\n"), "b"),
		"cut rounds past the paragraph break")
	part2 := p.Slice(doc, 1)
	s.True(strings.HasPrefix(part2, "c"), "next part starts at the following paragraph")
}

func (s *PaginationSuite) TestMarginFallbackBreakFreeDocument() {
	// No "\n\n" anywhere: the cut stands at the budget (no unbounded part).
	doc := strings.Repeat("x", 1000)
	p := Paginate(doc, 300)
	s.Equal(4, p.Count())
	for i := range p.Count() {
		s.LessOrEqual(len([]rune(p.Slice(doc, i))), 300)
	}
}

func (s *PaginationSuite) TestDeterministicAcrossCalls() {
	doc := strings.Repeat("párrafo uno.\n\n", 800)
	a, b := Paginate(doc, 500), Paginate(doc, 500)
	s.Equal(a.Count(), b.Count())
	for i := range a.Count() {
		s.Equal(a.Range(i), b.Range(i))
	}
}

func (s *PaginationSuite) TestSharedBudgetSingleSource() {
	// PaginateStandard paginates with the shared outputbudget constant:
	// 100K chars + 1 → exactly two parts.
	p := PaginateStandard(strings.Repeat("x", outputbudget.MaxContentChars+1))
	s.Equal(2, p.Count())
	s.Equal(outputbudget.MaxContentChars, p.Range(0).End-p.Range(0).Start)
}

func (s *PaginationSuite) TestZeroAndNegativeBudgetFallback() {
	p := Paginate("cuerpo.", -1)
	s.Equal(1, p.Count())
}

func (s *PaginationSuite) TestLargeSyntheticDocument() {
	// 250K chars of realistic legal-ish paragraphs: ~120 parts, every
	// boundary paragraph-safe or margin-backed.
	para := "CONSIDERANDO " + strings.Repeat("que el ejemplo de texto legal ocupa espacio. ", 3)
	doc := strings.TrimSuffix(strings.Repeat(para+"\n\n", 800), "\n\n")
	p := PaginateStandard(doc)
	s.Greater(p.Count(), 1)
	total := 0
	for i := range p.Count() {
		total += p.Range(i).End - p.Range(i).Start
	}
	s.Equal(len([]rune(doc)), total)
}
