// Linear pagination for flat CGR documents (output-budget capability):
// budget-sized parts with paragraph-rounded boundaries, deterministic by
// construction — the same document always yields the same parts.
package cgr

import (
	"strings"

	"github.com/alvarosdev/lex-chile-mcp/internal/outputbudget"
)

const (
	// paragraphMargin bounds how far a cut may extend to reach the next
	// paragraph break; a document without paragraph breaks still paginates.
	paragraphMargin = 2_000
	// paragraphSep splits legal documents into readable parts.
	paragraphSep = "\n\n"
)

// Page is one [Start,End) rune range of the paginated document.
type Page struct {
	Start int
	End   int
}

// Pages is the pagination of one document. Immutable after Paginate.
type Pages struct {
	parts []Page
}

// Paginate splits doc into parts of at most budget runes. A cut that
// would fall inside a paragraph moves forward to the next "\n\n" break
// when one lies within paragraphMargin runes; otherwise the cut stands
// (a pathological break-free document cannot produce unbounded parts).
// Pure: identical inputs produce identical parts, so part N of one call
// continues exactly where part N-1 of another ended.
func Paginate(doc string, budget int) *Pages {
	p := &Pages{}
	if doc == "" {
		p.parts = []Page{{Start: 0, End: 0}}
		return p
	}
	runes := []rune(doc)
	if budget <= 0 {
		budget = outputbudget.MaxContentChars
	}
	for start := 0; start < len(runes); {
		end := start + budget
		if end >= len(runes) {
			end = len(runes)
		} else {
			end = roundToParagraph(runes, end)
		}
		p.parts = append(p.parts, Page{Start: start, End: end})
		start = end
	}
	return p
}

// PaginateStandard paginates with the shared server output budget
// (outputbudget.MaxContentChars).
func PaginateStandard(doc string) *Pages {
	return Paginate(doc, outputbudget.MaxContentChars)
}

// roundToParagraph moves end forward to just past the next "\n\n" when
// one lies within the margin; otherwise returns end unchanged.
func roundToParagraph(runes []rune, end int) int {
	limit := end + paragraphMargin
	if limit > len(runes) {
		limit = len(runes)
	}
	for i := end; i < limit-1; i++ {
		if runes[i] == '\n' && runes[i+1] == '\n' {
			return i + 2
		}
	}
	return end
}

// Count returns the number of parts (always ≥ 1).
func (p *Pages) Count() int {
	return len(p.parts)
}

// Slice returns the text of the 0-indexed part i.
func (p *Pages) Slice(doc string, i int) string {
	return string([]rune(doc)[p.parts[i].Start:p.parts[i].End])
}

// Range returns the [Start,End) rune offsets of the 0-indexed part.
func (p *Pages) Range(i int) Page {
	return p.parts[i]
}

// TrimSpaceParagraph is a small helper for display: single-blank-line
// separation of the sliced part.
func (p *Pages) TrimSpaceParagraph(doc string, i int) string {
	return strings.TrimSpace(p.Slice(doc, i))
}
