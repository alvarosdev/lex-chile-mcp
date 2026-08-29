package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// GetCgrDictamenArgs carries the arguments of the get_cgr_dictamen tool.
// Part selects the document page (1-indexed, default 1) — see the
// output-budget linear pagination: parts of at most the server budget,
// cut at paragraph breaks, every part carrying its range signal.
type GetCgrDictamenArgs struct {
	DictamenID string `json:"dictamen_id" jsonschema:"the dictamen id (dictamen_id) from search_cgr_dictamenes results, e.g. E179593N25"`
	Part       int    `json:"part,omitempty" jsonschema:"document part to read, starting at 1 (default 1); continue with part=2 when part 1 signals more parts"`
}

// GetCgrDictamenOutput is the structured content of get_cgr_dictamen.
// CharCount always describes the COMPLETE document; Documento carries
// only the requested part's slice, scoped by Part/TotalParts and the
// CharStart/CharEnd rune offsets.
type GetCgrDictamenOutput struct {
	DictamenID     string `json:"dictamen_id"`
	NDictamen      string `json:"n_dictamen"`
	NumericID      string `json:"numeric_doc_id"`
	FechaDoc       string `json:"fecha_documento"`
	Materia        string `json:"materia"`
	Descriptores   string `json:"descriptores"`
	Criterio       string `json:"criterio"`
	Origen         string `json:"origen"`
	Caracter       string `json:"caracter"`
	Destinatarios  string `json:"destinatarios"`
	Abogados       string `json:"abogados"`
	FuentesLegales string `json:"fuentes_legales"`
	Documento      string `json:"documento_completo"`
	CharCount      int    `json:"char_count"`
	URL            string `json:"url"`
	PDFURL         string `json:"pdf_url"`
	Part           int    `json:"part"`
	TotalParts     int    `json:"total_parts"`
	CharStart      int    `json:"char_start"`
	CharEnd        int    `json:"char_end"`
}

// RegisterGetCgrDictamen registers the get_cgr_dictamen tool.
func RegisterGetCgrDictamen(srv *mcp.Server, client cgr.CgrClient) {
	registerTool(srv, &mcp.Tool{
		Name: "get_cgr_dictamen",
		Description: "Get a Contraloría dictamen by its dictamen_id (from search_cgr_dictamenes). Returns " +
			"metadata (materia, descriptores, criterio, origen, destinatarios, abogados, fuentes_legales, caracter) " +
			"and the full document text. Long documents arrive paginated: part 1 carries the metadata and the " +
			"first ~100K chars; continue with part=2, part=3… following the range signal " +
			"(`part N of M · chars X–Y of Z`).",
	}, makeGetCgrDictamen(client))
}

const defaultPart = 1

func makeGetCgrDictamen(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrDictamenArgs, GetCgrDictamenOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrDictamenArgs) (*mcp.CallToolResult, GetCgrDictamenOutput, error) {
		if args.DictamenID == "" {
			return errorResult("dictamen_id is required"), GetCgrDictamenOutput{}, nil
		}
		// part 0 = default; a negative part fails before any fetch. The
		// upper bound is only known after the document is retrieved (it
		// fixes the part count).
		part := args.Part
		if part == 0 {
			part = defaultPart
		}
		if part < 1 {
			return errorResult(fmt.Sprintf("part must be 1 or greater, got %d", args.Part)), GetCgrDictamenOutput{}, nil
		}

		full, err := client.GetDictamen(ctx, args.DictamenID)
		if err != nil {
			if errors.Is(err, cgr.ErrDictamenNotFound) {
				return errorResult(fmt.Sprintf("dictamen not found: dictamen_id %q does not exist", args.DictamenID)), GetCgrDictamenOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr dictamen failed: %v", err)), GetCgrDictamenOutput{}, nil
		}

		pages := cgr.PaginateStandard(full.Documento)
		if part > pages.Count() {
			return errorResult(fmt.Sprintf("part must be between 1 and %d for dictamen %q (%s chars), got %d",
				pages.Count(), args.DictamenID, humanCount(full.CharCount), part)), GetCgrDictamenOutput{}, nil
		}

		page := pages.Range(part - 1)
		output := GetCgrDictamenOutput{
			DictamenID:     full.DictamenID,
			NDictamen:      full.NDictamen,
			NumericID:      full.NumericID,
			FechaDoc:       full.FechaDoc,
			Materia:        full.Materia,
			Descriptores:   full.Descriptores,
			Criterio:       full.Criterio,
			Origen:         full.Origen,
			Caracter:       full.Caracter,
			Destinatarios:  full.Destinatarios,
			Abogados:       full.Abogados,
			FuentesLegales: full.FuentesLegales,
			Documento:      pages.Slice(full.Documento, part-1),
			CharCount:      full.CharCount,
			URL:            full.URL,
			PDFURL:         full.PDFURL,
			Part:           part,
			TotalParts:     pages.Count(),
			CharStart:      page.Start,
			CharEnd:        page.End,
		}

		// Part 1 carries the full metadata header; continuation parts
		// repeat only the range header (metadata is ~1K tokens of pure
		// repetition across parts).
		var text string
		if part == 1 {
			text = formatDictamen(full, pages, part)
		} else {
			text = formatDictamenPart(full, pages, part)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: text},
			},
		}, output, nil
	}
}

