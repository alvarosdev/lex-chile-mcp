// Package cgr provides the Contraloría General de la República client:
// search and retrieval of dictámenes via contraloria.cl/apibusca.
// All HTTP behavior (timeout, retry, circuit breaker) is configured per
// resource from the api.resources.yaml contract — no hardcoded URLs or
// policies in code. Mirrors internal/bcn patterns with clean-directo
// sanitization and LRU without ETag (CGR does not send ETag).
package cgr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
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

// ErrDictamenNotFound is returned when the requested dictamen id does not
// exist (search returns hits.total.value == 0).
var ErrDictamenNotFound = errors.New("dictamen not found")

// CgrClient defines the operations the MCP tools need from the CGR API.
type CgrClient interface {
	SearchDictamenes(ctx context.Context, params SearchParams) (SearchResponse, error)
	GetDictamen(ctx context.Context, dictamenID string) (DictamenFull, error)
	CountJurisprudencia(ctx context.Context, query string, exactSearch bool) (CountResponse, error)
}

// Client implements CgrClient over resty, with one *resty.Client per
// resource — resty's circuit breaker is client-level. Clients are built
// once in NewClient and reused for every request.
type Client struct {
	logger     *slog.Logger
	resources  *config.Resources
	clients    map[string]*resty.Client
	flights    singleflight.Group
	searches   *lruCache[SearchResponse]
	dictamenes *lruCache[DictamenFull]
	counts     *lruCache[CountResponse]
}

