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

// GetCgrAuditoriaArgs carries the arguments of the get_cgr_auditoria tool.
type GetCgrAuditoriaArgs struct {
	AuditoriaID string `json:"auditoria_id" jsonschema:"auditoria id e.g. 371/2026 or 371N26"`
}

// GetCgrAuditoriaOutput is the structured content of get_cgr_auditoria.
type GetCgrAuditoriaOutput struct {
	AuditoriaID   string `json:"auditoria_id"`
	Numero        string `json:"numero"`
	Nombre        string `json:"nombre"`
	Tipo          string `json:"tipo"`
	Objetivo      string `json:"objetivo"`
	Conclusiones  string `json:"conclusiones"`
	Universo      string `json:"universo"`
	Muestra       string `json:"muestra"`
	Destinatarios string `json:"destinatarios"`
	Servicio      string `json:"servicio"`
	UnidadCgr     string `json:"unidad_cgr"`
	FechaDoc      string `json:"fecha_documento"`
	PDFURL        string `json:"pdf_url"`
	Contenido     string `json:"contenido"`
	CharCount     int    `json:"char_count"`
	Truncated     bool   `json:"truncated"`
}

var auditoriaToolIDRe = regexp.MustCompile(`^([0-9]{1,4}/[0-9]{4}|[A-Z]*[0-9]+N[0-9]{2}|[0-9]{1,6})$`)

// RegisterGetCgrAuditoria registers the get_cgr_auditoria tool.
func RegisterGetCgrAuditoria(srv *mcp.Server, client cgr.CgrClient) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_cgr_auditoria",
		Description: "Get a Contraloría auditoría informe by its auditoria_id (e.g. 371/2026 or 371N26). Returns metadata (número, nombre, tipo, objetivo, conclusiones, universo, muestra, destinatarios, servicio, unidad_cgr, fecha_documento) and sanitized contenido_pdf with char_count, truncated flag and PDF URL.",
	}, makeGetCgrAuditoria(client))
}

func makeGetCgrAuditoria(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrAuditoriaArgs, GetCgrAuditoriaOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrAuditoriaArgs) (*mcp.CallToolResult, GetCgrAuditoriaOutput, error) {
		trimmed := strings.TrimSpace(args.AuditoriaID)
		if trimmed == "" {
			return errorResult("auditoria_id is required"), GetCgrAuditoriaOutput{}, nil
		}
		if len(trimmed) > 40 {
			return errorResult(fmt.Sprintf("invalid auditoria_id %q: too long (max 40)", trimmed)), GetCgrAuditoriaOutput{}, nil
		}
		if strings.Contains(trimmed, "..") || strings.Contains(trimmed, "//") || strings.Contains(trimmed, "%") {
			return errorResult(fmt.Sprintf("invalid auditoria_id %q", trimmed)), GetCgrAuditoriaOutput{}, nil
		}
		if strings.ContainsAny(trimmed, ".?#\n\r") {
			return errorResult(fmt.Sprintf("invalid auditoria_id %q", trimmed)), GetCgrAuditoriaOutput{}, nil
		}
		if strings.Contains(trimmed, "\x00") {
			return errorResult(fmt.Sprintf("invalid auditoria_id %q", trimmed)), GetCgrAuditoriaOutput{}, nil
		}
		upper := strings.ToUpper(trimmed)
		if !auditoriaToolIDRe.MatchString(trimmed) && !auditoriaToolIDRe.MatchString(upper) {
			return errorResult(fmt.Sprintf("invalid auditoria_id %q", trimmed)), GetCgrAuditoriaOutput{}, nil
		}
		full, err := client.GetAuditoria(ctx, trimmed)
		if err != nil {
			if errors.Is(err, cgr.ErrAuditoriaNotFound) {
				return errorResult(fmt.Sprintf("auditoria not found: auditoria_id %q does not exist", trimmed)), GetCgrAuditoriaOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr auditoria failed: %v", err)), GetCgrAuditoriaOutput{}, nil
		}
		truncated := full.CharCount > 30000 || strings.Contains(full.ContenidoPDF, "[contenido truncado")
		auditoriaID := full.DocID
		if auditoriaID == "" {
			auditoriaID = full.Numero
		}
		if auditoriaID == "" {
			auditoriaID = trimmed
		}
		output := GetCgrAuditoriaOutput{
			AuditoriaID:   auditoriaID,
			Numero:        full.Numero,
			Nombre:        full.Nombre,
			Tipo:          full.Tipo,
			Objetivo:      full.Objetivo,
			Conclusiones:  full.Conclusiones,
			Universo:      "",
			Muestra:       "",
			Destinatarios: full.Destinatarios,
			Servicio:      "",
			UnidadCgr:     full.UnidadCgr,
			FechaDoc:      full.FechaDoc,
			PDFURL:        full.PDF,
			Contenido:     full.ContenidoPDF,
			CharCount:     full.CharCount,
			Truncated:     truncated,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatAuditoria(full, truncated)},
			},
		}, output, nil
	}
}

func formatAuditoria(a cgr.AuditoriaFull, truncated bool) string {
	var b strings.Builder
	id := a.DocID
	if id == "" {
		id = a.Numero
	}
	fmt.Fprintf(&b, "# Auditoría %s", id)
	if a.Nombre != "" {
		fmt.Fprintf(&b, " — %s", a.Nombre)
	}
	if a.FechaDoc != "" {
		fmt.Fprintf(&b, " (%s)", a.FechaDoc)
	}
	b.WriteString("\n\n")
	if a.Numero != "" {
		fmt.Fprintf(&b, "**Número:** %s\n", a.Numero)
	}
	if a.Tipo != "" {
		fmt.Fprintf(&b, "**Tipo:** %s\n", a.Tipo)
	}
	if a.UnidadCgr != "" {
		fmt.Fprintf(&b, "**Unidad CGR:** %s\n", a.UnidadCgr)
	}
	if a.FechaDoc != "" {
		fmt.Fprintf(&b, "**Fecha:** %s\n", a.FechaDoc)
	}
	fmt.Fprintf(&b, "**Tamaño:** %s chars", humanCount(a.CharCount))
	if truncated {
		b.WriteString(" (truncado)")
	}
	b.WriteString("\n")
	if a.Objetivo != "" {
		fmt.Fprintf(&b, "\n## Objetivo\n\n%s\n", a.Objetivo)
	}
	if a.Conclusiones != "" {
		fmt.Fprintf(&b, "\n## Conclusiones\n\n%s\n", a.Conclusiones)
	}
	if a.Destinatarios != "" {
		fmt.Fprintf(&b, "\n**Destinatarios:** %s\n", a.Destinatarios)
	}
	if a.ContenidoPDF == "" {
		b.WriteString("\n*Contenido sin texto extraído*\n")
	} else {
		fmt.Fprintf(&b, "\n## Contenido\n\n%s\n", a.ContenidoPDF)
	}
	if a.PDF != "" {
		b.WriteString("\n---\n**PDF:**\n")
		fmt.Fprintf(&b, "- Descarga PDF: %s\n", a.PDF)
	}
	return b.String()
}
