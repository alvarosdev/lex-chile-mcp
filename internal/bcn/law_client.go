// Package bcn provides the LeyChile (Biblioteca del Congreso Nacional)
// client: search and retrieval of Chilean legal norms. All HTTP behavior
// (timeout, retry, circuit breaker) is configured per resource from the
// api.resources.yaml contract — no hardcoded URLs or policies in code.
package bcn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/sync/singleflight"
	"resty.dev/v3"

	"github.com/alvarosdev/lex-chile-mcp/internal/config"
)

// Resource ids in api.resources.yaml.
const (
	resourceSearchLaws = "search_laws"
	resourceGetLaw     = "get_law"
	resourceGetLawHist = "get_law_history"
)

// ErrNormaNotFound is returned when the requested norm id does not exist
// in LeyChile (the API answers HTTP 500 with a "not found" body).
var ErrNormaNotFound = errors.New("norma not found")

// LawClient defines the operations the MCP tools need from the BCN API.
// Implemented by *Client; mocked by MockLawClient (mockery).
type LawClient interface {
	// Search returns paginated search results for a query string.
	Search(ctx context.Context, params SearchParams) (SearchResponse, error)
	// GetNorma returns the full structured content of one norm.
	GetNorma(ctx context.Context, q NormaQuery) (NormaFull, error)
	// GetNormaSummary returns the lightweight metadata view of one norm
	// (no content). Shares the ETag cache with GetNorma.
	GetNormaSummary(ctx context.Context, q NormaQuery) (NormaSummary, error)
	// GetLawHistory returns the legislative history groups of one norm.
	GetLawHistory(ctx context.Context, normID int64) ([]HistoriaGrupo, error)
}

// NormaQuery identifies what to fetch: a norm id and, optionally, the
// version in force at a given date (YYYY-MM-DD). Empty VersionDate means
// the latest version.
type NormaQuery struct {
	NormID      int64
	VersionDate string
}

// Client implements LawClient over resty, with one *resty.Client per
// resource — resty's circuit breaker is client-level. Clients are built
// once in NewClient and reused for every request (no per-call setup).
//
// Each converter instance is thread-safe via an internal mutex, which
// SERIALIZES conversions per instance: a single converter would make the
// conversion of one long law block every other request. The pool gives
// each in-flight conversion its own converter (conversion is the CPU hot
// spot of GetNorma).
type Client struct {
	logger     *slog.Logger
	resources  *config.Resources
	clients    map[string]*resty.Client
	converters sync.Pool
	flights    singleflight.Group
	normas     *etagCache[NormaFull]
	historias  *etagCache[[]HistoriaGrupo]
}

// NewClient builds a Client from the resources contract.
func NewClient(resources *config.Resources, logger *slog.Logger) *Client {
	c := &Client{
		logger:     logger,
		resources:  resources,
		clients:    make(map[string]*resty.Client, len(resources.Resources)),
		converters: sync.Pool{New: func() any { return newConverter() }},
		normas:     newEtagCache[NormaFull](),
		historias:  newEtagCache[[]HistoriaGrupo](),
	}
	for name, res := range resources.Resources {
		c.clients[name] = c.newRestyClient(name, res)
	}
	return c
}

// newRestyClient configures one resty client per resource: base URL,
// timeout, count-based circuit breaker, and error logging per attempt.
func (c *Client) newRestyClient(name string, res config.Resource) *resty.Client {
	return resty.New().
		SetBaseURL(res.URL).
		SetTimeout(time.Duration(res.Timeout)).
		SetCircuitBreaker(resty.NewCircuitBreakerCount(
			uint64(res.CircuitBreaker.FailureThreshold),
			uint64(res.CircuitBreaker.SuccessThreshold),
			time.Duration(res.CircuitBreaker.ResetTimeout),
		)).
		OnError(func(req *resty.Request, err error) {
			c.logger.Debug("bcn request attempt failed",
				"resource", name,
				"error", err,
			)
		})
}

