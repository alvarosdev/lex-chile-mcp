package tools

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// GetCgrConsolidadoArgs carries the arguments of the get_cgr_consolidado tool.
type GetCgrConsolidadoArgs struct {
	ConsolidadoID string `json:"consolidado_id" jsonschema:"consolidado id e.g. CIC21/2026"`
}

// GetCgrConsolidadoOutput is the structured content of get_cgr_consolidado.
type GetCgrConsolidadoOutput struct {
	ConsolidadoID     string `json:"consolidado_id"`
	Numero            string `json:"numero"`
	Nombre            string `json:"nombre"`
	Tipo              string `json:"tipo"`
	Resena            string `json:"resena"`
	ContenidoExtraido string `json:"contenido_extraido"`
	FechaDoc          string `json:"fecha_documento"`
	UnidadCgr         string `json:"unidad_cgr"`
	Sector            string `json:"sector"`
	PDFURL            string `json:"pdf_url"`
	CharCount         int    `json:"char_count"`
}

var consolidadoToolIDRe = regexp.MustCompile(`^CIC[0-9]{1,4}/[0-9]{4}$`)

// RegisterGetCgrConsolidado registers the get_cgr_consolidado tool.
func RegisterGetCgrConsolidado(srv *mcp.Server, client cgr.CgrClient) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_cgr_consolidado",
		Description: "Get a Contraloría consolidado CIC by its consolidado_id (e.g. CIC21/2026). Returns metadata (número, nombre, tipo, reseña, fecha_documento, unidad_cgr, sector) and sanitized contenido_extraido with char_count and PDF URL.",
	}, makeGetCgrConsolidado(client))
}

func makeGetCgrConsolidado(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrConsolidadoArgs, GetCgrConsolidadoOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrConsolidadoArgs) (*mcp.CallToolResult, GetCgrConsolidadoOutput, error) {
		trimmed := strings.TrimSpace(args.ConsolidadoID)
		if trimmed == "" {
			return errorResult("consolidado_id is required"), GetCgrConsolidadoOutput{}, nil
		}
		trimmed = strings.ToUpper(trimmed)
		if len(trimmed) > 20 {
			return errorResult(fmt.Sprintf("invalid consolidado_id %q: too long (max 20)", trimmed)), GetCgrConsolidadoOutput{}, nil
		}
		if strings.ContainsAny(trimmed, ".%?#\n\r\x00") {
			return errorResult(fmt.Sprintf("invalid consolidado_id %q", trimmed)), GetCgrConsolidadoOutput{}, nil
		}
		if strings.Contains(trimmed, "..") || strings.Contains(trimmed, "//") || strings.Contains(trimmed, "%") {
			return errorResult(fmt.Sprintf("invalid consolidado_id %q", trimmed)), GetCgrConsolidadoOutput{}, nil
		}
		// Reject control chars
		for _, r := range trimmed {
			if r < 0x20 && r != '\t' && r != '\n' {
				return errorResult(fmt.Sprintf("invalid consolidado_id %q", trimmed)), GetCgrConsolidadoOutput{}, nil
			}
		}
		if !consolidadoToolIDRe.MatchString(trimmed) {
			return errorResult(fmt.Sprintf("invalid consolidado_id %q", trimmed)), GetCgrConsolidadoOutput{}, nil
		}
		full, err := client.GetConsolidado(ctx, trimmed)
		if err != nil {
			if errors.Is(err, cgr.ErrConsolidadoNotFound) {
				return errorResult(fmt.Sprintf("consolidado not found: consolidado_id %q does not exist", trimmed)), GetCgrConsolidadoOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr consolidado failed: %v", err)), GetCgrConsolidadoOutput{}, nil
		}
		consolidadoID := full.Numero
		if consolidadoID == "" {
			consolidadoID = trimmed
		}
		output := GetCgrConsolidadoOutput{
			ConsolidadoID:     consolidadoID,
			Numero:            full.Numero,
			Nombre:            full.Nombre,
			Tipo:              full.Tipo,
			Resena:            full.Resena,
			ContenidoExtraido: full.ContenidoExtraido,
			FechaDoc:          full.FechaDoc,
			UnidadCgr:         full.UnidadCgr,
			Sector:            "",
			PDFURL:            full.PDFWeb,
			CharCount:         full.CharCount,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatConsolidado(full)},
			},
		}, output, nil
	}
}

func formatConsolidado(c cgr.ConsolidadoFull) string {
	var b strings.Builder
	id := c.Numero
	fmt.Fprintf(&b, "# Consolidado %s", id)
	if c.Nombre != "" {
		fmt.Fprintf(&b, " — %s", c.Nombre)
	}
	if c.FechaDoc != "" {
		fmt.Fprintf(&b, " (%s)", c.FechaDoc)
	}
	b.WriteString("\n\n")
	if c.Numero != "" {
		fmt.Fprintf(&b, "**Número:** %s\n", c.Numero)
	}
	if c.Tipo != "" {
		fmt.Fprintf(&b, "**Tipo:** %s\n", c.Tipo)
	}
	if c.UnidadCgr != "" {
		fmt.Fprintf(&b, "**Unidad CGR:** %s\n", c.UnidadCgr)
	}
	if c.FechaDoc != "" {
		fmt.Fprintf(&b, "**Fecha:** %s\n", c.FechaDoc)
	}
	fmt.Fprintf(&b, "**Tamaño:** %s chars\n", humanCount(c.CharCount))
	if c.Resena != "" {
		fmt.Fprintf(&b, "\n## Reseña\n\n%s\n", c.Resena)
	}
	if c.ContenidoExtraido == "" {
		b.WriteString("\n*Contenido sin texto extraído*\n")
	} else {
		fmt.Fprintf(&b, "\n## Contenido Extraído\n\n%s\n", c.ContenidoExtraido)
	}
	if c.PDFWeb != "" {
		b.WriteString("\n---\n**PDF:**\n")
		fmt.Fprintf(&b, "- Descarga PDF: %s\n", c.PDFWeb)
	}
	return b.String()
}
