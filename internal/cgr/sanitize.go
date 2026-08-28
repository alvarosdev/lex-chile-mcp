package cgr

import (
	"html"
	"slices"
	"strings"
)

// SanitizeDocumento cleans the documento_completo field of dictamen
// responses: decodes HTML entities and normalizes whitespace. Returns
// LLM-readable plain text. Duplicated normalize logic from bcn to keep
// cgr decoupled.
func SanitizeDocumento(raw string) string {
	s := html.UnescapeString(raw)
	s = resumenTagRe.ReplaceAllString(s, "")
	return normalize(s)
}

// SanitizeMateria cleans the materia field of search results.
func SanitizeMateria(raw string) string {
	s := html.UnescapeString(raw)
	return normalize(s)
}

// SanitizeTexto is the generic CGR text sanitizer: decodes HTML entities,
// strips any HTML/XML tags, and normalizes whitespace. All per-type
// sanitizers delegate here unless they need extra steps (e.g. XML header).
func SanitizeTexto(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeAuditoriaContenido cleans auditoria contenido_pdf which is an
// XML-extracted dump prefixed with <?xml ...> and <meta name="pdf:PDFVersion" ...>.
// It decodes entities, strips the XML header/meta, strips remaining tags, and normalizes.
func SanitizeAuditoriaContenido(raw string) string {
	s := html.UnescapeString(raw)
	s = stripXMLHeader(s)
	s = stripTags(s)
	return normalize(s)
}

// --- Per-type named wrappers (keep for clarity and testability) ---

// SanitizeContableTexto cleans contable _source.texto.
func SanitizeContableTexto(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeContableParte cleans contable _source.parte.
func SanitizeContableParte(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeParte is a generic alias for parte fields.
func SanitizeParte(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeDestinatarios cleans destinatarios fields (may contain <ul><li>).
func SanitizeDestinatarios(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeLegislacionTexto cleans legislacion _source.texto_ .
func SanitizeLegislacionTexto(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeLegislacionMaterias cleans legislacion materias.
func SanitizeLegislacionMaterias(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeConsolidadoResena cleans consolidados resena.
func SanitizeConsolidadoResena(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeConsolidadoContenido cleans consolidados contenido_extraido.
func SanitizeConsolidadoContenido(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeCuentaTexto cleans cuentas _source.texto.
func SanitizeCuentaTexto(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeAuditoriaObjetivo cleans auditoria objetivo.
func SanitizeAuditoriaObjetivo(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeAuditoriaConclusiones cleans auditoria conclusiones.
func SanitizeAuditoriaConclusiones(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// SanitizeInstructivoDocumento cleans instructivo documento_completo.
func SanitizeInstructivoDocumento(raw string) string {
	s := html.UnescapeString(raw)
	s = stripTags(s)
	return normalize(s)
}

// stripTags removes any HTML/XML tags using the centralized tagRe.
func stripTags(s string) string {
	return tagRe.ReplaceAllString(s, "")
}

// stripXMLHeader removes the XML prolog and pdf:PDFVersion meta tag that
// prefix auditoria contenido_pdf dumps.
func stripXMLHeader(s string) string {
	s = xmlHeaderRe.ReplaceAllString(s, "")
	s = pdfMetaRe.ReplaceAllString(s, "")
	return s
}

// truncateWithNotice truncates s to limit runes and appends a truncation
// notice. Returns the (possibly truncated) string and whether truncation occurred.
func truncateWithNotice(s string, limit int) (string, bool) {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit]) + "... [truncado, ver PDF oficial]", true
	}
	return s, false
}

// normalize is the single-pass state machine that strips the garbage defined
// in garbage.go:
//
//   - spacesToNormalize runes become a plain space
//   - zeroWidthChars and C0 control runes are dropped
//   - consecutive spaces collapse (pending-space technique)
//   - trailing whitespace per line is dropped (spaces before \n)
//   - leading whitespace at line start is dropped
//
// Quotes and links are preserved — they are content, not garbage.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	atLineStart := true
	newlineRun := 0
	for _, r := range s {
		switch {
		case isSpaceToNormalize(r):
			r = ' '
		case isZeroWidth(r):
			continue
		case isControl(r):
			continue
		}
		if r == ' ' {
			pendingSpace = true
			continue
		}
		if r == '\n' {
			pendingSpace = false // trim trailing whitespace per line
			atLineStart = true
			// Collapse runs of 3+ newlines to at most 2.
			if newlineRun < 2 {
				b.WriteRune(r)
			}
			newlineRun++
			continue
		}
		if pendingSpace && !atLineStart {
			b.WriteByte(' ')
		}
		pendingSpace = false
		atLineStart = false
		newlineRun = 0
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func isSpaceToNormalize(r rune) bool {
	return slices.Contains(spacesToNormalize, r)
}

func isZeroWidth(r rune) bool {
	return slices.Contains(zeroWidthChars, r)
}

func isControl(r rune) bool {
	return r >= controlMin && r <= controlMax && r != '\n' && r != '\t'
}