// SearchParams carries the arguments for a LeyChile search.
type SearchParams struct {
	Query    string
	Page     int
	PageSize int
}

// SearchResponse is the paginated result of a LeyChile search.
//
// The wire format is a 3-element array [results, pagination, facets];
// UnmarshalJSON splits it into the typed fields. Facets (element 2) are
// intentionally ignored for now.
type SearchResponse struct {
	Results    []Norma
	Pagination Pagination
}

// UnmarshalJSON decodes the heterogeneous 3-element array of buscarjson.
func (r *SearchResponse) UnmarshalJSON(data []byte) error {
	var parts [3]json.RawMessage
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}
	if err := json.Unmarshal(parts[0], &r.Results); err != nil {
		return fmt.Errorf("decode results: %w", err)
	}
	if err := json.Unmarshal(parts[1], &r.Pagination); err != nil {
		return fmt.Errorf("decode pagination: %w", err)
	}
	return nil
}

// Norma is one search result. Field names mirror the raw API response.
type Norma struct {
	IDNorma          int64  `json:"IDNORMA"`
	Norma            string `json:"NORMA"`
	Tipo             string `json:"DESCRIPCION"`
	TituloNorma      string `json:"TITULO_NORMA"`
	Organismo        string `json:"ORGANISMO"`
	FechaPublicacion string `json:"FECHA_PUBLICACION"`
	Resumen          string `json:"RESUMEN"`
}

// Pagination reports the total result count so callers can page through.
// The numeric fields decode through FlexInt: buscarjson alternates between
// strings ("3") and numbers (3) for the same fields depending on the query
// (e.g. "Ley 21.600" answers strings, "Ley 21461" answers numbers) — a
// plain string type crashed the decode on the numeric shape.
type Pagination struct {
	TotalItems FlexInt `json:"totalitems"`
	Page       FlexInt `json:"npagina"`
	PageSize   FlexInt `json:"itemsporpagina"`
	Query      string  `json:"cadena"`
}

// NormaFull is the full structured content of one norm.
type NormaFull struct {
	Html       []HtmlBlock      `json:"html"`
	Proyectos  []Proyecto       `json:"proyectos"`
	Metadatos  Metadatos        `json:"metadatos"`
	Estructura []EstructuraPart `json:"estructura"`
}

// HtmlBlock is one content part of the norm (header, article, annex...).
// Blocks NEST via H: the API returns titles and paragraphs with their
// articles as children (e.g. Ley 21.600) — a flat parse loses the articles.
// Markdown and SectionName are filled by ConvertContent, not the API.
type HtmlBlock struct {
	T           string      `json:"t"`
	I           int64       `json:"i"`
	V           []string    `json:"v,omitempty"`
	H           []HtmlBlock `json:"h,omitempty"`
	Markdown    string      `json:"-"`
	SectionName string      `json:"-"`
}

// EstructuraPart is one entry of the norm's table of contents. The TOC
// nests via H too: T (1=título, 4=párrafo, 6=artículo) classifies entries.
type EstructuraPart struct {
	N string           `json:"n"`
	I int64            `json:"i"`
	T int              `json:"t,omitempty"`
	H []EstructuraPart `json:"h,omitempty"`
}

// Proyecto links the norm to its legislative bill (boletín).
type Proyecto struct {
	Categoria string `json:"categoria"`
	Pls       []struct {
		Enlace      string `json:"enlace"`
		Informacion string `json:"informacion"`
		NroBoletin  string `json:"nroBoletin"`
	} `json:"pls"`
}

