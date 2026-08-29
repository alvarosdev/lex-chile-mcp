package bcn

import (
	"fmt"
	"strings"
)

// StructurePartOut is one flattened entry of the nested norm structure.
// The API tree is recursive; outputs flatten it with a depth field so the
// JSON schema stays acyclic (a recursive type in an Output panics the
// schema generator) and consumers can rebuild the tree in order.
type StructurePartOut struct {
	Name  string `json:"name"`
	ID    int64  `json:"id"`
	Type  int    `json:"type,omitempty"`
	Depth int    `json:"depth"`
}

// FlattenStructure walks the nested structure tree into a depth-annotated
// flat list (schema-safe for structured outputs).
func FlattenStructure(parts []EstructuraPart) []StructurePartOut {
	var out []StructurePartOut
	var walk func(parts []EstructuraPart, depth int)
	walk = func(parts []EstructuraPart, depth int) {
		for _, p := range parts {
			out = append(out, StructurePartOut{Name: p.N, ID: p.I, Type: p.T, Depth: depth})
			walk(p.H, depth+1)
		}
	}
	walk(parts, 0)
	return out
}

// SectionContent returns the subtree of Html rooted at the block whose I
// equals sectionID (blocks nest via H). The bool reports whether the
// section exists in the norm.
func (n *NormaFull) SectionContent(sectionID int64) ([]HtmlBlock, bool) {
	var walk func(blocks []HtmlBlock) ([]HtmlBlock, bool)
	walk = func(blocks []HtmlBlock) ([]HtmlBlock, bool) {
		for i := range blocks {
			if blocks[i].I == sectionID {
				return []HtmlBlock{blocks[i]}, true
			}
			if subtree, ok := walk(blocks[i].H); ok {
				return subtree, true
			}
		}
		return nil, false
	}
	return walk(n.Html)
}

// CountArticles counts the artículo entries (T == 6) of the whole structure.
func (n *NormaFull) CountArticles() int {
	return n.countArticleParts(0)
}

// CountSectionArticles counts the artículo entries (T == 6) within the
// structure subtree rooted at partID (including the root). Zero when the
// part does not exist.
func (n *NormaFull) CountSectionArticles(partID int64) int {
	return n.countArticleParts(partID)
}

// countArticleParts walks the structure counting artículo entries (T == 6;
// the classification 1=título, 4=párrafo, 6=artículo is documented in
// law_client.go). With partID == 0 the whole structure counts.
func (n *NormaFull) countArticleParts(partID int64) int {
	count := 0
	var walk func(parts []EstructuraPart, inside bool)
	walk = func(parts []EstructuraPart, inside bool) {
		for _, p := range parts {
			in := inside || (partID != 0 && p.I == partID)
			if in && p.T == 6 {
				count++
			}
			walk(p.H, in)
		}
	}
	walk(n.Estructura, partID == 0)
	return count
}

// ContentCharCount returns the rune count of the Markdown rendering of the
// given block subtree. It MIRRORS renderBlocks in internal/tools (heading +
// leaf markdown, recursively) without building the string, so it stays
// cheap for structure_only requests. Keep the two in sync when either
// changes.
func ContentCharCount(blocks []HtmlBlock) int {
	total := 0
	var walk func(blocks []HtmlBlock, depth int)
	walk = func(blocks []HtmlBlock, depth int) {
		for _, block := range blocks {
			heading := block.SectionName
			if heading == "" {
				heading = fmt.Sprintf("Section %d", block.I)
			}
			prefix := "#"
			if depth < 3 {
				prefix = strings.Repeat("#", 3+depth)
			} else {
				prefix = "######"
			}
			total += len(prefix) + 1 + len([]rune(heading)) + 2 // "%s %s\n\n"
			if len(block.H) == 0 {
				total += len([]rune(block.Markdown)) + 2 // markdown + "\n\n"
			}
			walk(block.H, depth+1)
		}
	}
	walk(blocks, 0)
	return total
}

// ContentSizes maps every block id to the char count of its subtree —
// exactly what ContentCharCount returns for that subtree slice (block at
// depth 0, descendants at their relative depths). The fold renderer and
// the tests rely on this equality; keep in sync with ContentCharCount.
// The walk recomputes each subtree (integer adds only, lens are O(1)):
// measured corpus tops out at a few thousand blocks, so this stays in the
// microsecond range.
func ContentSizes(blocks []HtmlBlock) map[int64]int {
	sizes := make(map[int64]int)
	var walk func(blocks []HtmlBlock)
	walk = func(blocks []HtmlBlock) {
		for _, block := range blocks {
			sizes[block.I] = subtreeSize(block, 0)
			walk(block.H)
		}
	}
	walk(blocks)
	return sizes
}

// subtreeSize computes the ContentCharCount arithmetic of the subtree
// rooted at block at the given depth (heading prefix grows with depth,
// capped at ######; leaf blocks add their markdown).
func subtreeSize(block HtmlBlock, depth int) int {
	heading := block.SectionName
	if heading == "" {
		heading = fmt.Sprintf("Section %d", block.I)
	}
	prefix := 6
	if depth < 3 {
		prefix = 3 + depth
	}
	total := prefix + 1 + len([]rune(heading)) + 2 // "%s %s\n\n"
	if len(block.H) == 0 {
		total += len([]rune(block.Markdown)) + 2 // markdown + "\n\n"
	}
	for _, child := range block.H {
		total += subtreeSize(child, depth+1)
	}
	return total
}

// SectionStructure returns the structure subtree rooted at the part whose
// I equals sectionID (estructura nests via H). The bool reports whether
// the part exists.
func (n *NormaFull) SectionStructure(sectionID int64) ([]EstructuraPart, bool) {
	var walk func(parts []EstructuraPart) ([]EstructuraPart, bool)
	walk = func(parts []EstructuraPart) ([]EstructuraPart, bool) {
		for i := range parts {
			if parts[i].I == sectionID {
				return []EstructuraPart{parts[i]}, true
			}
			if subtree, ok := walk(parts[i].H); ok {
				return subtree, true
			}
		}
		return nil, false
	}
	return walk(n.Estructura)
}