// rangeSignal renders the completeness signal every part carries: part
// position, char range and document total, plus the continuation path.
// HTTP 206-style honesty: a part never poses as the whole document.
func rangeSignal(pages *cgr.Pages, doc string, part int) string {
	page := pages.Range(part - 1)
	total := len([]rune(doc))
	signal := fmt.Sprintf("**Documento:** part %d of %d · chars %d–%d of %d",
		part, pages.Count(), page.Start+1, page.End, total)
	if part < pages.Count() {
		signal += fmt.Sprintf(" · continue with part=%d", part+1)
	}
	return signal
}

// formatDictamen renders part 1: full metadata header, the document slice
// and the range signal with citación.
func formatDictamen(d cgr.DictamenFull, pages *cgr.Pages, part int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Dictamen %s — %s (%s)\n\n", d.DictamenID, d.NDictamen, d.FechaDoc)
	fmt.Fprintf(&b, "**Materia:** %s\n", d.Materia)
	if d.Descriptores != "" {
		fmt.Fprintf(&b, "**Descriptores:** %s\n", d.Descriptores)
	}
	if d.Origen != "" {
		fmt.Fprintf(&b, "**Origen:** %s\n", d.Origen)
	}
	if d.Criterio != "" {
		fmt.Fprintf(&b, "**Criterio:** %s\n", d.Criterio)
	}
	fmt.Fprintf(&b, "**Carácter:** %s", d.Caracter)
	if d.Destinatarios != "" {
		fmt.Fprintf(&b, " | **Destinatarios:** %s", d.Destinatarios)
	}
	if d.Abogados != "" {
		fmt.Fprintf(&b, " | **Abogados:** %s", d.Abogados)
	}
	b.WriteString("\n")
	if d.FuentesLegales != "" {
		fmt.Fprintf(&b, "**Fuentes legales:** %s\n", d.FuentesLegales)
	}
	fmt.Fprintf(&b, "**Tamaño:** %s chars\n", humanCount(d.CharCount))
	if d.Documento == "" {
		b.WriteString("\n*Documento sin contenido*\n")
		fmt.Fprintf(&b, "\n%s\n", rangeSignal(pages, d.Documento, part))
	} else {
		fmt.Fprintf(&b, "\n## Documento Completo\n\n%s\n", pages.Slice(d.Documento, part-1))
		fmt.Fprintf(&b, "\n%s\n", rangeSignal(pages, d.Documento, part))
	}
	b.WriteString("\n---\n**Citación:**\n")
	fmt.Fprintf(&b, "- Visualización: %s\n", d.URL)
	fmt.Fprintf(&b, "- Descarga PDF: %s\n", d.PDFURL)
	return b.String()
}

// formatDictamenPart renders a continuation part: range header + document
// slice + range signal. No metadata, no citación — part 1 carries them.
func formatDictamenPart(d cgr.DictamenFull, pages *cgr.Pages, part int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Dictamen %s — part %d of %d\n\n", d.DictamenID, part, pages.Count())
	fmt.Fprintf(&b, "## Documento Completo (continuación)\n\n%s\n", pages.Slice(d.Documento, part-1))
	b.WriteString(rangeSignal(pages, d.Documento, part))
	return b.String()
}
