package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// GetCgrCuentaArgs carries the arguments of the get_cgr_cuenta tool.
type GetCgrCuentaArgs struct {
	CuentaID string `json:"cuenta_id" jsonschema:"cuenta id e.g. 2982331"`
}

// GetCgrCuentaOutput is the structured content of get_cgr_cuenta.
type GetCgrCuentaOutput struct {
	CuentaID         string `json:"cuenta_id"`
	NumeroSentencia  string `json:"numero_sentencia"`
	NumeroExpediente string `json:"numero_expediente"`
	Texto            string `json:"texto"`
	FechaSentencia   string `json:"fecha_sentencia"`
	FechaExpediente  string `json:"fecha_expediente"`
	PDFURL           string `json:"pdf_url"`
	PDFURL2          string `json:"pdf_url2"`
	CharCount        int    `json:"char_count"`
}

// RegisterGetCgrCuenta registers the get_cgr_cuenta tool.
func RegisterGetCgrCuenta(srv *mcp.Server, client cgr.CgrClient) {
	registerTool(srv, &mcp.Tool{
		Name:        "get_cgr_cuenta",
		Description: "Get a Contraloría cuenta sentencia by its cuenta_id (from search_cgr with source cuentas). Returns numero_sentencia, numero_expediente, sanitized texto, fecha_sentencia, fecha_expediente and PDF URLs with char_count for citation.",
	}, makeGetCgrCuenta(client))
}

func makeGetCgrCuenta(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrCuentaArgs, GetCgrCuentaOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrCuentaArgs) (*mcp.CallToolResult, GetCgrCuentaOutput, error) {
		if strings.TrimSpace(args.CuentaID) == "" {
			return errorResult("cuenta_id is required"), GetCgrCuentaOutput{}, nil
		}
		full, err := client.GetCuenta(ctx, args.CuentaID)
		if err != nil {
			if errors.Is(err, cgr.ErrCuentaNotFound) {
				return errorResult(fmt.Sprintf("cuenta not found: cuenta_id %q does not exist", args.CuentaID)), GetCgrCuentaOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr cuenta failed: %v", err)), GetCgrCuentaOutput{}, nil
		}
		output := GetCgrCuentaOutput{
			CuentaID:         full.DocID,
			NumeroSentencia:  full.NumeroSentencia,
			NumeroExpediente: full.NumeroExpediente,
			Texto:            full.Texto,
			FechaSentencia:   full.FechaSentencia,
			FechaExpediente:  full.FechaDoc,
			PDFURL:           full.PDF,
			PDFURL2:          full.PDF2,
			CharCount:        full.CharCount,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatCuenta(full)},
			},
		}, output, nil
	}
}

func formatCuenta(c cgr.CuentaFull) string {
	var b strings.Builder
	// Header with identificadores
	fmt.Fprintf(&b, "# Cuenta %s", c.DocID)
	if c.NumeroSentencia != "" {
		fmt.Fprintf(&b, " — Sentencia %s", c.NumeroSentencia)
	}
	if c.NumeroExpediente != "" {
		fmt.Fprintf(&b, " (Exp. %s)", c.NumeroExpediente)
	}
	b.WriteString("\n\n")
	if c.FechaSentencia != "" {
		fmt.Fprintf(&b, "**Fecha sentencia:** %s\n", c.FechaSentencia)
	}
	if c.FechaDoc != "" {
		fmt.Fprintf(&b, "**Fecha documento:** %s\n", c.FechaDoc)
	}
	fmt.Fprintf(&b, "**Tamaño:** %s chars\n", humanCount(c.CharCount))
	if c.Texto == "" {
		b.WriteString("\n*Texto sin contenido*\n")
	} else {
		fmt.Fprintf(&b, "\n## Texto\n\n%s\n", c.Texto)
	}
	// Citación con PDFs
	hasPDF := c.PDF != "" || c.PDF2 != ""
	if hasPDF {
		b.WriteString("\n---\n**Citación:**\n")
		if c.PDF != "" {
			fmt.Fprintf(&b, "- PDF: %s\n", c.PDF)
		}
		if c.PDF2 != "" {
			fmt.Fprintf(&b, "- PDF2: %s\n", c.PDF2)
		}
	}
	return b.String()
}
