// Package cgr provides the Contraloría General de la República client:
// search and retrieval of dictámenes via contraloria.cl/apibusca.
// All HTTP behavior (timeout, retry, circuit breaker) is configured per
// resource from the api.resources.yaml contract — no hardcoded URLs or
// policies in code. Mirrors internal/bcn patterns with clean-directo
// sanitization and LRU without ETag (CGR does not send ETag).
package cgr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
	"resty.dev/v3"

	"github.com/alvarosdev/lex-chile-mcp/internal/config"
)

// Resource ids in api.resources.yaml.
const (
	resourceCgrSearch = "cgr_search"
	resourceCgrCount  = "cgr_count"
)

var allowedSources = []string{"dictamenes", "instructivos", "contable", "auditoria", "legislacion", "cuentas", "consolidados", "web", "todos"}

// Errors per type.
var (
	ErrDictamenNotFound    = errors.New("dictamen not found")
	ErrInstructivoNotFound = errors.New("instructivo not found")
	ErrContableNotFound    = errors.New("contable not found")
	ErrAuditoriaNotFound   = errors.New("auditoria not found")
	ErrConsolidadoNotFound = errors.New("consolidado not found")
	ErrCuentaNotFound      = errors.New("cuenta not found")
	ErrLegislacionNotFound = errors.New("legislacion not found")
)

// CgrClient defines the operations the MCP tools need from the CGR API.
type CgrClient interface {
	Search(ctx context.Context, params SearchParams) (SearchResponse, error)
	SearchDictamenes(ctx context.Context, params SearchParams) (SearchResponse, error)
	GetDictamen(ctx context.Context, dictamenID string) (DictamenFull, error)
	GetInstructivo(ctx context.Context, instructivoID string) (InstructivoFull, error)
	GetContable(ctx context.Context, contableID string) (ContableFull, error)
	GetAuditoria(ctx context.Context, auditoriaID string) (AuditoriaFull, error)
	GetConsolidado(ctx context.Context, consolidadoID string) (ConsolidadoFull, error)
	GetCuenta(ctx context.Context, cuentaID string) (CuentaFull, error)
	GetLegislacion(ctx context.Context, legislacionID string) (LegislacionFull, error)
	CountJurisprudencia(ctx context.Context, query string, exactSearch bool) (CountResponse, error)
}

// Client implements CgrClient over resty, with one *resty.Client per
// resource — resty's circuit breaker is client-level. Clients are built
// once in NewClient and reused for every request.
type Client struct {
	logger        *slog.Logger
	resources     *config.Resources
	clients       map[string]*resty.Client
	flights       singleflight.Group
	searches      *lruCache[SearchResponse]
	dictamenes    *lruCache[DictamenFull]
	instructivos  *lruCache[InstructivoFull]
	contables     *lruCache[ContableFull]
	auditorias    *lruCache[AuditoriaFull]
	consolidados  *lruCache[ConsolidadoFull]
	cuentas       *lruCache[CuentaFull]
	legislaciones *lruCache[LegislacionFull]
	counts        *lruCache[CountResponse]
}

// NewClient builds a Client from the resources contract.
func NewClient(resources *config.Resources, logger *slog.Logger) *Client {
	c := &Client{
		logger:        logger,
		resources:     resources,
		clients:       make(map[string]*resty.Client, len(resources.Resources)),
		searches:      newLRUCache[SearchResponse](),
		dictamenes:    newLRUCache[DictamenFull](),
		instructivos:  newLRUCacheWithMax[InstructivoFull](cacheMaxLight),
		contables:     newLRUCacheWithMax[ContableFull](cacheMaxLight),
		auditorias:    newLRUCacheWithMax[AuditoriaFull](cacheMaxHeavy),
		consolidados:  newLRUCacheWithMax[ConsolidadoFull](cacheMaxLight),
		cuentas:       newLRUCacheWithMax[CuentaFull](cacheMaxLight),
		legislaciones: newLRUCacheWithMax[LegislacionFull](cacheMaxHeavy),
		counts:        newLRUCache[CountResponse](),
	}
	for name, res := range resources.Resources {
		if name == resourceCgrSearch || name == resourceCgrCount {
			c.clients[name] = c.newRestyClient(name, res)
		}
	}
	return c
}

func (c *Client) newRestyClient(name string, res config.Resource) *resty.Client {
	return resty.New().
		SetBaseURL(res.URL).
		SetTimeout(time.Duration(res.Timeout)).
		SetRetryAllowNonIdempotent(true).
		SetCircuitBreaker(resty.NewCircuitBreakerCount(
			uint64(res.CircuitBreaker.FailureThreshold),
			uint64(res.CircuitBreaker.SuccessThreshold),
			time.Duration(res.CircuitBreaker.ResetTimeout),
		)).
		OnError(func(req *resty.Request, err error) {
			c.logger.Debug("cgr request attempt failed",
				"resource", name,
				"error", err,
			)
		})
}

var retryConditions = []resty.RetryConditionFunc{
	resty.RetryConditionStatus5XX,
	resty.RetryConditionStatusZero,
	func(resp *resty.Response, err error) bool {
		return resp != nil && resp.StatusCode() == httpStatusTooManyRequests
	},
}

const httpStatusTooManyRequests = 429

var dictamenIDRe = regexp.MustCompile(`^[A-Z]*[0-9]+N[0-9]{2}$`)
var instructivoIDRe = regexp.MustCompile(`^[A-Z]{0,3}[0-9]{1,6}N[0-9]{2}$`)
var contableIDRe = regexp.MustCompile(`^(E[0-9]{1,6}|OFE[0-9]{10,13}|[A-Z]*[0-9]+N[0-9]{2})$`)
var auditoriaIDRe = regexp.MustCompile(`^([0-9]{1,4}/[0-9]{4}|[A-Z]*[0-9]+N[0-9]{2}|[0-9]{1,6})$`)
var consolidadoIDRe = regexp.MustCompile(`^CIC[0-9]{1,4}/[0-9]{4}$`)
var cuentaIDRe = regexp.MustCompile(`^[0-9]{1,7}$`)
var legislacionIDRe = regexp.MustCompile(`^(RZA|FRA)[0-9]{9,12}$`)

