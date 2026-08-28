package cgr

// SearchParams carries the arguments for a Contraloría search.
type SearchParams struct {
	Query       string
	ExactSearch bool
	Order       string // "date" | "dateasc" | "score"
	Page        int    // 1-indexed for caller (0 means default 1)
	Source      string // "dictamenes" | "instructivos" | "contable" | "auditoria" | "legislacion" | "cuentas" | "consolidados" | "web" | "todos"
}

// SearchResponse is the paginated result of a Contraloría search.
type SearchResponse struct {
	Results    []DictamenSummary `json:"results"`
	Pagination Pagination        `json:"pagination"`
}

// DictamenSummary is the lightweight projection of a dictamen — the fields
// that answer "what is this dictamen about", without the full document.
// Mirrors the minimal + carácter decision from design.
type DictamenSummary struct {
	DictamenID   string `json:"dictamen_id"`
	NDictamen    string `json:"n_dictamen"`
	NumericID    string `json:"numeric_doc_id"`
	FechaDoc     string `json:"fecha_documento"`
	Materia      string `json:"materia"`
	Descriptores string `json:"descriptores"`
	Criterio     string `json:"criterio"`
	Origen       string `json:"origen"`
	Caracter     string `json:"caracter"`
	URL          string `json:"url"`
	PDFURL       string `json:"pdf_url"`
}

// DictamenFull is the full structured content of one dictamen, including
// the sanitized document. DictamenSummary is embedded for reuse (parity with
// NormaSummary projection in bcn).
type DictamenFull struct {
	DictamenSummary `json:",inline"`
	Destinatarios   string `json:"destinatarios"`
	Abogados        string `json:"abogados"`
	FuentesLegales  string `json:"fuentes_legales"`
	Documento       string `json:"documento_completo"`
	CharCount       int    `json:"char_count"`
}

// InstructivoSummary is the lightweight projection of an instructivo.
type InstructivoSummary struct {
	InstructivoID string `json:"instructivo_id"`
	NDictamen     string `json:"n_dictamen"`
	NumericID     string `json:"numeric_doc_id"`
	FechaDoc      string `json:"fecha_documento"`
	Materia       string `json:"materia"`
	Descriptores  string `json:"descriptores"`
	Caracter      string `json:"caracter"`
	URL           string `json:"url"`
	PDFURL        string `json:"pdf_url"`
}

// InstructivoFull is the full content of an instructivo.
type InstructivoFull struct {
	InstructivoSummary `json:",inline"`
	Destinatarios      string `json:"destinatarios"`
	Abogados           string `json:"abogados"`
	FuentesLegales     string `json:"fuentes_legales"`
	Documento          string `json:"documento_completo"`
	CharCount          int    `json:"char_count"`
}

// ContableSummary is the lightweight projection of a contable oficio.
type ContableSummary struct {
	DocID             string `json:"doc_id"`
	Numero            string `json:"numero"`
	NormativaContable string `json:"normativa_contable"`
	Tipo              string `json:"tipo"`
	Parte             string `json:"parte"`
	Destinatarios     string `json:"destinatarios"`
	FechaDoc          string `json:"fecha_documento"`
	Origen            string `json:"origen"`
}

// ContableFull is the full content of a contable oficio.
type ContableFull struct {
	ContableSummary `json:",inline"`
	Texto           string `json:"texto"`
	CharCount       int    `json:"char_count"`
}

// AuditoriaSummary is the lightweight projection of an auditoria informe.
type AuditoriaSummary struct {
	DocID     string `json:"doc_id"`
	Numero    string `json:"numero"`
	Nombre    string `json:"nombre"`
	Tipo      string `json:"tipo"`
	Objetivo  string `json:"objetivo"`
	UnidadCgr string `json:"unidad_cgr"`
	FechaDoc  string `json:"fecha_documento"`
	PDF       string `json:"pdf"`
}

// AuditoriaFull is the full content of an auditoria informe.
type AuditoriaFull struct {
	AuditoriaSummary `json:",inline"`
	Conclusiones     string `json:"conclusiones"`
	Destinatarios    string `json:"destinatarios"`
	ContenidoPDF     string `json:"contenido_pdf"`
	CharCount        int    `json:"char_count"`
}

