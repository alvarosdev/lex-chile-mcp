package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// GetCgrLegislacionArgs carries the arguments of the get_cgr_legislacion tool.
type GetCgrLegislacionArgs struct {
	LegislacionID string `json:"legislacion_id" jsonschema:"legislacion id e.g. RZA005691400 or FRA000012000"`
}

// GetCgrLegislacionOutput is the structured content of get_cgr_legislacion.
type GetCgrLegislacionOutput struct {
	LegislacionID     string `json:"legislacion_id"`
	Tipo              string `json:"tipo"`
	Numero            string `json:"numero"`
	Organismo         string `json:"organismo"`
	Materias          string `json:"materias"`
	Texto             string `json:"texto"`
	FechaDoc          string `json:"fecha_documento"`
	FechaPromulgacion string `json:"fecha_promulgacion"`
	FechaPublicacion  string `json:"fecha_publicacion"`
	DLAsociada        string `json:"dl_asociada"`
	Caracter          string `json:"caracter"`
	CharCount         int    `json:"char_count"`
}

// RegisterGetCgrLegislacion registers the get_cgr_legislacion tool.
func RegisterGetCgrLegislacion(srv *mcp.Server, client cgr.CgrClient) {
	registerTool(srv, &mcp.Tool{
		Name:        "get_cgr_legislacion",
		Description: "Get a Contraloría legislación entry by its legislacion_id (from search_cgr with source legislacion, e.g. RZA005691400 or FRA000012000). Returns tipo, número, organismo, materias, sanitized texto_, fecha_documento and caracter with char_count for citation.",
	}, makeGetCgrLegislacion(client))
}

func makeGetCgrLegislacion(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrLegislacionArgs, GetCgrLegislacionOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrLegislacionArgs) (*mcp.CallToolResult, GetCgrLegislacionOutput, error) {
		if strings.TrimSpace(args.LegislacionID) == "" {
			return errorResult("legislacion_id is required"), GetCgrLegislacionOutput{}, nil
		}
		full, err := client.GetLegislacion(ctx, args.LegislacionID)
		if err != nil {
			if errors.Is(err, cgr.ErrLegislacionNotFound) {
				return errorResult(fmt.Sprintf("legislacion not found: legislacion_id %q does not exist", args.LegislacionID)), GetCgrLegislacionOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr legislacion failed: %v", err)), GetCgrLegislacionOutput{}, nil
		}
		output := GetCgrLegislacionOutput{
			LegislacionID: full.DocID,
			Tipo:          full.Tipo,
			Numero:        full.Numero,
			Organismo:     full.Organismo,
			Materias:      full.Materias,
			Texto:         full.Texto,
			FechaDoc:      full.FechaDoc,
			Caracter:      full.Caracter,
			CharCount:     full.CharCount,
		}
		// FechaPromulgacion, FechaPublicacion, DLAsociada are not in current
		// wire model (cgrLegislacionSource only exposes fecha_documento, tipo,
		// numero, organismo, materias, texto_, caracter). Leave empty to avoid
		// leaking old_url or texto__raw; future wire expansion can populate them
		// without changing the tool schema.
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatLegislacion(full)},
			},
		}, output, nil
	}
}

func formatLegislacion(l cgr.LegislacionFull) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s %s", l.Tipo, l.Numero)
	if l.Organismo != "" {
		fmt.Fprintf(&b, " — %s", l.Organismo)
	}
	b.WriteString("\n\n")
	if l.Materias != "" {
		fmt.Fprintf(&b, "**Materias:** %s\n", l.Materias)
	}
	if l.FechaDoc != "" {
		fmt.Fprintf(&b, "**Fecha documento:** %s\n", l.FechaDoc)
	}
	if l.Caracter != "" {
		fmt.Fprintf(&b, "**Carácter:** %s\n", l.Caracter)
	}
	fmt.Fprintf(&b, "**Tamaño:** %s chars\n", humanCount(l.CharCount))
	if l.Texto == "" {
		b.WriteString("\n*Texto sin contenido*\n")
	} else {
		fmt.Fprintf(&b, "\n## Texto\n\n%s\n", l.Texto)
	}
	return b.String()
}
