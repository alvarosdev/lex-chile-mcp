package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// GetLawArgs carries the arguments of the get_law tool. VersionDate,
// StructureOnly and SectionID are optional (omitempty keeps them out of
// the required schema).
type GetLawArgs struct {
	NormID        int64  `json:"norm_id" jsonschema:"the norm id (norm_id) from search_laws results"`
	VersionDate   string `json:"version_date,omitempty" jsonschema:"version in force at this date (YYYY-MM-DD, optional — defaults to the latest version)"`
	StructureOnly bool   `json:"structure_only,omitempty" jsonschema:"return metadata and table of contents only, without the full content (default false)"`
	SectionID     int64  `json:"section_id,omitempty" jsonschema:"structure id of the section to return (from get_law_summary or get_law structure_only); omit to return the whole norm"`
}

// GetLawOutput is the structured content of get_law. Estructura follows
// the request scope: the full flatten for structure_only, the section
// subtree when SectionID is set, empty in the degraded whole-norm map
// (the folded text TOC is the map; the complete structure lives in
// get_law_summary / structure_only). The rendered markdown travels in the
// text view only — never duplicated here. VersionDate and SectionID echo
// the requested scope; CharCount and ArticleCount always describe the
// REAL scope (whole norm or section), including when the content was
// withheld (Degraded) for exceeding the output budget.
type GetLawOutput struct {
	Metadatos    bcn.Metadatos          `json:"metadatos"`
	Estructura   []bcn.StructurePartOut `json:"estructura,omitempty"`
	Proyectos    []bcn.Proyecto         `json:"proyectos"`
	VersionDate  string                 `json:"version_date,omitempty"`
	SectionID    int64                  `json:"section_id,omitempty"`
	CharCount    int                    `json:"char_count"`
	ArticleCount int                    `json:"article_count"`
	Degraded     bool                   `json:"degraded,omitempty"`
}

// RegisterGetLaw registers the get_law tool on the MCP server.
func RegisterGetLaw(srv *mcp.Server, client bcn.LawClient) {
	registerTool(srv, &mcp.Tool{
		Name: "get_law",
		Description: "Get the content of a Chilean law, decree or resolution by its norm_id " +
			"(from search_laws). Returns metadata, the table of contents and the text in " +
			"Markdown. Warning: long norms can exceed hundreds of thousands of characters — " +
			"prefer get_law_summary first, then structure_only=true to explore, then " +
			"section_id to read only the parts you need.",
	}, makeGetLaw(client))
}

func makeGetLaw(client bcn.LawClient) mcp.ToolHandlerFor[GetLawArgs, GetLawOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetLawArgs) (*mcp.CallToolResult, GetLawOutput, error) {
		if args.NormID <= 0 {
			return errorResult("norm_id must be a positive number"), GetLawOutput{}, nil
		}
		if err := validateVersionDate(args.VersionDate); err != nil {
			return errorResult(err.Error()), GetLawOutput{}, nil
		}
		if args.SectionID < 0 {
			return errorResult("section_id must be a positive number"), GetLawOutput{}, nil
		}

		query := bcn.NormaQuery{NormID: args.NormID, VersionDate: args.VersionDate}
		norma, err := client.GetNorma(ctx, query)
		if err != nil {
			if errors.Is(err, bcn.ErrNormaNotFound) {
				return errorResult(fmt.Sprintf("norma not found: norm_id %d does not exist in LeyChile", args.NormID)), GetLawOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get law failed: %v", err)), GetLawOutput{}, nil
		}

		// Semantic validation needs the norm's structure, so it runs after
		// the fetch (ETag-cached anyway). The error teaches the recovery
		// path instead of leaving the agent guessing.
		if args.SectionID > 0 {
			if _, ok := norma.SectionContent(args.SectionID); !ok {
				return errorResult(fmt.Sprintf("section not found: section_id %d does not exist in this norm — call get_law with structure_only=true to list valid section ids", args.SectionID)), GetLawOutput{}, nil
			}
		}

		scope := resolveScope(norma, args)
		output := buildGetLawOutput(norma, args, scope)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatNorma(norma, args, scope)},
			},
		}, output, nil
	}
}

// contentBlocks returns the block subtree this response renders: the whole
// norm, or the section subtree when SectionID is set (falling back to the
// whole norm when the section does not exist — the handler validates first).
func contentBlocks(norma bcn.NormaFull, args GetLawArgs) []bcn.HtmlBlock {
	if args.SectionID > 0 {
		if subtree, ok := norma.SectionContent(args.SectionID); ok {
			return subtree
		}
	}
	return norma.Html
}

// normaScope is the resolved delivery scope of a get_law request: the
// blocks to render, per-section sizes for the folded TOC, the REAL counts
// of the requested scope, and whether the content exceeds the output
// budget (degraded — delivered as a navigable map instead of a body).
type normaScope struct {
	blocks       []bcn.HtmlBlock
	sizes        map[int64]int
	charCount    int
	articleCount int
	degraded     bool
}