// Metadatos holds the selected norm metadata.
type Metadatos struct {
	TiposNumeros     []TipoNumero  `json:"tipos_numeros"`
	Organismos       []string      `json:"organismos"`
	TituloNorma      string        `json:"titulo_norma"`
	Fuente           string        `json:"fuente"`
	NumeroFuente     string        `json:"numero_fuente"`
	Materias         []string      `json:"materias"`
	CategoriasNorma  []string      `json:"categorias_norma"`
	Derogado         bool          `json:"derogado"`
	FechaPublicacion string        `json:"fecha_publicacion"`
	Vigencia         Vigencia      `json:"vigencia"`
	Vinculaciones    []Vinculacion `json:"vinculaciones"`
	Resumenes        []string      `json:"resumenes"`
}

// TipoNumero identifies the norm type and number (e.g. Ley 21600).
// All five API fields are kept: the structured content carries complete
// data. CanonicalType/CanonicalAbbr are APPENDED (not replacing the raw
// values) from the official catalog; omitted when the code is unknown.
type TipoNumero struct {
	Tipo          string `json:"tipo"`
	Numero        string `json:"numero"`
	Abreviacion   string `json:"abreviacion"`
	Descripcion   string `json:"descripcion"`
	Compuesto     string `json:"compuesto"`
	CanonicalType string `json:"canonical_type,omitempty"`
	CanonicalAbbr string `json:"canonical_abbr,omitempty"`
}

// Vigencia is the norm's validity window.
type Vigencia struct {
	InicioVigencia string `json:"inicio_vigencia"`
	FinVigencia    string `json:"fin_vigencia"`
}

// Vinculacion is a relation to other norms (modification, concordance...).
type Vinculacion struct {
	Text    string `json:"text"`
	URLPart string `json:"url_part"`
}

// NormaSummary is the lightweight projection of a norm: the metadata fields
// that answer "what is this norm about", plus the flattened structure (with
// the section ids to drill down with get_law) and the size of the full
// content — without the content itself.
type NormaSummary struct {
	TituloNorma     string             `json:"titulo_norma"`
	Fuente          string             `json:"fuente"`
	Materias        []string           `json:"materias"`
	CategoriasNorma []string           `json:"categorias_norma"`
	Resumenes       []string           `json:"resumenes"`
	Estructura      []StructurePartOut `json:"estructura"`
	CharCount       int                `json:"char_count"`
	ArticleCount    int                `json:"article_count"`
}

// HistoriaGrupo is one group of the legislative history of a norm:
// its own history (1), laws that modified it (3), laws it modified (4).
type HistoriaGrupo struct {
	Titulo   string            `json:"titulo"`
	TipoDesc string            `json:"tipo_desc"`
	TipoCod  int               `json:"tipo_cod"`
	Hls      []HistoriaEntrada `json:"hls"`
}

// HistoriaEntrada is one entry of a history group.
//
// ID semantics (verified against the BCN data model):
//   - IDNormaHL is the LeyChile idNorma of the norm this record belongs to
//     (the one named in Descripcion/Bajada) — THE id for building ficha
//     links: https://www.leychile.cl/Navegar?idNorma=<id_norma_hl>.
//   - IDNorma (no suffix) points to the RELATED norm (the modificatoria or
//     modificada in this context) — never use it to link the record's norm.
//   - The number inside Enlace (e.g. /historia-de-la-ley/6910/) is the
//     Historia ID of the tramitación document — not a LeyChile norm id.
type HistoriaEntrada struct {
	Tipo        int    `json:"tipo"`
	IDNorma     int64  `json:"id_norma"`
	Enlace      string `json:"enlace"`
	Bajada      string `json:"bajada"`
	Fecha       string `json:"fecha"`
	Descripcion string `json:"descripcion"`
	IDNormaHL   int64  `json:"id_norma_hl"`
}

// newConverter builds the shared HTML→Markdown converter. The standard
// plugins (base + commonmark + table) already render BCN's div.p markup
// as paragraphs — verified with real API HTML in a spike; no custom rules
// are needed.
func newConverter() *converter.Converter {
	return converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
		),
	)
}