func dictamenURL(id string) string {
	return fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/dictamenes/%s/html", id)
}

func dictamenPDFURL(id string) string {
	return fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/dictamenes/%s/pdf", id)
}

func searchCacheKey(p SearchParams) string {
	return fmt.Sprintf("search:%s|%s|%t|%s|%d", p.Source, p.Query, p.ExactSearch, p.Order, p.Page)
}

func countCacheKey(query string, exact bool) string {
	return fmt.Sprintf("count:%s|%t", query, exact)
}

// validOrder reports whether order is one of the allowed values.
func validOrder(order string) bool {
	return order == "date" || order == "dateasc" || order == "score"
}

func validSource(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) == 0 || len(s) > 20 {
		return false
	}
	if strings.Contains(s, "/") || strings.Contains(s, ".") || strings.Contains(s, "%") || strings.Contains(s, "?") || strings.Contains(s, "#") {
		return false
	}
	return slices.Contains(allowedSources, s)
}

func validQuery(q string) (string, bool) {
	runes := []rune(q)
	if len(runes) > 500 {
		return string(runes[:500]), true
	}
	return q, false
}

func validPage(page int) error {
	if page < 1 {
		return fmt.Errorf("page must be >= 1")
	}
	if page > 500 {
		return fmt.Errorf("page must be <= 500")
	}
	return nil
}

func truncateLogQuery(q string) string {
	if len(q) > 80 {
		return q[:80] + "..."
	}
	return q
}

func timeoutForSource(source string) (time.Duration, int, time.Duration, time.Duration) {
	s := strings.ToLower(strings.TrimSpace(source))
	switch s {
	case "contable", "instructivos", "consolidados", "cuentas":
		return 10 * time.Second, 3, 500 * time.Millisecond, 5 * time.Second
	case "dictamenes", "web", "todos":
		return 12 * time.Second, 3, 500 * time.Millisecond, 5 * time.Second
	case "auditoria", "legislacion":
		return 15 * time.Second, 2, 1 * time.Second, 4 * time.Second
	default:
		return 12 * time.Second, 3, 500 * time.Millisecond, 5 * time.Second
	}
}

func (c *Client) searchPath(source string) string {
	res, ok := c.resources.Resources[resourceCgrSearch]
	if !ok {
		return "/apibusca/search/" + source
	}
	base := res.Path
	suffix := "/" + source
	if strings.HasSuffix(base, suffix) {
		return base
	}
	if strings.HasPrefix(base, "/apibusca/search/") {
		idx := strings.LastIndex(base, "/")
		return base[:idx] + suffix
	}
	// base is "/apibusca/search" -> append
	return base + suffix
}

// Search is the generic multi-source search. It validates Source, Order, Page
// and query length before any I/O, then checks cache, then singleflight.
func (c *Client) Search(ctx context.Context, params SearchParams) (SearchResponse, error) {
	if params.Source == "" {
		params.Source = "dictamenes"
	}
	params.Source = strings.ToLower(strings.TrimSpace(params.Source))
	if !validSource(params.Source) {
		return SearchResponse{}, fmt.Errorf("invalid source %q", params.Source)
	}
	if params.Order == "" {
		params.Order = "date"
	}
	if !validOrder(params.Order) {
		return SearchResponse{}, fmt.Errorf("invalid order %q: must be date, dateasc or score", params.Order)
	}
	if params.Page == 0 {
		params.Page = 1
	}
	if err := validPage(params.Page); err != nil {
		return SearchResponse{}, err
	}
	if q, truncated := validQuery(params.Query); truncated {
		c.logger.Warn("cgr search query truncated", "original_len", len([]rune(params.Query)), "truncated_len", 500, "query", truncateLogQuery(q))
		params.Query = q
	}
	key := searchCacheKey(params)
	if v, ok := c.searches.get(key); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do(key, func() (any, error) {
		return c.searchOnce(ctx, params, key)
	})
	if err != nil {
		return SearchResponse{}, err
	}
	return v.(SearchResponse), nil
}

// SearchDictamenes is an alias that calls Search with Source=dictamenes if caller didn't set Source.
func (c *Client) SearchDictamenes(ctx context.Context, params SearchParams) (SearchResponse, error) {
	if params.Source == "" {
		params.Source = "dictamenes"
	}
	return c.Search(ctx, params)
}

type cgrSearchRequest struct {
	Search      string `json:"search"`
	ExactSearch bool   `json:"exact_search"`
	Options     []any  `json:"options"`
	Order       string `json:"order"`
	DateName    string `json:"date_name"`
	Source      string `json:"source"`
	Page        int    `json:"page"`
}