// ConsolidadoSummary is the lightweight projection of a consolidado CIC.
type ConsolidadoSummary struct {
	Numero    string `json:"numero"`
	Nombre    string `json:"nombre"`
	Resena    string `json:"resena"`
	Tipo      string `json:"tipo"`
	FechaDoc  string `json:"fecha_documento"`
	UnidadCgr string `json:"unidad_cgr"`
	PDFWeb    string `json:"documento_cic_pdf_web"`
}

// ConsolidadoFull is the full content of a consolidado CIC.
type ConsolidadoFull struct {
	ConsolidadoSummary `json:",inline"`
	ContenidoExtraido  string `json:"contenido_extraido"`
	CharCount          int    `json:"char_count"`
}

// CuentaSummary is the lightweight projection of a cuenta sentencia.
type CuentaSummary struct {
	DocID            string `json:"doc_id"`
	NumeroSentencia  string `json:"numero_sentencia"`
	NumeroExpediente string `json:"numero_expediente"`
	Texto            string `json:"texto"`
	FechaSentencia   string `json:"fecha_sentencia"`
	FechaDoc         string `json:"fecha_documento"`
	PDF              string `json:"pdf"`
	PDF2             string `json:"pdf2"`
}

// CuentaFull is the full content of a cuenta sentencia.
type CuentaFull struct {
	CuentaSummary `json:",inline"`
	CharCount     int `json:"char_count"`
}

// LegislacionSummary is the lightweight projection of a legislacion entry.
type LegislacionSummary struct {
	DocID     string `json:"doc_id"`
	Tipo      string `json:"tipo"`
	Numero    string `json:"numero"`
	Organismo string `json:"organismo"`
	Materias  string `json:"materias"`
	Caracter  string `json:"caracter"`
	FechaDoc  string `json:"fecha_documento"`
}

// LegislacionFull is the full content of a legislacion entry.
type LegislacionFull struct {
	LegislacionSummary `json:",inline"`
	Texto              string `json:"texto"`
	CharCount          int    `json:"char_count"`
}

// Pagination reports the total result count so callers can page through.
// Page is 1-indexed, PageSize always 20 for CGR.
type Pagination struct {
	Total      int  `json:"total"`
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}

// CountResponse is the aggregated count cross-type for a query.
type CountResponse struct {
	Query   string        `json:"query"`
	Total   int           `json:"total"`
	Buckets []CountBucket `json:"buckets"`
}

// CountBucket is one entry of the count_by_type aggregation.
type CountBucket struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// Wire types for Elasticsearch responses — internal, not exposed in
// structuredContent. They decode the ES envelope and are projected to the
// public types above.

type cgrSearchResponse struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []cgrHit `json:"hits"`
	} `json:"hits"`
}

type cgrHit struct {
	ID     string    `json:"_id"`
	Index  string    `json:"_index"`
	Score  float64   `json:"_score"`
	Source cgrSource `json:"_source"`
}

type cgrSource struct {
	DocID          string `json:"doc_id"`
	NDictamen      string `json:"n_dictamen"`
	NumericDocID   string `json:"numeric_doc_id"`
	FechaDocumento string `json:"fecha_documento"`
	Carater        string `json:"carácter"`
	Materia        string `json:"materia"`
	Descriptores   string `json:"descriptores"`
	Criterio       string `json:"criterio"`
	Origen         string `json:"origen_"`
	Destinatarios  string `json:"destinatarios"`
	Abogados       string `json:"abogados"`
	FuentesLegales string `json:"fuentes_legales"`
	Documento      string `json:"documento_completo"`
	OldURL         string `json:"old_url"`
}

// Generic envelope for typed sources.

type cgrSearchEnvelope[T any] struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []cgrHitEnvelope[T] `json:"hits"`
	} `json:"hits"`
}

type cgrHitEnvelope[T any] struct {
	ID     string  `json:"_id"`
	Index  string  `json:"_index"`
	Score  float64 `json:"_score"`
	Source T       `json:"_source"`
}

// Wire types per source.