// ConvertContent converts every content block (recursively, blocks nest
// via H) to sanitized Markdown and attaches the section name from the
// structure tree (estructura[i] ↔ html[i]).
func (n *NormaFull) ConvertContent(conv *converter.Converter) {
	names := make(map[int64]string)
	var indexEstructura func(parts []EstructuraPart)
	indexEstructura = func(parts []EstructuraPart) {
		for _, part := range parts {
			names[part.I] = part.N
			indexEstructura(part.H)
		}
	}
	indexEstructura(n.Estructura)

	var convert func(blocks []HtmlBlock)
	convert = func(blocks []HtmlBlock) {
		for i := range blocks {
			md, err := conv.ConvertString(blocks[i].T)
			if err != nil {
				// Conversion errors are not fatal: fall back to sanitized raw text.
				md = blocks[i].T
			}
			blocks[i].Markdown = SanitizeMarkdown(md)
			blocks[i].SectionName = names[blocks[i].I]
			convert(blocks[i].H)
		}
	}
	convert(n.Html)
}

// retryConditions is the set of transient failures that trigger a retry:
// 5xx server errors and timeouts/connection failures (status zero).
// Declared explicitly — resty does not retry anything by default.
var retryConditions = []resty.RetryConditionFunc{
	resty.RetryConditionStatus5XX,
	resty.RetryConditionStatusZero,
}

// Search calls buscarjson with the query parameters, decodes the
// heterogeneous response and sanitizes every result summary.
func (c *Client) Search(ctx context.Context, params SearchParams) (SearchResponse, error) {
	res := c.resources.Resources[resourceSearchLaws]
	var result SearchResponse

	resp, err := c.clients[resourceSearchLaws].R().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"cadena":         params.Query,
			"npagina":        strconv.Itoa(params.Page),
			"itemsporpagina": strconv.Itoa(params.PageSize),
		}).
		SetRetryCount(res.Retry.Attempts).
		SetRetryWaitTime(time.Duration(res.Retry.Backoff)).
		SetRetryMaxWaitTime(time.Duration(res.Retry.MaxBackoff)).
		SetRetryConditions(retryConditions...).
		SetResult(&result).
		Get(res.Path)
	if err != nil {
		c.logger.Error("bcn search failed", "error", err)
		return SearchResponse{}, fmt.Errorf("search laws: %w", err)
	}
	if resp.IsStatusFailure() {
		return SearchResponse{}, fmt.Errorf("search laws: unexpected status %d", resp.StatusCode())
	}

	for i := range result.Results {
		result.Results[i].Resumen = SanitizeSummary(result.Results[i].Resumen)
	}
	c.logger.Debug("bcn search ok",
		"results", len(result.Results),
		"total_items", int(result.Pagination.TotalItems),
	)
	return result, nil
}

// normaCacheKey builds the cache key: norms are cached per (norm, version)
// — a historical version must never receive another version's cached entry
// (the API sends different ETags per version).
func normaCacheKey(q NormaQuery) string {
	return fmt.Sprintf("%d@%s", q.NormID, q.VersionDate)
}

// GetNorma calls get_norma_json for one norm (optionally the version in
// force at a date), converts the content to Markdown and maps the HTTP 500
// "not found" answer to ErrNormaNotFound.
//
// The response is cached per (norm, version) with ETag revalidation: a
// repeated request sends If-None-Match and a 304 serves the cached (already
// converted) norm without re-downloading or re-converting.
//
// Concurrent requests for the same (norm, version) are coalesced with
// singleflight: they share ONE call to BCN (retry and breaker run inside
// the flight). If the leader's context cancels, the flight errors for
// every waiter — the same timeout behavior as before.
func (c *Client) GetNorma(ctx context.Context, q NormaQuery) (NormaFull, error) {
	key := normaCacheKey(q)
	v, err, _ := c.flights.Do(key, func() (any, error) {
		return c.getNormaOnce(ctx, q, key)
	})
	if err != nil {
		return NormaFull{}, err
	}
	return v.(NormaFull), nil
}

