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
type GetCgrDictamenArgs struct {
	DictamenID string `json:"dictamen_id" jsonschema:"the dictamen id (dictamen_id) from search_cgr_dictamenes results, e.g. E179593N25"`
}

// GetCgrDictamenOutput is the structured content of get_cgr_dictamen.
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
}

// RegisterGetCgrDictamen registers the get_cgr_dictamen tool.
func RegisterGetCgrDictamen(srv *mcp.Server, client cgr.CgrClient) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_cgr_dictamen",
		Description: "Get a Contraloría dictamen by its dictamen_id (from search_cgr_dictamenes). Returns metadata (materia, descriptores, criterio, origen, destinatarios, abogados, fuentes_legales, caracter) and the sanitized documento_completo with char_count and the HTML/PDF URLs for citation and PDF download.",
	}, makeGetCgrDictamen(client))
}

func makeGetCgrDictamen(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrDictamenArgs, GetCgrDictamenOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrDictamenArgs) (*mcp.CallToolResult, GetCgrDictamenOutput, error) {
		if args.DictamenID == "" {
			return errorResult("dictamen_id is required"), GetCgrDictamenOutput{}, nil
		}
		full, err := client.GetDictamen(ctx, args.DictamenID)
		if err != nil {
			if errors.Is(err, cgr.ErrDictamenNotFound) {
				return errorResult(fmt.Sprintf("dictamen not found: dictamen_id %q does not exist", args.DictamenID)), GetCgrDictamenOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr dictamen failed: %v", err)), GetCgrDictamenOutput{}, nil
		}
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
			Documento:      full.Documento,
			CharCount:      full.CharCount,
			URL:            full.URL,
			PDFURL:         full.PDFURL,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatDictamen(full)},
			},
		}, output, nil
	}
}

func formatDictamen(d cgr.DictamenFull) string {
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
	} else {
		fmt.Fprintf(&b, "\n## Documento Completo\n\n%s\n", d.Documento)
	}
	b.WriteString("\n---\n**Citación:**\n")
	fmt.Fprintf(&b, "- Visualización: %s\n", d.URL)
	fmt.Fprintf(&b, "- Descarga PDF: %s\n", d.PDFURL)
	return b.String()
}
