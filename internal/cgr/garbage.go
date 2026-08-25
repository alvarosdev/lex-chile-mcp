// This file centralizes every piece of "garbage" that Contraloría responses
// carry, so the sanitizer never deals with magic strings or raw rune
// literals. Duplicated from internal/bcn/garbage.go to avoid cgr -> bcn
// dependency — keep the two in sync when adding new garbage.
package cgr

import "regexp"

const (
	// nbspRune is the non-breaking space (&nbsp;, U+00A0). CGR uses it as
	// visual indentation and inside documento_completo (e.g. "N°  E179593").
	nbspRune = '\u00a0'
	// enspRune is the en-space (&ensp;, U+2002) variant.
	enspRune = '\u2002'
	// emspRune is the em-space (&emsp;, U+2003) variant.
	emspRune = '\u2003'
	// zeroWidthSpaceRune is U+200B, occasionally embedded in CGR text.
	zeroWidthSpaceRune = '\u200b'
	// bomRune is the byte-order mark (U+FEFF) sometimes embedded in responses.
	bomRune = '\ufeff'
	// controlMin and controlMax delimit the C0 control block. The sanitizer
	// removes it, keeping only newline, tab and carriage return.
	controlMin = '\u0000'
	controlMax = '\u001f'
)

// spacesToNormalize lists every whitespace rune that must become a plain
// space before the text reaches the LLM.
var spacesToNormalize = []rune{nbspRune, enspRune, emspRune}

// zeroWidthChars lists invisible runes that are dropped entirely.
var zeroWidthChars = []rune{zeroWidthSpaceRune, bomRune}

// resumenTagRe matches the XML wrapper that sometimes surrounds resumen-like
// fields — kept for parity with bcn, though CGR documento_completo is plain.
var resumenTagRe = regexp.MustCompile(`(?i)</?resumen[^>]*>`)

// tagRe strips any HTML/XML tag (e.g. <font>, <td>, <ul><li>).
var tagRe = regexp.MustCompile(`<[^>]+>`)

// xmlHeaderRe strips the XML prolog that prefixes auditoria contenido_pdf
// (e.g. <?xml version="1.0" encoding="UTF-8"?>).
var xmlHeaderRe = regexp.MustCompile(`(?i)<\?xml[^>]*\?>`)

// pdfMetaRe strips the pdf:PDFVersion meta tag that appears in auditoria
// contenido_pdf headers (e.g. <meta name="pdf:PDFVersion" content="1.4"/>).
var pdfMetaRe = regexp.MustCompile(`(?i)<meta[^>]*pdf:[^>]*>`)
