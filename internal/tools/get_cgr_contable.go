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

// GetCgrContableArgs carries the arguments of the get_cgr_contable tool.
type GetCgrContableArgs struct {
	ContableID string `json:"contable_id" jsonschema:"contable id e.g. E080961 or OFE0809612600"`
}

// GetCgrContableOutput is the structured content of get_cgr_contable.
type GetCgrContableOutput struct {
	ContableID        string `json:"contable_id"`
	Numero            string `json:"numero"`
	NormativaContable string `json:"normativa_contable"`
	Tipo              string `json:"tipo"`
	Parte             string `json:"parte"`
	Destinatarios     string `json:"destinatarios"`
	Origen            string `json:"origen"`
	FechaDoc          string `json:"fecha_documento"`
	Texto             string `json:"texto"`
	CharCount         int    `json:"char_count"`
	URL               string `json:"url"`
	PDFURL            string `json:"pdf_url"`
}

var contableIDToolRe = regexp.MustCompile(`^(E[0-9]{1,6}|OFE[0-9]{10,13}|E[0-9]+N[0-9]{2})$`)

// RegisterGetCgrContable registers the get_cgr_contable tool.
func RegisterGetCgrContable(srv *mcp.Server, client cgr.CgrClient) {
	registerTool(srv, &mcp.Tool{
		Name:        "get_cgr_contable",
		Description: "Get a contable oficio by its contable_id (from search results, e.g. E080961 or OFE0809612600). Returns metadata (numero, normativa_contable, tipo, parte, destinatarios, origen, fecha_documento) and the sanitized texto with char_count.",
	}, makeGetCgrContable(client))
}

func makeGetCgrContable(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrContableArgs, GetCgrContableOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrContableArgs) (*mcp.CallToolResult, GetCgrContableOutput, error) {
		if args.ContableID == "" {
			return errorResult("contable_id is required"), GetCgrContableOutput{}, nil
		}
		id := strings.TrimSpace(strings.ToUpper(args.ContableID))
		if len(id) > 40 {
			return errorResult(fmt.Sprintf("invalid contable_id %q: too long", id)), GetCgrContableOutput{}, nil
		}
		if strings.ContainsAny(id, "/.%?#\n\r") {
			return errorResult(fmt.Sprintf("invalid contable_id %q", id)), GetCgrContableOutput{}, nil
		}
		if !contableIDToolRe.MatchString(id) {
			return errorResult(fmt.Sprintf("invalid contable_id %q", id)), GetCgrContableOutput{}, nil
		}
		full, err := client.GetContable(ctx, id)
		if err != nil {
			if errors.Is(err, cgr.ErrContableNotFound) || errors.Is(err, cgr.ErrDictamenNotFound) {
				return errorResult(fmt.Sprintf("contable not found: contable_id %q does not exist", id)), GetCgrContableOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr contable failed: %v", err)), GetCgrContableOutput{}, nil
		}
		output := GetCgrContableOutput{
			ContableID:        full.DocID,
			Numero:            full.Numero,
			NormativaContable: full.NormativaContable,
			Tipo:              full.Tipo,
			Parte:             full.Parte,
			Destinatarios:     full.Destinatarios,
			Origen:            full.Origen,
			FechaDoc:          full.FechaDoc,
			Texto:             full.Texto,
			CharCount:         full.CharCount,
			URL:               "",
			PDFURL:            "",
		}
		if output.ContableID == "" {
			output.ContableID = id
		}
		if output.Numero == "" {
			output.Numero = full.Numero
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatContable(full)},
			},
		}, output, nil
	}
}

func formatContable(c cgr.ContableFull) string {
	var b strings.Builder
	headerID := c.DocID
	if headerID == "" {
		headerID = c.Numero
	}
	if headerID == "" {
		headerID = c.NormativaContable
	}
	fmt.Fprintf(&b, "# Contable %s — %s (%s)\n\n", headerID, c.Numero, c.FechaDoc)
	if c.NormativaContable != "" {
		fmt.Fprintf(&b, "**Normativa contable:** %s\n", c.NormativaContable)
	}
	if c.Tipo != "" {
		fmt.Fprintf(&b, "**Tipo:** %s\n", c.Tipo)
	}
	if c.Origen != "" {
		fmt.Fprintf(&b, "**Origen:** %s\n", c.Origen)
	}
	if c.Destinatarios != "" {
		fmt.Fprintf(&b, "**Destinatarios:** %s\n", c.Destinatarios)
	}
	fmt.Fprintf(&b, "**Tamaño:** %s chars\n", humanCount(c.CharCount))
	if c.Parte != "" {
		fmt.Fprintf(&b, "\n## Parte\n\n%s\n", c.Parte)
	}
	if c.Texto == "" {
		if c.Parte == "" {
			b.WriteString("\n*Documento sin contenido*\n")
		}
	} else {
		fmt.Fprintf(&b, "\n## Texto\n\n%s\n", c.Texto)
	}
	return b.String()
}