// NewClient builds a Client from the resources contract.
func NewClient(resources *config.Resources, logger *slog.Logger) *Client {
	c := &Client{
		logger:     logger,
		resources:  resources,
		clients:    make(map[string]*resty.Client, len(resources.Resources)),
		searches:   newLRUCache[SearchResponse](),
		dictamenes: newLRUCache[DictamenFull](),
		counts:     newLRUCache[CountResponse](),
	}
	for name, res := range resources.Resources {
		// Only create resty clients for CGR resources; BCN resources are
		// handled by the bcn client. Creating all is harmless but avoids
		// polluting the map with unused clients when both clients coexist.
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
}

var dictamenIDRe = regexp.MustCompile(`^[A-Z]*[0-9]+N[0-9]{2}$`)

func dictamenURL(id string) string {
	return fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/dictamenes/%s/html", id)
}

func dictamenPDFURL(id string) string {
	return fmt.Sprintf("https://www.contraloria.cl/buscadorpdf/dictamenes/%s/pdf", id)
}

func searchCacheKey(p SearchParams) string {
	return fmt.Sprintf("search:%s|%t|%s|%d", p.Query, p.ExactSearch, p.Order, p.Page)
}

func countCacheKey(query string, exact bool) string {
	return fmt.Sprintf("count:%s|%t", query, exact)
}

// validOrder reports whether order is one of the allowed values.
func validOrder(order string) bool {
	return order == "date" || order == "dateasc" || order == "score"
}

// SearchDictamenes calls POST /apibusca/search/dictamenes with pagination
// (20 fixed per page, 0-indexed on the wire, 1-indexed for caller).
func (c *Client) SearchDictamenes(ctx context.Context, params SearchParams) (SearchResponse, error) {
	if params.Order == "" {
		params.Order = "date"
	}
	if !validOrder(params.Order) {
		return SearchResponse{}, fmt.Errorf("invalid order %q: must be date, dateasc or score", params.Order)
	}
	if params.Page == 0 {
		params.Page = 1
	}
	if params.Page < 1 {
		return SearchResponse{}, fmt.Errorf("page must be >= 1")
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
	apiPage := params.Page - 1
	body := cgrSearchRequest{
		Search:      params.Query,
		ExactSearch: params.ExactSearch,
		Options:     []any{},
		Order:       params.Order,
		DateName:    "fecha_documento",
		Source:      "dictamenes",
		Page:        apiPage,
	}
	var wire cgrSearchResponse
	resp, err := c.clients[resourceCgrSearch].R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetBody(body).
		SetRetryCount(res.Retry.Attempts).
		SetRetryWaitTime(time.Duration(res.Retry.Backoff)).
		SetRetryMaxWaitTime(time.Duration(res.Retry.MaxBackoff)).
		SetRetryConditions(retryConditions...).
		SetResult(&wire).
		Post(res.Path)
	if err != nil {
		c.logger.Error("cgr search failed", "error", err)
		return SearchResponse{}, fmt.Errorf("search dictamenes: %w", err)
	}
	if resp.IsStatusFailure() {
		return SearchResponse{}, fmt.Errorf("search dictamenes: unexpected status %d", resp.StatusCode())
	}

	total := wire.Hits.Total.Value
	results := make([]DictamenSummary, 0, len(wire.Hits.Hits))
	for _, h := range wire.Hits.Hits {
		s := h.Source
		ds := DictamenSummary{
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
		// Fallback: if doc_id differs from _id, prefer _id
		if ds.DictamenID == "" {
			ds.DictamenID = s.DocID
			ds.URL = dictamenURL(ds.DictamenID)
			ds.PDFURL = dictamenPDFURL(ds.DictamenID)
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
	c.searches.put(key, out)
	c.logger.Debug("cgr search ok",
		"query", params.Query,
		"results", len(results),
		"total", total,
		"page", params.Page,
	)
	return out, nil
}

// GetDictamen returns the full dictamen for a given dictamen_id via
// POST /search with exact_search:true. The dictamen is cached by id.
func (c *Client) GetDictamen(ctx context.Context, dictamenID string) (DictamenFull, error) {
	if dictamenID == "" {
		return DictamenFull{}, fmt.Errorf("dictamen_id is required")
	}
	if !dictamenIDRe.MatchString(dictamenID) {
		return DictamenFull{}, fmt.Errorf("invalid dictamen_id %q", dictamenID)
	}
	if v, ok := c.dictamenes.get(dictamenID); ok {
		return v, nil
	}
	v, err, _ := c.flights.Do(dictamenID, func() (any, error) {
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
	body := cgrSearchRequest{
		Search:      dictamenID,
		ExactSearch: true,
		Options:     []any{},
		Order:       "date",
		DateName:    "fecha_documento",
		Source:      "dictamenes",
		Page:        0,
	}
	var wire cgrSearchResponse
	resp, err := c.clients[resourceCgrSearch].R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://www.contraloria.cl").
		SetBody(body).
		SetRetryCount(res.Retry.Attempts).
		SetRetryWaitTime(time.Duration(res.Retry.Backoff)).
		SetRetryMaxWaitTime(time.Duration(res.Retry.MaxBackoff)).
		SetRetryConditions(retryConditions...).
		SetResult(&wire).
		Post(res.Path)
	if err != nil {
		c.logger.Error("cgr get dictamen failed", "error", err, "id", dictamenID)
		return DictamenFull{}, fmt.Errorf("get dictamen: %w", err)
	}
	if resp.IsStatusFailure() {
		return DictamenFull{}, fmt.Errorf("get dictamen: unexpected status %d", resp.StatusCode())
	}
	if wire.Hits.Total.Value == 0 || len(wire.Hits.Hits) == 0 {
		return DictamenFull{}, ErrDictamenNotFound
	}
	h := wire.Hits.Hits[0]
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
	c.dictamenes.put(dictamenID, full)
	c.logger.Debug("cgr get dictamen ok", "id", dictamenID, "char_count", full.CharCount)
	return full, nil
}

// CountJurisprudencia calls POST /apibusca/count/dictamenes and returns
// the cross-type aggregation buckets.
func (c *Client) CountJurisprudencia(ctx context.Context, query string, exactSearch bool) (CountResponse, error) {
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
		SetBody(body).
		SetRetryCount(res.Retry.Attempts).
		SetRetryWaitTime(time.Duration(res.Retry.Backoff)).
		SetRetryMaxWaitTime(time.Duration(res.Retry.MaxBackoff)).
		SetRetryConditions(retryConditions...).
		SetResult(&wire).
		Post(res.Path)
	if err != nil {
		c.logger.Error("cgr count failed", "error", err)
		return CountResponse{}, fmt.Errorf("count jurisprudencia: %w", err)
	}
	if resp.IsStatusFailure() {
		return CountResponse{}, fmt.Errorf("count jurisprudencia: unexpected status %d", resp.StatusCode())
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
	c.logger.Debug("cgr count ok", "query", query, "total", out.Total)
	return out, nil
}
