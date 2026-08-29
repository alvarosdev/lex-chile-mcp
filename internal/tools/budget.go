package tools

import (
	"fmt"
	"strings"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
	"github.com/alvarosdev/lex-chile-mcp/internal/outputbudget"
)

// Output budget for content-bearing tool responses (output-budget spec).
// A rendered content body larger than this degrades to a navigable map
// instead of being delivered (never truncated silently). ~100K chars is
// ~25K tokens — deliberately generous for mid-size norms, decisive for
// meganorms (the Código Civil renders ~1.28M chars and does not fit any
// mainstream context).
const maxContentChars = outputbudget.MaxContentChars

// maxListedChildren bounds how many direct children a folded-TOC node may
// list before collapsing them into a single ranged line. Tunable; the
// char budget above stays the hard backstop.
const maxListedChildren = 20

// maxRelatedNorms caps the Related norms header line; the structured
// output keeps the complete list.
const maxRelatedNorms = 10

// maxTOCChars bounds the folded table of contents itself. Deep-narrow
// structures (the Código Civil nests books→titles→§→articles under a
// refunded double articulation, with no node over 20 direct children)
// still explode the by-children fold, so a TOC over this size re-renders
// as a compact map: one line per top-level container plus one ranged line
// per direct child. ~12K chars ≈ 3K tokens.
const maxTOCChars = 12_000

// withinBudget reports whether the rendered content of blocks fits the
// output budget. It mirrors the renderer size via bcn.ContentCharCount.
func withinBudget(blocks []bcn.HtmlBlock) bool {
	return bcn.ContentCharCount(blocks) <= maxContentChars
}

// collapseSpaces reduces runs of internal whitespace to single spaces and
// trims the result — structure names arrive with wide gaps
// ("Título VII      DE LA FILIACIÓN").
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// renderFoldedTOC renders the adaptive folded table of contents of a
// flattened structure (depth-annotated, document order):
//
//   - a node with at most maxListedChildren direct children lists them
//     (name · section_id · ~size) and recurses;
//   - a node with more direct children collapses into one line with its
//     textual article range (first–last label, not numeric — "58 BIS",
//     "TRANSITORIO" and "FINAL" exist), size and article count. The range
//     is the localization mechanism: an agent matches a target article
//     against container ranges and drills one level down.
//
// The TOC itself is budgeted: deep-narrow structures where no node breaks
// the children limit would still explode (the Código Civil nests
// book→title→§→article with every node under the limit), so a fold that
// exceeds maxTOCChars re-renders as a COMPACT MAP — one line per
// top-level container plus one ranged line per direct child. Drill-down
func renderFoldedTOC(b *strings.Builder, flat []bcn.StructurePartOut, sizes map[int64]int) {
	renderTOCBounded(b, flat, sizes, 0, len(flat))
}

// renderTOCBounded renders the node range [from,to) through the
// degradation chain, bounded by the TOC budget:
//
//  1. folded TOC (children listed up to maxListedChildren, collapsed
//     ranges above);
//  2. over maxTOCChars → compact map: one line per top-level container
//     plus one ranged line per direct child;
//  3. still over → top-level lines only.
//
// Each level keeps every section id reachable — drill-down is always one
// section call away.
func renderTOCBounded(b *strings.Builder, flat []bcn.StructurePartOut, sizes map[int64]int, from, to int) {
	var folded strings.Builder
	for i := from; i < to; {
		i = foldNode(&folded, flat, i, sizes, 0)
	}
	if folded.Len() <= maxTOCChars {
		b.WriteString(folded.String())
		return
	}
	var compact strings.Builder
	for i := from; i < to; {
		i = compactNode(&compact, flat, i, sizes, 0)
	}
	if compact.Len() <= maxTOCChars {
		b.WriteString(compact.String())
		return
	}
	for i := from; i < to; {
		i = topLine(b, flat, i, sizes)
	}
}

// topLine renders just the node's own line and returns the exclusive end
// of its run — the last degradation step of the bounded TOC.
func topLine(b *strings.Builder, flat []bcn.StructurePartOut, i int, sizes map[int64]int) int {
	end := runEnd(flat, i)
	part := flat[i]
	fmt.Fprintf(b, "- %s | section_id: %d | ~%s", collapseSpaces(part.Name), part.ID, humanCount(sizes[part.ID]))
	if first, last, arts := articleBounds(flat, i+1, end); arts > 0 {
		fmt.Fprintf(b, " | arts. %s–%s · %s", first, last, formatArticles(arts))
	}
	b.WriteByte('\n')
	return end
}

