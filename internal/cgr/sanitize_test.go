package cgr

import (
	"strings"
	"testing"
)

func TestSanitize_StripsTags(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`<font>hello</font>`, "hello"},
		{`<td>world</td>`, "world"},
		{`<ul><li>PRESIDENTA</li><li>ALCALDE</li></ul>`, "PRESIDENTAALCALDE"}, // tags stripped; normalize collapses
		{`<div><p>Ampliación</p> <b>Unidad</b></div>`, "Ampliación Unidad"},
		{`plain text`, "plain text"},
		{`<p>line1</p><p>line2</p>`, "line1line2"},
	}
	for _, tc := range cases {
		got := SanitizeTexto(tc.in)
		if got != tc.want {
			t.Errorf("SanitizeTexto(%q) = %q, want %q", tc.in, got, tc.want)
		}
		// also check per-type wrappers delegate correctly
		if got2 := SanitizeContableTexto(tc.in); got2 != tc.want {
			t.Errorf("SanitizeContableTexto(%q) = %q, want %q", tc.in, got2, tc.want)
		}
		if got3 := SanitizeLegislacionTexto(tc.in); got3 != tc.want {
			t.Errorf("SanitizeLegislacionTexto(%q) = %q, want %q", tc.in, got3, tc.want)
		}
		if got4 := SanitizeCuentaTexto(tc.in); got4 != tc.want {
			t.Errorf("SanitizeCuentaTexto(%q) = %q, want %q", tc.in, got4, tc.want)
		}
	}
	// Destinatarios with list items should not retain tags
	if got := SanitizeDestinatarios("<ul><li>PRESIDENTA DE LA REPÚBLICA</li></ul>"); strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("SanitizeDestinatarios still contains tags: %q", got)
	}
}

func TestSanitize_StripXML(t *testing.T) {
	raw := `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><meta name="pdf:PDFVersion" content="1.4"/><meta name="other" content="x"/></head><body><p>contenido informe</p></body></html>`
	got := SanitizeAuditoriaContenido(raw)
	if strings.Contains(got, "<?xml") {
		t.Errorf("SanitizeAuditoriaContenido still contains <?xml: %q", got)
	}
	if strings.Contains(got, "pdf:PDFVersion") {
		t.Errorf("SanitizeAuditoriaContenido still contains pdf:PDFVersion: %q", got)
	}
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("SanitizeAuditoriaContenido still contains tags: %q", got)
	}
	if !strings.Contains(got, "contenido informe") {
		t.Errorf("SanitizeAuditoriaContenido missing expected text: %q", got)
	}

	// Case insensitive XML header
	raw2 := `<?XML VERSION="1.0"?><META NAME="pdf:PDFVersion" CONTENT="1.7"/>hello`
	got2 := SanitizeAuditoriaContenido(raw2)
	if strings.Contains(strings.ToLower(got2), "xml") && strings.Contains(got2, "?") {
		t.Errorf("case-insensitive xml header not stripped: %q", got2)
	}
	if got2 != "hello" {
		t.Errorf("SanitizeAuditoriaContenido(%q) = %q, want %q", raw2, got2, "hello")
	}

	// SanitizeTexto should also strip generic tags but not specifically handle xml header as distinct step
	// It still removes <...> so <?xml ...?> would be removed as a tag as well.
	if got3 := SanitizeTexto(`<?xml version="1.0"?><p>hi</p>`); strings.Contains(got3, "<?xml") {
		t.Errorf("SanitizeTexto should strip xml-like tag: %q", got3)
	}
}

func TestSanitize_NoInternalURLLeak(t *testing.T) {
	// These values should never be exposed via sanitize output when filtered in client.
	// Here we verify sanitizers do not inject or preserve internal URL patterns
	// when given normal content, and that stripped output never contains raw internal markers.
	internals := []string{
		"http://172.30.21.160/old/path",
		"old_url",
		"texto_raw",
		"172.30.x",
	}
	for _, s := range internals {
		// SanitizeTexto should preserve the literal if it appears as content text,
		// but the client must filter fields before calling sanitize. Test that
		// none of the named funcs accidentally reference those field names internally.
		// This is a structural guarantee: no sanitize func should output the field name itself when given empty input.
		if got := SanitizeTexto(""); strings.Contains(got, s) {
			t.Errorf("SanitizeTexto empty leaked %q: %q", s, got)
		}
	}
	// Real leak check: when _source contains old_url field, client discards it before sanitize.
	// Ensure sanitize does not reintroduce tags that could hide URLs.
	raw := `<a href="http://172.30.21.160/secret">ver documento</a>`
	got := SanitizeTexto(raw)
	if strings.Contains(got, "172.30.21.160") {
		t.Errorf("SanitizeTexto should strip href with internal IP but got %q", got)
	}
	if strings.Contains(got, "<a") {
		t.Errorf("SanitizeTexto still contains <a tag: %q", got)
	}
	if got != "ver documento" {
		t.Errorf("SanitizeTexto(%q) = %q, want %q", raw, got, "ver documento")
	}
}