// resolveScope computes the delivery scope once for both views (text and
// structured) so the budget check and the counts cannot drift apart.
func resolveScope(norma bcn.NormaFull, args GetLawArgs) normaScope {
	blocks := contentBlocks(norma, args)
	articleCount := norma.CountArticles()
	if args.SectionID > 0 {
		articleCount = norma.CountSectionArticles(args.SectionID)
	}
	charCount := bcn.ContentCharCount(blocks)
	return normaScope{
		blocks:       blocks,
		sizes:        bcn.ContentSizes(norma.Html),
		charCount:    charCount,
		articleCount: articleCount,
		degraded:     !args.StructureOnly && charCount > maxContentChars,
	}
}

// buildGetLawOutput projects the norm into the structured output. The
// markdown body is never duplicated here (the text view renders it);
// Estructura follows the request scope (see GetLawOutput). The counts
// always describe the REAL requested scope — whole norm or section —
// including in degraded responses, where they state what was withheld.
func buildGetLawOutput(norma bcn.NormaFull, args GetLawArgs, scope normaScope) GetLawOutput {
	out := GetLawOutput{
		Metadatos:    norma.Metadatos,
		Proyectos:    norma.Proyectos,
		VersionDate:  args.VersionDate,
		SectionID:    args.SectionID,
		CharCount:    scope.charCount,
		ArticleCount: scope.articleCount,
		Degraded:     scope.degraded,
	}
	switch {
	case args.StructureOnly:
		out.Estructura = bcn.FlattenStructure(norma.Estructura)
	case args.SectionID > 0:
		if subtree, ok := norma.SectionStructure(args.SectionID); ok {
			out.Estructura = bcn.FlattenStructure(subtree)
		}
	case scope.degraded:
		// Degraded whole-norm map: the folded text TOC is the map. The
		// complete flatten stays in get_law_summary / structure_only.
	default:
		out.Estructura = bcn.FlattenStructure(norma.Estructura)
	}
	return out
}

// normaContentMarkdown assembles the content section of a norm. Blocks nest
// (titles contain articles): each block gets a heading at its depth and its
// own text only when it has no children — a title block's body repeats the
// title, so it is skipped in favor of its children.
func normaContentMarkdown(blocks []bcn.HtmlBlock) string {
	var b strings.Builder
	renderBlocks(&b, blocks, 0)
	return b.String()
}

// renderBlocks walks the block tree, heading depth grows with nesting
// (max ######). bcn.ContentCharCount mirrors this rendering — keep the
// two in sync when either changes.
func renderBlocks(b *strings.Builder, blocks []bcn.HtmlBlock, depth int) {
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
		fmt.Fprintf(b, "%s %s\n\n", prefix, heading)
		if len(block.H) == 0 {
			b.WriteString(block.Markdown)
			b.WriteString("\n\n")
		}
		renderBlocks(b, block.H, depth+1)
	}
}