// renderFoldedSubTOC renders the folded TOC of the direct children of
// sectionID — the local drill-down context of a section response, bounded
// by the same TOC budget chain as the global map. It returns false when
// the section has no children (leaf) or is unknown.
func renderFoldedSubTOC(b *strings.Builder, flat []bcn.StructurePartOut, sizes map[int64]int, sectionID int64) bool {
	i, ok := findPart(flat, sectionID)
	if !ok {
		return false
	}
	end := runEnd(flat, i)
	if i+1 >= end {
		return false // leaf section: nothing to list
	}
	renderTOCBounded(b, flat, sizes, i+1, end)
	return true
}

// compactNode renders one top-level entry of the compact map: its own
// line plus ONE collapsed ranged line per direct child (grandchildren
// absorbed by the range). Returns the exclusive end of the node's run.
func compactNode(b *strings.Builder, flat []bcn.StructurePartOut, i int, sizes map[int64]int, depth int) int {
	end := runEnd(flat, i)
	part := flat[i]
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(b, "%s- %s · section_id: %d · %s\n", indent, collapseSpaces(part.Name), part.ID, humanCount(sizes[part.ID]))
	for j := i + 1; j < end; {
		end2 := runEnd(flat, j)
		child := flat[j]
		line := fmt.Sprintf("%s  - %s | section_id: %d | ~%s", indent, collapseSpaces(child.Name), child.ID, humanCount(sizes[child.ID]))
		if first, last, arts := articleBounds(flat, j+1, end2); arts > 0 {
			line += fmt.Sprintf(" | arts. %s–%s · %s", first, last, formatArticles(arts))
		}
		b.WriteString(line)
		b.WriteByte('\n')
		j = end2
	}
	return end
}

// foldNode renders the node at index i (and, when expanded, its subtree)
// and returns the exclusive end index of the node's run.
func foldNode(b *strings.Builder, flat []bcn.StructurePartOut, i int, sizes map[int64]int, depth int) int {
	end := runEnd(flat, i)
	part := flat[i]
	name := collapseSpaces(part.Name)
	indent := strings.Repeat("  ", depth)

	// sizeTag renders " · <size>" — omitted when the size is unknown
	// (zero), so a structure without matching content blocks does not
	// print noise.
	sizeTag := func(id int64) string {
		if n := sizes[id]; n > 0 {
			return " · " + humanCount(n)
		}
		return ""
	}

	switch {
	case i+1 >= end:
		// Leaf: article or childless container.
		fmt.Fprintf(b, "%s- %s · section_id: %d%s\n", indent, name, part.ID, sizeTag(part.ID))
	case countDirectChildren(flat, i, end) <= maxListedChildren:
		fmt.Fprintf(b, "%s- %s · section_id: %d%s\n", indent, name, part.ID, sizeTag(part.ID))
		for j := i + 1; j < end; {
			j = foldNode(b, flat, j, sizes, depth+1)
		}
	default:
		line := fmt.Sprintf("%s- %s | section_id: %d | ~%s", indent, name, part.ID, humanCount(sizes[part.ID]))
		if first, last, arts := articleBounds(flat, i+1, end); arts > 0 {
			line += fmt.Sprintf(" | arts. %s–%s · %s", first, last, formatArticles(arts))
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return end
}

// runEnd returns the exclusive index where the subtree starting at i ends:
// the first subsequent entry with depth <= flat[i].Depth (flattened
// structures nest without depth skips — see bcn.FlattenStructure).
func runEnd(flat []bcn.StructurePartOut, i int) int {
	depth := flat[i].Depth
	for j := i + 1; j < len(flat); j++ {
		if flat[j].Depth <= depth {
			return j
		}
	}
	return len(flat)
}

// findPart returns the index of the entry with the given id.
func findPart(flat []bcn.StructurePartOut, id int64) (int, bool) {
	for i := range flat {
		if flat[i].ID == id {
			return i, true
		}
	}
	return 0, false
}

// countDirectChildren counts the entries one level below flat[i] inside
// its run (i+1..end).
func countDirectChildren(flat []bcn.StructurePartOut, i, end int) int {
	n := 0
	for j := i + 1; j < end; j++ {
		if flat[j].Depth == flat[i].Depth+1 {
			n++
		}
	}
	return n
}

// articleBounds returns the labels of the first and last article entries
// (Type == 6, any depth) inside [i,end), in document order, and how many
// there are. Textual labels — not numbers — because "58 BIS" et al. exist.
func articleBounds(flat []bcn.StructurePartOut, i, end int) (first, last string, count int) {
	for j := i; j < end; j++ {
		if flat[j].Type == 6 {
			if count == 0 {
				first = collapseSpaces(flat[j].Name)
			}
			last = collapseSpaces(flat[j].Name)
			count++
		}
	}
	return first, last, count
}