// maxNormResponseBytes bounds the norm response body. The largest known
// norm is ~280KB, so 5MB leaves generous headroom while keeping a
// misbehaving upstream from exhausting memory (GOMEMLIMIT is 256MiB).
const maxNormResponseBytes = 5 << 20

// getNormaOnce is the single-flight body of GetNorma: cache lookup, HTTP
// with ETag revalidation, bounded body read, decode, convert and store.
func (c *Client) getNormaOnce(ctx context.Context, q NormaQuery, key string) (NormaFull, error) {
	res := c.resources.Resources[resourceGetLaw]
	entry, cached := c.normas.get(key)

	req := c.clients[resourceGetLaw].R().
		SetContext(ctx).
		SetQueryParam("idNorma", strconv.FormatInt(q.NormID, 10)).
		SetRetryCount(res.Retry.Attempts).
		SetRetryWaitTime(time.Duration(res.Retry.Backoff)).
		SetRetryMaxWaitTime(time.Duration(res.Retry.MaxBackoff)).
		SetRetryConditions(retryConditions...).
		// The body is read manually with a hard size limit instead of
		// auto-parsing with SetResult (guard OOM — see maxNormResponseBytes).
		SetResponseDoNotParse(true)
	if q.VersionDate != "" {
		req.SetQueryParam("idVersion", q.VersionDate)
	}
	if cached {
		req.SetHeader("If-None-Match", entry.etag)
	}

	resp, err := req.Get(res.Path)
	if err != nil {
		c.logger.Error("bcn get norm failed", "norm_id", q.NormID, "version_date", q.VersionDate, "error", err)
		return NormaFull{}, fmt.Errorf("get norm: %w", err)
	}
	defer resp.Body.Close()

	// A 304 on a cached norm is a hit: serve the cached copy.
	if resp.StatusCode() == http.StatusNotModified && cached {
		c.logger.Debug("bcn get norm cache hit", "norm_id", q.NormID, "version_date", q.VersionDate)
		return entry.value, nil
	}
	if resp.StatusCode() == http.StatusInternalServerError {
		// The API answers nonexistent ids with HTTP 500 and a "no se
		// encuentra en nuestra Base de Datos" body.
		return NormaFull{}, ErrNormaNotFound
	}
	if resp.IsStatusFailure() {
		return NormaFull{}, fmt.Errorf("get norm: unexpected status %d", resp.StatusCode())
	}

	data, err := readBounded(resp.Body, maxNormResponseBytes)
	if err != nil {
		c.logger.Error("bcn get norm body failed", "norm_id", q.NormID, "version_date", q.VersionDate, "error", err)
		return NormaFull{}, fmt.Errorf("get norm: %w", err)
	}

	var result NormaFull
	if err := json.Unmarshal(data, &result); err != nil {
		return NormaFull{}, fmt.Errorf("get norm: decode: %w", err)
	}

	// Conversion is the CPU hot spot: acquire a converter from the pool so
	// concurrent norms convert in parallel (one converter = one conversion
	// at a time, the library mutex serializes per instance).
	conv := c.converters.Get().(*converter.Converter)
	result.ConvertContent(conv)
	c.converters.Put(conv)

	// The API delivers metadatos.resumenes with leading indentation;
	// sanitize it like every other text that reaches the LLM.
	for i := range result.Metadatos.Resumenes {
		result.Metadatos.Resumenes[i] = SanitizeSummary(result.Metadatos.Resumenes[i])
	}
	// Append (never replace) the canonical norm types from the catalog.
	enrichCanonicalTypes(&result)

	c.normas.put(key, etagEntry[NormaFull]{etag: resp.Header().Get("ETag"), value: result})
	return result, nil
}

// readBounded reads r up to limit+1 bytes and fails if the body exceeds
// the limit — a hard cap, never truncated content.
func readBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > int(limit) {
		return nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return data, nil
}