func (c *Client) searchOnce(ctx context.Context, params SearchParams, key string) (SearchResponse, error) {
	res, ok := c.resources.Resources[resourceCgrSearch]
	if !ok {
		return SearchResponse{}, fmt.Errorf("resource %q not configured", resourceCgrSearch)
	}
	timeout, attempts, backoff, maxBackoff := timeoutForSource(params.Source)
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	apiPage := params.Page - 1
	body := cgrSearchRequest{
		Search:      params.Query,
		ExactSearch: params.ExactSearch,
		Options:     []any{},
		Order:       params.Order,
		DateName:    "fecha_documento",
		Source:      params.Source,
		Page:        apiPage,
	}
	path := c.searchPath(params.Source)

	// Use resty but capture raw bytes to check ContentLength before unmarshal.
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)

	resp, err := req.Post(path)
	if err != nil {
		c.logger.Error("cgr search failed", "error", err, "source", params.Source, "query", truncateLogQuery(params.Query))
		return SearchResponse{}, fmt.Errorf("search cgr: %w", err)
	}
	if resp.IsStatusFailure() {
		if resp.StatusCode() == httpStatusTooManyRequests {
			retryAfter := resp.Header().Get("Retry-After")
			c.logger.Warn("cgr search 429", "source", params.Source, "retry_after", retryAfter)
		}
		return SearchResponse{}, fmt.Errorf("search cgr: unexpected status %d", resp.StatusCode())
	}
	// ContentLength check before Unmarshal: >6MB
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 6*1024*1024 {
			return SearchResponse{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 6*1024*1024 {
		return SearchResponse{}, fmt.Errorf("response too large")
	}

	// Decode based on source to handle different _source shapes. For simplicity,
	// we decode into generic envelope then map to DictamenSummary. For dictamenes/web/todos
	// the shape is cgrSource; for other sources we decode per-type and map.
	var total int
	var relation string
	// First decode envelope to get hits raw
	var envelope struct {
		Hits struct {
			Total struct {
				Value    int    `json:"value"`
				Relation string `json:"relation"`
			} `json:"total"`
			Hits []struct {
				ID     string          `json:"_id"`
				Index  string          `json:"_index"`
				Score  float64         `json:"_score"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(resp.Bytes(), &envelope); err != nil {
		return SearchResponse{}, fmt.Errorf("search cgr: decode failed: %w", err)
	}
	total = envelope.Hits.Total.Value
	relation = envelope.Hits.Total.Relation
	_ = relation
	results := make([]DictamenSummary, 0, len(envelope.Hits.Hits))
	for _, h := range envelope.Hits.Hits {
		// hit._index check
		if params.Source != "todos" {
			expected := "cgr-" + params.Source
			if h.Index != "" && h.Index != expected {
				c.logger.Debug("cgr hit index mismatch", "expected", expected, "got", h.Index, "id", h.ID)
				continue
			}
		} else {
			if h.Index != "" && !strings.HasPrefix(h.Index, "cgr-") {
				continue
			}
		}
		ds := DictamenSummary{}
		switch params.Source {
		case "dictamenes", "web", "todos":
			var s cgrSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			// filter old_url / 172.30.x is implicit (not exposed)
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    s.NDictamen,
				NumericID:    s.NumericDocID,
				FechaDoc:     s.FechaDocumento,
				Materia:      SanitizeMateria(s.Materia),
				Descriptores: s.Descriptores,
				Criterio:     s.Criterio,
				Origen:       s.Origen,
				Caracter:     s.Carater,
				URL:          dictamenURL(h.ID),
				PDFURL:       dictamenPDFURL(h.ID),
			}
			if ds.DictamenID == "" {
				ds.DictamenID = s.DocID
				ds.URL = dictamenURL(ds.DictamenID)
				ds.PDFURL = dictamenPDFURL(ds.DictamenID)
			}
		case "instructivos":
			var s cgrInstructivoSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    s.NDictamen,
				NumericID:    s.NumericDocID,
				FechaDoc:     s.FechaDocumento,
				Materia:      SanitizeMateria(s.Materia),
				Descriptores: s.Descriptores,
				Criterio:     s.Criterio,
				Origen:       s.Origen,
				Caracter:     s.Carater,
				URL:          dictamenURL(h.ID),
				PDFURL:       dictamenPDFURL(h.ID),
			}
			if ds.DictamenID == "" {
				ds.DictamenID = s.DocID
				ds.URL = dictamenURL(ds.DictamenID)
				ds.PDFURL = dictamenPDFURL(ds.DictamenID)
			}
		case "contable":
			var s cgrContableSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			numero := s.Numero
			if numero == "" {
				numero = s.NumeroAlt
			}
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    numero,
				NumericID:    s.DocID,
				FechaDoc:     s.FechaDocumento,
				Materia:      SanitizeMateria(s.Parte),
				Descriptores: s.Tipo,
				Criterio:     s.NormativaContable,
				Origen:       s.Origen,
				Caracter:     s.Tipo,
				URL:          fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/contable/%s/html", h.ID),
				PDFURL:       fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/contable/%s/pdf", h.ID),
			}
			if ds.DictamenID == "" {
				ds.DictamenID = s.DocID
			}
			if s.Origen == "" && s.OrigenAlt != "" {
				ds.Origen = s.OrigenAlt
			}
		case "auditoria":
			var s cgrAuditoriaSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			numero := s.Numero
			if numero == "" {
				numero = s.NumeroAlt
			}
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    numero,
				NumericID:    s.NumericDocID,
				FechaDoc:     s.FechaDocumento,
				Materia:      SanitizeMateria(s.Nombre),
				Descriptores: s.Tipo,
				Criterio:     s.Objetivo,
				Origen:       s.UnidadCgr,
				Caracter:     s.Tipo,
				URL:          s.PDF,
				PDFURL:       s.PDF,
			}
			if s.FechaDocAlt != "" && ds.FechaDoc == "" {
				ds.FechaDoc = s.FechaDocAlt
			}
			if ds.DictamenID == "" {
				ds.DictamenID = s.DocID
			}
		case "consolidados":
			var s cgrConsolidadoSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    s.Numero,
				NumericID:    s.Numero,
				FechaDoc:     s.FechaDocumento,
				Materia:      SanitizeMateria(s.Nombre),
				Descriptores: s.Resena,
				Criterio:     s.Tipo,
				Origen:       s.UnidadCgr,
				Caracter:     s.Tipo,
				URL:          s.PDFWeb,
				PDFURL:       s.PDFWeb,
			}
			if s.TipoAlt != "" && ds.Criterio == "" {
				ds.Criterio = s.TipoAlt
			}
		case "cuentas":
			var s cgrCuentaSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    s.NumeroSentencia,
				NumericID:    s.NumericDocID,
				FechaDoc:     s.FechaSentencia,
				Materia:      SanitizeMateria(s.Texto),
				Descriptores: s.NumeroExpediente,
				Origen:       "",
				Caracter:     "cuentas",
				URL:          s.PDF,
				PDFURL:       s.PDF,
			}
			if ds.FechaDoc == "" {
				ds.FechaDoc = s.FechaDocumento
			}
			if ds.DictamenID == "" {
				ds.DictamenID = s.DocID
			}
		case "legislacion":
			var s cgrLegislacionSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			numero := s.Numero
			if numero == "" {
				numero = s.NumeroAlt
			}
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    numero,
				NumericID:    s.NumericDocID,
				FechaDoc:     s.FechaDocumento,
				Materia:      SanitizeMateria(s.Materias),
				Descriptores: s.Tipo,
				Criterio:     s.Caracter,
				Origen:       s.Organismo,
				Caracter:     s.Caracter,
				URL:          fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/legislacion/%s/html", h.ID),
				PDFURL:       fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/legislacion/%s/pdf", h.ID),
			}
			if ds.DictamenID == "" {
				ds.DictamenID = s.DocID
			}
		default:
			var s cgrSource
			if err := json.Unmarshal(h.Source, &s); err != nil {
				continue
			}
			ds = DictamenSummary{
				DictamenID:   h.ID,
				NDictamen:    s.NDictamen,
				NumericID:    s.NumericDocID,
				FechaDoc:     s.FechaDocumento,
				Materia:      SanitizeMateria(s.Materia),
				Descriptores: s.Descriptores,
				Criterio:     s.Criterio,
				Origen:       s.Origen,
				Caracter:     s.Carater,
				URL:          dictamenURL(h.ID),
				PDFURL:       dictamenPDFURL(h.ID),
			}
		}
		// Ensure URL not leaking internal 172.30.x
		if strings.Contains(ds.URL, "172.30.") || strings.Contains(ds.PDFURL, "172.30.") {
			ds.URL = ""
			ds.PDFURL = ""
		}
		results = append(results, ds)
	}

	const pageSize = 20
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	hasMore := params.Page*pageSize < total

	out := SearchResponse{
		Results: results,
		Pagination: Pagination{
			Total:      total,
			Page:       params.Page,
			PageSize:   pageSize,
			TotalPages: totalPages,
			HasMore:    hasMore,
		},
	}
	// Do not cache heavy responses >2MB
	if resp.Size() > 2*1024*1024 && (params.Source == "auditoria" || params.Source == "legislacion") {
		c.logger.Debug("cgr search heavy not cached", "source", params.Source, "size", resp.Size())
	} else {
		c.searches.put(key, out)
	}
	c.logger.Debug("cgr search ok",
		"query", truncateLogQuery(params.Query),
		"source", params.Source,
		"results", len(results),
		"total", total,
		"page", params.Page,
	)
	// Avoid unused import for resource variable when base path used via searchPath
	_ = res
	return out, nil
}

// GetDictamen returns the full dictamen for a given dictamen_id via
// POST /search with exact_search:true. The dictamen is cached by id.
func (c *Client) GetDictamen(ctx context.Context, dictamenID string) (DictamenFull, error) {
	if dictamenID == "" {
		return DictamenFull{}, fmt.Errorf("dictamen_id is required")
	}
	if len(dictamenID) > 40 {
		return DictamenFull{}, fmt.Errorf("invalid dictamen_id %q: too long", dictamenID)
	}
	if strings.ContainsAny(dictamenID, "/.%?#\n\r") {
		return DictamenFull{}, fmt.Errorf("invalid dictamen_id %q", dictamenID)
	}
	trimmed := strings.TrimSpace(strings.ToUpper(dictamenID))
	dictamenID = trimmed
	if !dictamenIDRe.MatchString(dictamenID) {
		return DictamenFull{}, fmt.Errorf("invalid dictamen_id %q", dictamenID)
	}
	if v, ok := c.dictamenes.get(dictamenID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do("get:dictamenes:"+dictamenID, func() (any, error) {
		return c.getDictamenOnce(ctx, dictamenID)
	})
	if err != nil {
		return DictamenFull{}, err
	}
	return v.(DictamenFull), nil
}

func (c *Client) getDictamenOnce(ctx context.Context, dictamenID string) (DictamenFull, error) {
	res, ok := c.resources.Resources[resourceCgrSearch]
	if !ok {
		return DictamenFull{}, fmt.Errorf("resource %q not configured", resourceCgrSearch)
	}
	timeout, attempts, backoff, maxBackoff := timeoutForSource("dictamenes")
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := cgrSearchRequest{
		Search:      dictamenID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "dictamenes",
		Page:        0,
	}
	path := c.searchPath("dictamenes")
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)
	resp, err := req.Post(path)
	if err != nil {
		c.logger.Error("cgr get dictamen failed", "error", err, "id", dictamenID)
		return DictamenFull{}, fmt.Errorf("get dictamen: %w", err)
	}
	if resp.IsStatusFailure() {
		return DictamenFull{}, fmt.Errorf("get dictamen: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 6*1024*1024 {
			return DictamenFull{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 6*1024*1024 {
		return DictamenFull{}, fmt.Errorf("response too large")
	}
	var wire cgrSearchEnvelope[cgrSource]
	if err := json.Unmarshal(resp.Bytes(), &wire); err != nil {
		return DictamenFull{}, fmt.Errorf("get dictamen: decode failed: %w", err)
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return DictamenFull{}, ErrDictamenNotFound
	}
	h := wire.Hits.Hits[0]
	if h.Index != "" && h.Index != "cgr-dictamenes" {
		return DictamenFull{}, ErrDictamenNotFound
	}
	s := h.Source
	doc := SanitizeDocumento(s.Documento)
	full := DictamenFull{
		DictamenSummary: DictamenSummary{
			DictamenID:   h.ID,
			NDictamen:    s.NDictamen,
			NumericID:    s.NumericDocID,
			FechaDoc:     s.FechaDocumento,
			Materia:      SanitizeMateria(s.Materia),
			Descriptores: s.Descriptores,
			Criterio:     s.Criterio,
			Origen:       s.Origen,
			Caracter:     s.Carater,
			URL:          dictamenURL(h.ID),
			PDFURL:       dictamenPDFURL(h.ID),
		},
		Destinatarios:  s.Destinatarios,
		Abogados:       s.Abogados,
		FuentesLegales: s.FuentesLegales,
		Documento:      doc,
		CharCount:      len([]rune(doc)),
	}
	if full.DictamenID == "" {
		full.DictamenID = s.DocID
		full.URL = dictamenURL(full.DictamenID)
		full.PDFURL = dictamenPDFURL(full.DictamenID)
	}
	_ = res
	c.dictamenes.put(dictamenID, full)
	c.logger.Debug("cgr get dictamen ok", "id", dictamenID, "char_count", full.CharCount)
	return full, nil
}

// GetInstructivo returns the full instructivo.
func (c *Client) GetInstructivo(ctx context.Context, instructivoID string) (InstructivoFull, error) {
	if instructivoID == "" {
		return InstructivoFull{}, fmt.Errorf("instructivo_id is required")
	}
	trimmed := strings.TrimSpace(strings.ToUpper(instructivoID))
	if len(trimmed) > 40 {
		return InstructivoFull{}, fmt.Errorf("invalid instructivo_id %q: too long", trimmed)
	}
	if strings.ContainsAny(trimmed, "/.%?#\n\r") {
		return InstructivoFull{}, fmt.Errorf("invalid instructivo_id %q", trimmed)
	}
	if !instructivoIDRe.MatchString(trimmed) {
		return InstructivoFull{}, fmt.Errorf("invalid instructivo_id %q", trimmed)
	}
	instructivoID = trimmed
	if v, ok := c.instructivos.get(instructivoID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do("get:instructivos:"+instructivoID, func() (any, error) {
		return c.getInstructivoOnce(ctx, instructivoID)
	})
	if err != nil {
		return InstructivoFull{}, err
	}
	return v.(InstructivoFull), nil
}

func (c *Client) getInstructivoOnce(ctx context.Context, instructivoID string) (InstructivoFull, error) {
	timeout, attempts, backoff, maxBackoff := timeoutForSource("instructivos")
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := cgrSearchRequest{
		Search:      instructivoID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "instructivos",
		Page:        0,
	}
	path := c.searchPath("instructivos")
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)
	resp, err := req.Post(path)
	if err != nil {
		return InstructivoFull{}, fmt.Errorf("get instructivo: %w", err)
	}
	if resp.IsStatusFailure() {
		return InstructivoFull{}, fmt.Errorf("get instructivo: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 6*1024*1024 {
			return InstructivoFull{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 6*1024*1024 {
		return InstructivoFull{}, fmt.Errorf("response too large")
	}
	var wire cgrSearchEnvelope[cgrInstructivoSource]
	if err := json.Unmarshal(resp.Bytes(), &wire); err != nil {
		return InstructivoFull{}, fmt.Errorf("get instructivo: decode failed: %w", err)
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return InstructivoFull{}, ErrInstructivoNotFound
	}
	h := wire.Hits.Hits[0]
	if h.Index != "" && h.Index != "cgr-instructivos" {
		return InstructivoFull{}, ErrInstructivoNotFound
	}
	s := h.Source
	doc := SanitizeInstructivoDocumento(s.Documento)
	full := InstructivoFull{
		InstructivoSummary: InstructivoSummary{
			InstructivoID: h.ID,
			NDictamen:     s.NDictamen,
			NumericID:     s.NumericDocID,
			FechaDoc:      s.FechaDocumento,
			Materia:       SanitizeMateria(s.Materia),
			Descriptores:  SanitizeTexto(s.Descriptores),
			Caracter:      s.Carater,
			URL:           dictamenURL(h.ID),
			PDFURL:        dictamenPDFURL(h.ID),
		},
		Destinatarios:  SanitizeDestinatarios(s.Destinatarios),
		Abogados:       SanitizeTexto(s.Abogados),
		FuentesLegales: SanitizeTexto(s.FuentesLegales),
		Documento:      doc,
		CharCount:      len([]rune(doc)),
	}
	if full.InstructivoID == "" {
		full.InstructivoID = s.DocID
	}
	c.instructivos.put(instructivoID, full)
	return full, nil
}

// GetContable returns the full contable oficio.
func (c *Client) GetContable(ctx context.Context, contableID string) (ContableFull, error) {
	if contableID == "" {
		return ContableFull{}, fmt.Errorf("contable_id is required")
	}
	trimmed := strings.TrimSpace(strings.ToUpper(contableID))
	if len(trimmed) > 40 {
		return ContableFull{}, fmt.Errorf("invalid contable_id %q: too long", trimmed)
	}
	if strings.ContainsAny(trimmed, ".%?#\n\r") {
		return ContableFull{}, fmt.Errorf("invalid contable_id %q", trimmed)
	}
	// Allow slash only if it matches auditoria pattern? For contable no slash expected.
	if strings.Contains(trimmed, "/") {
		return ContableFull{}, fmt.Errorf("invalid contable_id %q", trimmed)
	}
	if !contableIDRe.MatchString(trimmed) {
		return ContableFull{}, fmt.Errorf("invalid contable_id %q", trimmed)
	}
	contableID = trimmed
	if v, ok := c.contables.get(contableID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do("get:contable:"+contableID, func() (any, error) {
		return c.getContableOnce(ctx, contableID)
	})
	if err != nil {
		return ContableFull{}, err
	}
	return v.(ContableFull), nil
}

func (c *Client) getContableOnce(ctx context.Context, contableID string) (ContableFull, error) {
	timeout, attempts, backoff, maxBackoff := timeoutForSource("contable")
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := cgrSearchRequest{
		Search:      contableID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "contable",
		Page:        0,
	}
	path := c.searchPath("contable")
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)
	resp, err := req.Post(path)
	if err != nil {
		return ContableFull{}, fmt.Errorf("get contable: %w", err)
	}
	if resp.IsStatusFailure() {
		return ContableFull{}, fmt.Errorf("get contable: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 2*1024*1024 {
			return ContableFull{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 2*1024*1024 {
		return ContableFull{}, fmt.Errorf("response too large")
	}
	var wire cgrSearchEnvelope[cgrContableSource]
	if err := json.Unmarshal(resp.Bytes(), &wire); err != nil {
		return ContableFull{}, fmt.Errorf("get contable: decode failed: %w", err)
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return ContableFull{}, ErrContableNotFound
	}
	h := wire.Hits.Hits[0]
	if h.Index != "" && h.Index != "cgr-contable" {
		return ContableFull{}, ErrContableNotFound
	}
	s := h.Source
	sanitizedTexto := SanitizeContableTexto(s.Texto)
	full := ContableFull{
		ContableSummary: ContableSummary{
			DocID:             h.ID,
			Numero:            s.Numero,
			NormativaContable: s.NormativaContable,
			Tipo:              SanitizeTexto(s.Tipo),
			Parte:             SanitizeContableParte(s.Parte),
			Destinatarios:     SanitizeDestinatarios(s.Destinatarios),
			FechaDoc:          s.FechaDocumento,
			Origen:            SanitizeTexto(s.Origen),
		},
		Texto:     sanitizedTexto,
		CharCount: len([]rune(sanitizedTexto)),
	}
	if full.Numero == "" {
		full.Numero = s.NumeroAlt
	}
	if full.DocID == "" {
		full.DocID = s.DocID
	}
	if full.Origen == "" && s.OrigenAlt != "" {
		full.Origen = s.OrigenAlt
	}
	c.contables.put(contableID, full)
	return full, nil
}

// GetAuditoria returns the full auditoria informe.
func (c *Client) GetAuditoria(ctx context.Context, auditoriaID string) (AuditoriaFull, error) {
	if auditoriaID == "" {
		return AuditoriaFull{}, fmt.Errorf("auditoria_id is required")
	}
	trimmed := strings.TrimSpace(auditoriaID)
	if len(trimmed) > 40 {
		return AuditoriaFull{}, fmt.Errorf("invalid auditoria_id %q: too long", trimmed)
	}
	if strings.ContainsAny(trimmed, ".%?#\n\r") {
		return AuditoriaFull{}, fmt.Errorf("invalid auditoria_id %q", trimmed)
	}
	// Normalize to upper for regex but keep slash
	upper := strings.ToUpper(trimmed)
	if !auditoriaIDRe.MatchString(upper) && !auditoriaIDRe.MatchString(trimmed) {
		return AuditoriaFull{}, fmt.Errorf("invalid auditoria_id %q", trimmed)
	}
	auditoriaID = trimmed
	if v, ok := c.auditorias.get(auditoriaID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do("get:auditoria:"+auditoriaID, func() (any, error) {
		return c.getAuditoriaOnce(ctx, auditoriaID)
	})
	if err != nil {
		return AuditoriaFull{}, err
	}
	return v.(AuditoriaFull), nil
}

func (c *Client) getAuditoriaOnce(ctx context.Context, auditoriaID string) (AuditoriaFull, error) {
	timeout, attempts, backoff, maxBackoff := timeoutForSource("auditoria")
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := cgrSearchRequest{
		Search:      auditoriaID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "auditoria",
		Page:        0,
	}
	path := c.searchPath("auditoria")
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)
	resp, err := req.Post(path)
	if err != nil {
		return AuditoriaFull{}, fmt.Errorf("get auditoria: %w", err)
	}
	if resp.IsStatusFailure() {
		return AuditoriaFull{}, fmt.Errorf("get auditoria: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 6*1024*1024 {
			return AuditoriaFull{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 6*1024*1024 {
		return AuditoriaFull{}, fmt.Errorf("response too large")
	}
	var wire cgrSearchEnvelope[cgrAuditoriaSource]
	if err := json.Unmarshal(resp.Bytes(), &wire); err != nil {
		return AuditoriaFull{}, fmt.Errorf("get auditoria: decode failed: %w", err)
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return AuditoriaFull{}, ErrAuditoriaNotFound
	}
	h := wire.Hits.Hits[0]
	if h.Index != "" && h.Index != "cgr-auditoria" {
		return AuditoriaFull{}, ErrAuditoriaNotFound
	}
	s := h.Source
	contenido := SanitizeAuditoriaContenido(s.ContenidoPDF)
	if len([]rune(contenido)) > 30000 {
		contenido = string([]rune(contenido)[:30000]) + "\n\n[contenido truncado, ver PDF oficial]"
	}
	full := AuditoriaFull{
		AuditoriaSummary: AuditoriaSummary{
			DocID:     h.ID,
			Numero:    s.Numero,
			Nombre:    SanitizeMateria(s.Nombre),
			Tipo:      SanitizeTexto(s.Tipo),
			Objetivo:  SanitizeAuditoriaObjetivo(s.Objetivo),
			UnidadCgr: SanitizeTexto(s.UnidadCgr),
			FechaDoc:  s.FechaDocumento,
			PDF:       s.PDF,
		},
		Conclusiones:  SanitizeAuditoriaConclusiones(s.Conclusiones),
		Destinatarios: SanitizeDestinatarios(s.Destinatarios),
		ContenidoPDF:  contenido,
		CharCount:     len([]rune(SanitizeAuditoriaContenido(s.ContenidoPDF))),
	}
	if full.Numero == "" {
		full.Numero = s.NumeroAlt
	}
	if full.DocID == "" {
		full.DocID = s.DocID
	}
	if full.FechaDoc == "" {
		full.FechaDoc = s.FechaDocAlt
	}
	c.auditorias.put(auditoriaID, full)
	return full, nil
}

// GetConsolidado returns the full consolidado.
func (c *Client) GetConsolidado(ctx context.Context, consolidadoID string) (ConsolidadoFull, error) {
	if consolidadoID == "" {
		return ConsolidadoFull{}, fmt.Errorf("consolidado_id is required")
	}
	trimmed := strings.TrimSpace(strings.ToUpper(consolidadoID))
	if len(trimmed) > 20 {
		return ConsolidadoFull{}, fmt.Errorf("invalid consolidado_id %q: too long", trimmed)
	}
	if strings.ContainsAny(trimmed, ".%?#\n\r") {
		return ConsolidadoFull{}, fmt.Errorf("invalid consolidado_id %q", trimmed)
	}
	if !consolidadoIDRe.MatchString(trimmed) {
		return ConsolidadoFull{}, fmt.Errorf("invalid consolidado_id %q", trimmed)
	}
	consolidadoID = trimmed
	if v, ok := c.consolidados.get(consolidadoID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do("get:consolidados:"+consolidadoID, func() (any, error) {
		return c.getConsolidadoOnce(ctx, consolidadoID)
	})
	if err != nil {
		return ConsolidadoFull{}, err
	}
	return v.(ConsolidadoFull), nil
}

func (c *Client) getConsolidadoOnce(ctx context.Context, consolidadoID string) (ConsolidadoFull, error) {
	timeout, attempts, backoff, maxBackoff := timeoutForSource("consolidados")
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := cgrSearchRequest{
		Search:      consolidadoID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "consolidados",
		Page:        0,
	}
	path := c.searchPath("consolidados")
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)
	resp, err := req.Post(path)
	if err != nil {
		return ConsolidadoFull{}, fmt.Errorf("get consolidado: %w", err)
	}
	if resp.IsStatusFailure() {
		return ConsolidadoFull{}, fmt.Errorf("get consolidado: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 2*1024*1024 {
			return ConsolidadoFull{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 2*1024*1024 {
		return ConsolidadoFull{}, fmt.Errorf("response too large")
	}
	var wire cgrSearchEnvelope[cgrConsolidadoSource]
	if err := json.Unmarshal(resp.Bytes(), &wire); err != nil {
		return ConsolidadoFull{}, fmt.Errorf("get consolidado: decode failed: %w", err)
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return ConsolidadoFull{}, ErrConsolidadoNotFound
	}
	h := wire.Hits.Hits[0]
	if h.Index != "" && h.Index != "cgr-consolidados" {
		return ConsolidadoFull{}, ErrConsolidadoNotFound
	}
	s := h.Source
	full := ConsolidadoFull{
		ConsolidadoSummary: ConsolidadoSummary{
			Numero:    s.Numero,
			Nombre:    SanitizeMateria(s.Nombre),
			Resena:    SanitizeConsolidadoResena(s.Resena),
			Tipo:      SanitizeTexto(s.Tipo),
			FechaDoc:  s.FechaDocumento,
			UnidadCgr: SanitizeTexto(s.UnidadCgr),
			PDFWeb:    s.PDFWeb,
		},
		ContenidoExtraido: SanitizeConsolidadoContenido(s.ContenidoExtraido),
		CharCount:         len([]rune(SanitizeConsolidadoContenido(s.ContenidoExtraido))),
	}
	if full.Tipo == "" {
		full.Tipo = SanitizeTexto(s.TipoAlt)
	}
	c.consolidados.put(consolidadoID, full)
	return full, nil
}

// GetCuenta returns the full cuenta sentencia.
func (c *Client) GetCuenta(ctx context.Context, cuentaID string) (CuentaFull, error) {
	if cuentaID == "" {
		return CuentaFull{}, fmt.Errorf("cuenta_id is required")
	}
	trimmed := strings.TrimSpace(cuentaID)
	if len(trimmed) > 20 {
		return CuentaFull{}, fmt.Errorf("invalid cuenta_id %q: too long", trimmed)
	}
	if strings.ContainsAny(trimmed, "/.%?#\n\r") {
		return CuentaFull{}, fmt.Errorf("invalid cuenta_id %q", trimmed)
	}
	if !cuentaIDRe.MatchString(trimmed) {
		return CuentaFull{}, fmt.Errorf("invalid cuenta_id %q", trimmed)
	}
	cuentaID = trimmed
	if v, ok := c.cuentas.get(cuentaID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do("get:cuentas:"+cuentaID, func() (any, error) {
		return c.getCuentaOnce(ctx, cuentaID)
	})
	if err != nil {
		return CuentaFull{}, err
	}
	return v.(CuentaFull), nil
}

func (c *Client) getCuentaOnce(ctx context.Context, cuentaID string) (CuentaFull, error) {
	timeout, attempts, backoff, maxBackoff := timeoutForSource("cuentas")
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := cgrSearchRequest{
		Search:      cuentaID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "cuentas",
		Page:        0,
	}
	path := c.searchPath("cuentas")
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)
	resp, err := req.Post(path)
	if err != nil {
		return CuentaFull{}, fmt.Errorf("get cuenta: %w", err)
	}
	if resp.IsStatusFailure() {
		return CuentaFull{}, fmt.Errorf("get cuenta: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 2*1024*1024 {
			return CuentaFull{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 2*1024*1024 {
		return CuentaFull{}, fmt.Errorf("response too large")
	}
	var wire cgrSearchEnvelope[cgrCuentaSource]
	if err := json.Unmarshal(resp.Bytes(), &wire); err != nil {
		return CuentaFull{}, fmt.Errorf("get cuenta: decode failed: %w", err)
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return CuentaFull{}, ErrCuentaNotFound
	}
	h := wire.Hits.Hits[0]
	if h.Index != "" && h.Index != "cgr-cuentas" {
		return CuentaFull{}, ErrCuentaNotFound
	}
	s := h.Source
	texto := SanitizeCuentaTexto(s.Texto)
	full := CuentaFull{
		CuentaSummary: CuentaSummary{
			DocID:            h.ID,
			NumeroSentencia:  SanitizeTexto(s.NumeroSentencia),
			NumeroExpediente: SanitizeTexto(s.NumeroExpediente),
			Texto:            texto,
			FechaSentencia:   s.FechaSentencia,
			FechaDoc:         s.FechaDocumento,
			PDF:              s.PDF,
			PDF2:             s.PDF2,
		},
		CharCount: len([]rune(texto)),
	}
	if full.DocID == "" {
		full.DocID = s.DocID
	}
	c.cuentas.put(cuentaID, full)
	return full, nil
}

// GetLegislacion returns the full legislacion entry.
func (c *Client) GetLegislacion(ctx context.Context, legislacionID string) (LegislacionFull, error) {
	if legislacionID == "" {
		return LegislacionFull{}, fmt.Errorf("legislacion_id is required")
	}
	trimmed := strings.TrimSpace(strings.ToUpper(legislacionID))
	if len(trimmed) > 40 {
		return LegislacionFull{}, fmt.Errorf("invalid legislacion_id %q: too long", trimmed)
	}
	if strings.ContainsAny(trimmed, "/.%?#\n\r") {
		return LegislacionFull{}, fmt.Errorf("invalid legislacion_id %q", trimmed)
	}
	if !legislacionIDRe.MatchString(trimmed) {
		return LegislacionFull{}, fmt.Errorf("invalid legislacion_id %q", trimmed)
	}
	legislacionID = trimmed
	if v, ok := c.legislaciones.get(legislacionID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do("get:legislacion:"+legislacionID, func() (any, error) {
		return c.getLegislacionOnce(ctx, legislacionID)
	})
	if err != nil {
		return LegislacionFull{}, err
	}
	return v.(LegislacionFull), nil
}

func (c *Client) getLegislacionOnce(ctx context.Context, legislacionID string) (LegislacionFull, error) {
	timeout, attempts, backoff, maxBackoff := timeoutForSource("legislacion")
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := cgrSearchRequest{
		Search:      legislacionID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "legislacion",
		Page:        0,
	}
	path := c.searchPath("legislacion")
	req := c.clients[resourceCgrSearch].R().
		SetContext(ctxTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(attempts).
		SetRetryWaitTime(backoff).
		SetRetryMaxWaitTime(maxBackoff).
		SetRetryConditions(retryConditions...)
	resp, err := req.Post(path)
	if err != nil {
		return LegislacionFull{}, fmt.Errorf("get legislacion: %w", err)
	}
	if resp.IsStatusFailure() {
		return LegislacionFull{}, fmt.Errorf("get legislacion: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 6*1024*1024 {
			return LegislacionFull{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 6*1024*1024 {
		return LegislacionFull{}, fmt.Errorf("response too large")
	}
	var wire cgrSearchEnvelope[cgrLegislacionSource]
	if err := json.Unmarshal(resp.Bytes(), &wire); err != nil {
		return LegislacionFull{}, fmt.Errorf("get legislacion: decode failed: %w", err)
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return LegislacionFull{}, ErrLegislacionNotFound
	}
	h := wire.Hits.Hits[0]
	if h.Index != "" && h.Index != "cgr-legislacion" {
		return LegislacionFull{}, ErrLegislacionNotFound
	}
	s := h.Source
	texto := SanitizeLegislacionTexto(s.Texto)
	full := LegislacionFull{
		LegislacionSummary: LegislacionSummary{
			DocID:     h.ID,
			Tipo:      SanitizeTexto(s.Tipo),
			Numero:    s.Numero,
			Organismo: SanitizeTexto(s.Organismo),
			Materias:  SanitizeLegislacionMaterias(s.Materias),
			Caracter:  SanitizeTexto(s.Caracter),
			FechaDoc:  s.FechaDocumento,
		},
		Texto:     texto,
		CharCount: len([]rune(texto)),
	}
	if full.Numero == "" {
		full.Numero = s.NumeroAlt
	}
	if full.DocID == "" {
		full.DocID = s.DocID
	}
	c.legislaciones.put(legislacionID, full)
	return full, nil
}

// CountJurisprudencia calls POST /apibusca/count/todos and returns
// the cross-type aggregation buckets.
func (c *Client) CountJurisprudencia(ctx context.Context, query string, exactSearch bool) (CountResponse, error) {
	if q, truncated := validQuery(query); truncated {
		c.logger.Warn("cgr count query truncated", "original_len", len([]rune(query)), "query", truncateLogQuery(q))
		query = q
	}
	key := countCacheKey(query, exactSearch)
	if v, ok := c.counts.get(key); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do(key, func() (any, error) {
		return c.countOnce(ctx, query, exactSearch, key)
	})
	if err != nil {
		return CountResponse{}, err
	}
	return v.(CountResponse), nil
}

type cgrCountRequest struct {
	Search      string `json:"search"`
	ExactSearch bool   `json:"exact_search"`
}

func (c *Client) countOnce(ctx context.Context, query string, exactSearch bool, key string) (CountResponse, error) {
	res, ok := c.resources.Resources[resourceCgrCount]
	if !ok {
		return CountResponse{}, fmt.Errorf("resource %q not configured", resourceCgrCount)
	}
	body := cgrCountRequest{
		Search:      query,
		ExactSearch: exactSearch,
	}
	var wire cgrCountResponse
	resp, err := c.clients[resourceCgrCount].R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		SetRetryCount(res.Retry.Attempts).
		SetRetryWaitTime(time.Duration(res.Retry.Backoff)).
		SetRetryMaxWaitTime(time.Duration(res.Retry.MaxBackoff)).
		SetRetryConditions(retryConditions...).
		SetResult(&wire).
		Post(res.Path)
	if err != nil {
		c.logger.Error("cgr count failed", "error", err, "query", truncateLogQuery(query))
		return CountResponse{}, fmt.Errorf("count jurisprudencia: %w", err)
	}
	if resp.IsStatusFailure() {
		return CountResponse{}, fmt.Errorf("count jurisprudencia: unexpected status %d", resp.StatusCode())
	}
	if cl := resp.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > 1*1024*1024 {
			return CountResponse{}, fmt.Errorf("response too large")
		}
	}
	if resp.Size() > 1*1024*1024 {
		return CountResponse{}, fmt.Errorf("response too large")
	}
	buckets := make([]CountBucket, 0, len(wire.Aggregations.CountByType.Buckets))
	for _, b := range wire.Aggregations.CountByType.Buckets {
		buckets = append(buckets, CountBucket{Type: b.Key, Count: b.DocCount})
	}
	out := CountResponse{
		Query:   query,
		Total:   wire.Hits.Total.Value,
		Buckets: buckets,
	}
	c.counts.put(key, out)
	c.logger.Debug("cgr count ok", "query", truncateLogQuery(query), "total", out.Total)
	return out, nil
}
