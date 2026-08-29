package bcn

import (
	"context"
	"strings"
)

// NormaMatch is one section of a norm that matched an intra-norm search:
// the retrieval identifier the agent needs to fetch content with
// get_law(section_id=...), the section's real size and article count, and
// — for content matches — a context snippet around the first occurrence.
type NormaMatch struct {
	Name         string `json:"name"`
	SectionID    int64  `json:"section_id"`
	CharCount    int    `json:"char_count"`
	ArticleCount int    `json:"article_count"`
	Snippet      string `json:"snippet,omitempty"`
}

// NormaSearchResult carries the matches of an intra-norm search plus the
// number of structure sections walked (diagnostic for the no-match
// message required by the find-in-norm spec).
type NormaSearchResult struct {
	Matches []NormaMatch
	Walked  int
}

// SearchNorma searches inside one norm for a plain-text query: case-
// insensitive substring over two surfaces — section names (whitespace
// collapsed) and the content markdown of each section's subtree — with
// name matches ranked above content matches, document order within each
// group. The query is LITERAL text: never a regular expression. Executes
// in memory over the cached norm (ETag revalidation via GetNorma); no
// HTTP beyond fetching the norm itself.
func (c *Client) SearchNorma(ctx context.Context, q NormaQuery, query string) (NormaSearchResult, error) {
	norma, err := c.GetNorma(ctx, q)
	if err != nil {
		return NormaSearchResult{}, err
	}
	return searchNorma(norma, query), nil
}

// searchNorma is the pure search over a converted norm. Deterministic:
// the walk is the structure pre-order, so the same norm and query always
// produce the same result.
//
// Content containment is checked per markdown piece (each block's own
// body, pre-order) rather than on the assembled subtree render: a query
// spanning a heading/body boundary is not a content match (the heading
// text is the name surface, matched separately). This avoids rendering
// every subtree (quadratic string building on meganorms) while matching
// exactly the text the agent reads in the bodies.
func searchNorma(norma NormaFull, query string) NormaSearchResult {
	q := strings.ToLower(collapseQuerySpaces(query))
	if q == "" {
		return NormaSearchResult{}
	}

	sizes := ContentSizes(norma.Html)
	blocks := indexBlocksByID(norma.Html)

	result := NormaSearchResult{Matches: []NormaMatch{}}
	var nameMatches, contentMatches []NormaMatch

	var walkParts func(parts []EstructuraPart)
	walkParts = func(parts []EstructuraPart) {
		for i := range parts {
			part := &parts[i]
			result.Walked++

			match := NormaMatch{
				Name:         strings.Join(strings.Fields(part.N), " "),
				SectionID:    part.I,
				CharCount:    sizes[part.I],
				ArticleCount: countArticlesInPart(part),
			}

			if strings.Contains(strings.ToLower(match.Name), q) {
				nameMatches = append(nameMatches, match)
			} else if block, ok := blocks[part.I]; ok {
				if piece, idx, found := firstContaining(block, q); found {
					match.Snippet = snippetAround(piece, idx, len(q))
					contentMatches = append(contentMatches, match)
				}
			}
			walkParts(part.H)
		}
	}
	walkParts(norma.Estructura)

	// Deterministic ranking: name matches first, then content matches,
	// document order within each group. A node matching both surfaces is
	// listed once, as a name match.
	result.Matches = append(result.Matches, nameMatches...)
	result.Matches = append(result.Matches, contentMatches...)
	return result
}

// collapseQuerySpaces normalizes a query the same way section names are
// normalized for matching (wide gaps collapse; "Título VII  DE LA" must
// match "Título VII      DE LA FILIACIÓN").
func collapseQuerySpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// indexBlocksByID indexes every html block (recursively) by its id — the
// structure and the content trees share ids (section_id addresses both).
func indexBlocksByID(blocks []HtmlBlock) map[int64]*HtmlBlock {
	index := make(map[int64]*HtmlBlock)
	var walk func(blocks []HtmlBlock)
	walk = func(blocks []HtmlBlock) {
		for i := range blocks {
			index[blocks[i].I] = &blocks[i]
			walk(blocks[i].H)
		}
	}
	walk(blocks)
	return index
}

// firstContaining returns the first (pre-order) markdown piece of the
// block subtree that contains the lowercased query, with the occurrence
// index inside that piece. Containment is case-insensitive: the piece is
// lowered once per candidate piece.
func firstContaining(block *HtmlBlock, q string) (piece string, idx int, found bool) {
	var walk func(b *HtmlBlock) bool
	walk = func(b *HtmlBlock) bool {
		lower := strings.ToLower(b.Markdown)
		if i := strings.Index(lower, q); i >= 0 {
			piece, idx, found = b.Markdown, i, true
			return true
		}
		for i := range b.H {
			if walk(&b.H[i]) {
				return true
			}
		}
		return false
	}
	walk(block)
	return piece, idx, found
}

// Snippet window: ~200 chars around the first occurrence, rounded
// OUTWARD to paragraph breaks when they sit within a small margin, so a
// short paragraph arrives whole. No highlight markers — clean text.
const (
	snippetBefore  = 70
	snippetAfter   = 130
	snippetMargin  = 150 // max distance to a paragraph break for rounding
	snippetMaxHard = 600 // hard cap even after paragraph rounding
)

// snippetAround cuts a context window around body[idx:idx+qlen] and
// rounds the edges to \n\n paragraph boundaries within the margin.
func snippetAround(body string, idx, qlen int) string {
	runes := []rune(body)
	start := idx - snippetBefore
	if start < 0 {
		start = 0
	}
	end := idx + qlen + snippetAfter
	if end > len(runes) {
		end = len(runes)
	}

	// Round outward to paragraph breaks within the margin.
	if b := lastIndexParagraph(runes, start, snippetMargin); b >= 0 {
		start = b
	}
	if b := indexParagraph(runes, end, snippetMargin); b >= 0 {
		end = b
	}
	if end-start > snippetMaxHard {
		end = start + snippetMaxHard
	}
	if end > len(runes) {
		end = len(runes)
	}
	return strings.TrimSpace(string(runes[start:end]))
}

// lastIndexParagraph finds the offset just after the last "\n\n" that
// ends at or before from-margin; -1 when none lies within the margin.
func lastIndexParagraph(runes []rune, from, margin int) int {
	limit := from - margin
	if limit < 0 {
		limit = 0
	}
	for i := from; i > limit+1; i-- {
		if runes[i-1] == '\n' && runes[i-2] == '\n' {
			return i
		}
	}
	return -1
}

// indexParagraph finds the offset of the first "\n\n" at or after from
// within the margin; -1 when none lies within the margin.
func indexParagraph(runes []rune, from, margin int) int {
	limit := from + margin
	if limit > len(runes) {
		limit = len(runes)
	}
	for i := from; i < limit-1; i++ {
		if runes[i] == '\n' && runes[i+1] == '\n' {
			return i
		}
	}
	return -1
}

// countArticlesInPart counts the artículo entries (T == 6) of the
// structure subtree rooted at part, including the part itself.
func countArticlesInPart(part *EstructuraPart) int {
	count := 0
	if part.T == 6 {
		count++
	}
	for i := range part.H {
		count += countArticlesInPart(&part.H[i])
	}
	return count
}