func TestNormalize_ControlChars(t *testing.T) {
	// Control chars should be dropped
	if got := SanitizeTexto("a\x00b\x01c\x1f d"); got != "abc d" {
		t.Errorf("control chars not stripped: %q", got)
	}
	// \n and \t preserved via normalize handling
	if got := SanitizeTexto("a\nb"); got != "a\nb" {
		t.Errorf("newline not preserved: %q", got)
	}
	// zero-width and BOM dropped
	if got := SanitizeTexto("a\u200bb\ufeffc"); got != "abc" {
		t.Errorf("zero-width/BOM not stripped: %q", got)
	}
	// &nbsp; decoded to U+00A0 then normalized to space
	if got := SanitizeTexto("N°&nbsp;E179593"); got != "N° E179593" {
		t.Errorf("&nbsp; not normalized: %q", got)
	}
	// ensure html entities decoded before normalize
	// Note: stripTags runs after UnescapeString, so encoded tags like &lt;font&gt; are decoded then stripped.
	cases := []struct{ in, want string }{
		{"&#x201C;hola&#x201D;", "\u201Chola\u201D"}, // curly quotes preserved
		{"&#xF3; &#243;", "ó ó"},                     // hex and decimal
		{"&amp;", "&"},
		{"&lt;font&gt;hola&lt;/font&gt;", "hola"}, // encoded tag decoded then stripped
		{"&nbsp;test", "test"},                    // leading nbsp -> trimmed by normalize
	}
	for _, tc := range cases {
		if got := SanitizeTexto(tc.in); got != tc.want {
			t.Errorf("SanitizeTexto(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// spacesToNormalize: ensp, emsp should become plain space
	if got := SanitizeTexto("a\u2002b\u2003c"); got != "a b c" {
		t.Errorf("ensp/emsp not normalized: %q", got)
	}
}

func TestSanitizeTruncateWithNotice(t *testing.T) {
	s := "abcdefghij"
	if got, truncated := truncateWithNotice(s, 5); !truncated || got != "abcde... [truncado, ver PDF oficial]" {
		t.Errorf("truncateWithNotice not truncating correctly: got %q truncated=%v", got, truncated)
	}
	if got, truncated := truncateWithNotice(s, 10); truncated || got != s {
		t.Errorf("truncateWithNotice should not truncate at limit: got %q truncated=%v", got, truncated)
	}
	if got, truncated := truncateWithNotice(s, 20); truncated || got != s {
		t.Errorf("truncateWithNotice should not truncate when limit > len: got %q truncated=%v", got, truncated)
	}
	// rune-aware: emoji counts as one rune
	emoji := "😀😀😀😀😀"
	if got, truncated := truncateWithNotice(emoji, 3); !truncated || got != "😀😀😀... [truncado, ver PDF oficial]" {
		t.Errorf("truncateWithNotice rune handling failed: got %q", got)
	}
}

func TestSanitize_PerTypeWrappers(t *testing.T) {
	raw := `<p>&nbsp;Hola&nbsp;mundo</p>`
	want := "Hola mundo"
	wrappers := []struct {
		name string
		fn   func(string) string
	}{
		{"SanitizeContableParte", SanitizeContableParte},
		{"SanitizeParte", SanitizeParte},
		{"SanitizeDestinatarios", SanitizeDestinatarios},
		{"SanitizeLegislacionMaterias", SanitizeLegislacionMaterias},
		{"SanitizeConsolidadoResena", SanitizeConsolidadoResena},
		{"SanitizeConsolidadoContenido", SanitizeConsolidadoContenido},
		{"SanitizeAuditoriaObjetivo", SanitizeAuditoriaObjetivo},
		{"SanitizeAuditoriaConclusiones", SanitizeAuditoriaConclusiones},
		{"SanitizeInstructivoDocumento", SanitizeInstructivoDocumento},
	}
	for _, w := range wrappers {
		if got := w.fn(raw); got != want {
			t.Errorf("%s(%q) = %q, want %q", w.name, raw, got, want)
		}
	}
}