type cgrInstructivoSource struct {
	DocID          string `json:"doc_id"`
	NDictamen      string `json:"n_dictamen"`
	NumericDocID   string `json:"numeric_doc_id"`
	FechaDocumento string `json:"fecha_documento"`
	Carater        string `json:"carácter"`
	Materia        string `json:"materia"`
	Descriptores   string `json:"descriptores"`
	Criterio       string `json:"criterio"`
	Origen         string `json:"origen_"`
	Destinatarios  string `json:"destinatarios"`
	Abogados       string `json:"abogados"`
	FuentesLegales string `json:"fuentes_legales"`
	Documento      string `json:"documento_completo"`
	OldURL         string `json:"old_url"`
}

type cgrContableSource struct {
	DocID             string `json:"doc_id"`
	Numero            string `json:"número"`
	NumeroAlt         string `json:"numero"`
	NormativaContable string `json:"normativa_contable"`
	Tipo              string `json:"tipo"`
	Parte             string `json:"parte"`
	Destinatarios     string `json:"destinatarios"`
	Origen            string `json:"origen"`
	OrigenAlt         string `json:"origen_"`
	FechaDocumento    string `json:"fecha_documento"`
	Texto             string `json:"texto"`
	TextoRaw          string `json:"texto_raw"`
	OldURL            string `json:"old_url"`
}

type cgrAuditoriaSource struct {
	DocID          string `json:"doc_id"`
	NumericDocID   string `json:"numeric_doc_id"`
	Numero         string `json:"número"`
	NumeroAlt      string `json:"numero"`
	Nombre         string `json:"nombre"`
	Tipo           string `json:"tipo"`
	Objetivo       string `json:"objetivo"`
	Conclusiones   string `json:"conclusiones"`
	ContenidoPDF   string `json:"contenido_pdf"`
	PDF            string `json:"pdf"`
	Destinatarios  string `json:"destinatarios"`
	UnidadCgr      string `json:"unidad_cgr"`
	FechaDocumento string `json:"fecha_documento"`
	FechaDocAlt    string `json:"fecha_documento_"`
	OldURL         string `json:"old_url"`
}

type cgrConsolidadoSource struct {
	Numero            string `json:"numero"`
	Nombre            string `json:"nombre"`
	Resena            string `json:"resena"`
	Tipo              string `json:"tipo"`
	TipoAlt           string `json:"cra_tipo"`
	FechaDocumento    string `json:"fecha_documento"`
	UnidadCgr         string `json:"unidad_cgr"`
	ContenidoExtraido string `json:"contenido_extraido"`
	PDFWeb            string `json:"documento_cic_pdf_web"`
	OldURL            string `json:"old_url"`
}

type cgrCuentaSource struct {
	DocID            string `json:"doc_id"`
	NumericDocID     string `json:"numeric_doc_id"`
	NumeroSentencia  string `json:"numero_sentencia"`
	NumeroExpediente string `json:"numero_expediente"`
	Texto            string `json:"texto"`
	FechaSentencia   string `json:"fecha_sentencia"`
	FechaDocumento   string `json:"fecha_documento"`
	PDF              string `json:"pdf"`
	PDF2             string `json:"pdf2"`
	OldURL           string `json:"old_url"`
}

type cgrLegislacionSource struct {
	DocID          string `json:"doc_id"`
	NumericDocID   string `json:"numeric_doc_id"`
	Tipo           string `json:"tipo"`
	Numero         string `json:"número"`
	NumeroAlt      string `json:"numero"`
	Organismo      string `json:"organismo"`
	Materias       string `json:"materias"`
	Caracter       string `json:"caracter"`
	FechaDocumento string `json:"fecha_documento"`
	Texto          string `json:"texto_"`
	TextoRaw       string `json:"texto__raw"`
	OldURL         string `json:"old_url"`
}

type cgrCountResponse struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
	} `json:"hits"`
	Aggregations struct {
		CountByType struct {
			Buckets []struct {
				Key      string `json:"key"`
				DocCount int    `json:"doc_count"`
			} `json:"buckets"`
		} `json:"count_by_type"`
	} `json:"aggregations"`
}