// formatNorma renders a norm for the LLM: metadata header with the REAL
// scope size and (when sectioned) the section name, related bills, the
// folded table of contents — the whole norm's map, or the local sub-TOC of
// the section's children — and the Markdown content. Content that exceeds
// the output budget is NEVER delivered truncated: the response degrades to
// the folded map plus an explicit drill signal. When a historical version
// was requested, the header states it.
func formatNorma(norma bcn.NormaFull, args GetLawArgs, scope normaScope) string {
	var b strings.Builder
	m := norma.Metadatos

	fmt.Fprintf(&b, "# %s\n\n", m.TituloNorma)
	if args.VersionDate != "" {
		fmt.Fprintf(&b, "Version: as of %s\n", args.VersionDate)
	}
	fmt.Fprintf(&b, "Type: %s · Source: %s N° %s\n",
		formatTipos(m.TiposNumeros), m.Fuente, m.NumeroFuente)
	fmt.Fprintf(&b, "Organism: %s\n", strings.Join(m.Organismos, "; "))
	fmt.Fprintf(&b, "Published: %s · In force from: %s",
		m.FechaPublicacion, m.Vigencia.InicioVigencia)
	if m.Vigencia.FinVigencia != "" {
		fmt.Fprintf(&b, " to %s", m.Vigencia.FinVigencia)
	}
	fmt.Fprintf(&b, "\nDerogated: %t\n", m.Derogado)
	fmt.Fprintf(&b, "Size: %s chars · %s\n", humanCount(scope.charCount), formatArticles(scope.articleCount))
	if args.SectionID > 0 {
		fmt.Fprintf(&b, "Section: %s\n", sectionHeading(scope.blocks, args.SectionID))
	}
	if len(m.Materias) > 0 {
		fmt.Fprintf(&b, "Subjects: %s\n", strings.Join(m.Materias, ", "))
	}
	if len(m.CategoriasNorma) > 0 {
		fmt.Fprintf(&b, "Categories: %s\n", strings.Join(m.CategoriasNorma, ", "))
	}
	if len(m.Vinculaciones) > 0 {
		links := make([]string, 0, len(m.Vinculaciones))
		for _, v := range m.Vinculaciones {
			links = append(links, v.Text)
		}
		// Header hygiene: cap the visible list; the structured output
		// keeps every vinculacion.
		if len(links) > maxRelatedNorms {
			fmt.Fprintf(&b, "Related norms: %d total — showing first %d: %s\n",
				len(links), maxRelatedNorms, strings.Join(links[:maxRelatedNorms], "; "))
		} else {
			fmt.Fprintf(&b, "Related norms: %s\n", strings.Join(links, "; "))
		}
	}

	// The section view stays lightweight: the summary and related bills are
	// skipped in the TEXT when drilling into a section (they repeat what the
	// summary call already showed and they ride along complete in
	// structuredContent).
	if args.SectionID == 0 {
		if len(m.Resumenes) > 0 {
			fmt.Fprintf(&b, "\nSummary: %s\n", truncate(m.Resumenes[0], 1200))
		}
		if len(norma.Proyectos) > 0 {
			type billLine struct{ boletin, info, enlace string }
			var bills []billLine
			for _, p := range norma.Proyectos {
				for _, pl := range p.Pls {
					bills = append(bills, billLine{pl.NroBoletin, pl.Informacion, pl.Enlace})
				}
			}
			// Header hygiene, same rule as related norms: meganorms in
			// permanent amendment carry hundreds of bills (the Código
			// Civil lists ~330). The structured output keeps them all.
			b.WriteString("\n## Related bills\n")
			visible := bills
			if len(visible) > maxRelatedNorms {
				visible = visible[:maxRelatedNorms]
				fmt.Fprintf(&b, "%d total — showing first %d\n\n", len(bills), maxRelatedNorms)
			}
			for _, bl := range visible {
				fmt.Fprintf(&b, "- %s — %s\n", bl.boletin, bl.info)
				if bl.enlace != "" {
					fmt.Fprintf(&b, "  %s\n", bl.enlace)
				}
			}
		}
	}

	// Structure: the global folded map, or the LOCAL sub-TOC of the
	// requested section's children — the next drill-down level travels
	// with every section response, the global index does not.
	b.WriteString("\n## Structure\n")
	flat := bcn.FlattenStructure(norma.Estructura)
	if args.SectionID > 0 {
		renderFoldedSubTOC(&b, flat, scope.sizes, args.SectionID)
	} else {
		renderFoldedTOC(&b, flat, scope.sizes)
	}

	switch {
	case scope.degraded:
		// Completeness invariant: declare the omission, the real total and
		// the recovery path — never a silent cut.
		fmt.Fprintf(&b, "\n## Content\n\nContent withheld: %s chars exceeds the %s-char response budget. "+
			"Drill with section_id (ids and sizes listed above).\n",
			humanCount(scope.charCount), humanCount(maxContentChars))
	case !args.StructureOnly:
		b.WriteString("\n## Content\n\n")
		b.WriteString(normaContentMarkdown(scope.blocks))
	}

	return b.String()
}

// sectionHeading names the requested section for the header line.
func sectionHeading(blocks []bcn.HtmlBlock, sectionID int64) string {
	if len(blocks) > 0 && blocks[0].SectionName != "" {
		return blocks[0].SectionName
	}
	return fmt.Sprintf("Section %d", sectionID)
}

// humanCount formats a count for the LLM: the number itself under 1000,
// one decimal up to 10K ("3.2K"), a compact "426K" above.
func humanCount(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.0fK", float64(n)/1000)
}

// formatArticles renders the article count with singular/plural.
func formatArticles(n int) string {
	if n == 1 {
		return "1 article"
	}
	return fmt.Sprintf("%d articles", n)
}

// validateVersionDate enforces the strict YYYY-MM-DD format. The API
// silently ignores malformed dates (returns the latest version), so the
// tool fails fast instead of letting the model believe it read the
// requested version.
func validateVersionDate(versionDate string) error {
	if versionDate == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", versionDate); err != nil {
		return fmt.Errorf("version_date must be a valid date in YYYY-MM-DD format, got %q", versionDate)
	}
	return nil
}

// formatTipos renders the norm type list as "Ley 21600; Decreto 1".
func formatTipos(tipos []bcn.TipoNumero) string {
	parts := make([]string, 0, len(tipos))
	for _, t := range tipos {
		parts = append(parts, fmt.Sprintf("%s %s", t.Descripcion, t.Numero))
	}
	return strings.Join(parts, "; ")
}
