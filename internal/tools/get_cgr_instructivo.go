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

// GetCgrInstructivoArgs carries the arguments of the get_cgr_instructivo tool.
type GetCgrInstructivoArgs struct {
	InstructivoID string `json:"instructivo_id" jsonschema:"instructivo id, e.g. IN23N26 or E462387N24 from search"`
}

// GetCgrInstructivoOutput is the structured content of get_cgr_instructivo.
type GetCgrInstructivoOutput struct {
	InstructivoID string `json:"instructivo_id"`
	NDictamen     string `json:"n_dictamen"`
	FechaDoc      string `json:"fecha_documento"`
	Materia       string `json:"materia"`
	Descriptores  string `json:"descriptores"`
	Caracter      string `json:"caracter"`
	Documento     string `json:"documento_completo"`
	CharCount     int    `json:"char_count"`
	URL           string `json:"url"`
	PDFURL        string `json:"pdf_url"`
}

var instructivoIDToolRe = regexp.MustCompile(`^[A-Z]{0,3}[0-9]{1,6}N[0-9]{2}$`)

// RegisterGetCgrInstructivo registers the get_cgr_instructivo tool.
func RegisterGetCgrInstructivo(srv *mcp.Server, client cgr.CgrClient) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_cgr_instructivo",
		Description: "Get an instructivo by its instructivo_id (from search results, e.g. IN23N26 or E462387N24). Returns metadata (materia, descriptores, caracter) and the sanitized documento_completo with char_count and HTML/PDF URLs for citation and PDF download.",
	}, makeGetCgrInstructivo(client))
}

func makeGetCgrInstructivo(client cgr.CgrClient) mcp.ToolHandlerFor[GetCgrInstructivoArgs, GetCgrInstructivoOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCgrInstructivoArgs) (*mcp.CallToolResult, GetCgrInstructivoOutput, error) {
		if args.InstructivoID == "" {
			return errorResult("instructivo_id is required"), GetCgrInstructivoOutput{}, nil
		}
		id := strings.TrimSpace(strings.ToUpper(args.InstructivoID))
		if len(id) > 40 {
			return errorResult(fmt.Sprintf("invalid instructivo_id %q: too long", id)), GetCgrInstructivoOutput{}, nil
		}
		if strings.ContainsAny(id, "/.%?#\n\r") {
			return errorResult(fmt.Sprintf("invalid instructivo_id %q", id)), GetCgrInstructivoOutput{}, nil
		}
		if !instructivoIDToolRe.MatchString(id) {
			return errorResult(fmt.Sprintf("invalid instructivo_id %q", id)), GetCgrInstructivoOutput{}, nil
		}
		full, err := client.GetInstructivo(ctx, id)
		if err != nil {
			if errors.Is(err, cgr.ErrInstructivoNotFound) || errors.Is(err, cgr.ErrDictamenNotFound) {
				return errorResult(fmt.Sprintf("instructivo not found: instructivo_id %q does not exist", id)), GetCgrInstructivoOutput{}, nil
			}
			return errorResult(fmt.Sprintf("get cgr instructivo failed: %v", err)), GetCgrInstructivoOutput{}, nil
		}
		output := GetCgrInstructivoOutput{
			InstructivoID: full.InstructivoID,
			NDictamen:     full.NDictamen,
			FechaDoc:      full.FechaDoc,
			Materia:       full.Materia,
			Descriptores:  full.Descriptores,
			Caracter:      full.Caracter,
			Documento:     full.Documento,
			CharCount:     full.CharCount,
			URL:           full.URL,
			PDFURL:        full.PDFURL,
		}
		if output.InstructivoID == "" {
			output.InstructivoID = id
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatInstructivo(full)},
			},
		}, output, nil
	}
}

func formatInstructivo(in cgr.InstructivoFull) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Instructivo %s — %s (%s)\n\n", in.InstructivoID, in.NDictamen, in.FechaDoc)
	fmt.Fprintf(&b, "**Materia:** %s\n", in.Materia)
	if in.Descriptores != "" {
		fmt.Fprintf(&b, "**Descriptores:** %s\n", in.Descriptores)
	}
	fmt.Fprintf(&b, "**Carácter:** %s", in.Caracter)
	if in.Destinatarios != "" {
		fmt.Fprintf(&b, " | **Destinatarios:** %s", in.Destinatarios)
	}
	if in.Abogados != "" {
		fmt.Fprintf(&b, " | **Abogados:** %s", in.Abogados)
	}
	b.WriteString("\n")
	if in.FuentesLegales != "" {
		fmt.Fprintf(&b, "**Fuentes legales:** %s\n", in.FuentesLegales)
	}
	fmt.Fprintf(&b, "**Tamaño:** %s chars\n", humanCount(in.CharCount))
	if in.Documento == "" {
		b.WriteString("\n*Documento sin contenido*\n")
	} else {
		fmt.Fprintf(&b, "\n## Documento Completo\n\n%s\n", in.Documento)
	}
	b.WriteString("\n---\n**Citación:**\n")
	fmt.Fprintf(&b, "- Visualización: %s\n", in.URL)
	fmt.Fprintf(&b, "- Descarga PDF: %s\n", in.PDFURL)
	return b.String()
}