// GetNormaSummary returns the lightweight metadata view of a norm. On a
// cache hit it derives the summary from the cached entry without any HTTP
// call; on a miss it goes through GetNorma (full flow with revalidation)
// and projects. The cache entry is shared, so a later get_law benefits.
func (c *Client) GetNormaSummary(ctx context.Context, q NormaQuery) (NormaSummary, error) {
	if entry, ok := c.normas.get(normaCacheKey(q)); ok {
		c.logger.Debug("bcn norm summary cache hit", "norm_id", q.NormID, "version_date", q.VersionDate)
		return projectSummary(entry.value), nil
	}
	norma, err := c.GetNorma(ctx, q)
	if err != nil {
		return NormaSummary{}, err
	}
	return projectSummary(norma), nil
}

// GetLawHistory calls get_historias_de_ley for one norm. The response is
// cached per norm with ETag revalidation (the endpoint sends ETag and
// answers 304). A nonexistent norm yields an empty list — no error.
func (c *Client) GetLawHistory(ctx context.Context, normID int64) ([]HistoriaGrupo, error) {
	res := c.resources.Resources[resourceGetLawHist]
	key := strconv.FormatInt(normID, 10)
	entry, cached := c.historias.get(key)

	req := c.clients[resourceGetLawHist].R().
		SetContext(ctx).
		SetQueryParam("idNorma", key).
		SetRetryCount(res.Retry.Attempts).
		SetRetryWaitTime(time.Duration(res.Retry.Backoff)).
		SetRetryMaxWaitTime(time.Duration(res.Retry.MaxBackoff)).
		SetRetryConditions(retryConditions...)
	if cached {
		req.SetHeader("If-None-Match", entry.etag)
	}

	var result []HistoriaGrupo
	resp, err := req.SetResult(&result).Get(res.Path)

	if resp != nil && resp.StatusCode() == http.StatusNotModified && cached {
		c.logger.Debug("bcn law history cache hit", "norm_id", normID)
		return entry.value, nil
	}
	if err != nil {
		c.logger.Error("bcn law history failed", "norm_id", normID, "error", err)
		return nil, fmt.Errorf("get law history: %w", err)
	}
	if resp.IsStatusFailure() {
		return nil, fmt.Errorf("get law history: unexpected status %d", resp.StatusCode())
	}

	c.historias.put(key, etagEntry[[]HistoriaGrupo]{etag: resp.Header().Get("ETag"), value: result})
	return result, nil
}

// enrichCanonicalTypes appends the canonical name and abbreviation to each
// norm type, resolved from the official catalog by its numeric code. Raw
// API values are left intact; unknown codes leave the fields empty
// (omitted from JSON by omitempty).
func enrichCanonicalTypes(n *NormaFull) {
	for i := range n.Metadatos.TiposNumeros {
		cod, err := strconv.Atoi(n.Metadatos.TiposNumeros[i].Tipo)
		if err != nil {
			continue
		}
		valor, abbr, ok := canonicalNormType(cod)
		if !ok {
			continue
		}
		n.Metadatos.TiposNumeros[i].CanonicalType = valor
		n.Metadatos.TiposNumeros[i].CanonicalAbbr = abbr
	}
}

// projectSummary derives the lightweight view from a full norm. The
// resumenes are already sanitized by GetNorma. The structure is flattened
// (schema-safe) and the counts describe the FULL norm — the summary is the
// "map" of the law, so the size is always the whole document.
func projectSummary(n NormaFull) NormaSummary {
	return NormaSummary{
		TituloNorma:     n.Metadatos.TituloNorma,
		Fuente:          n.Metadatos.Fuente,
		Materias:        n.Metadatos.Materias,
		CategoriasNorma: n.Metadatos.CategoriasNorma,
		Resumenes:       n.Metadatos.Resumenes,
		Estructura:      FlattenStructure(n.Estructura),
		CharCount:       ContentCharCount(n.Html),
		ArticleCount:    n.CountArticles(),
	}
}
