package bcn

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"resty.dev/v3"

	"github.com/alvarosdev/lex-chile-mcp/internal/config"
)

// LawClientSuite exercises the real resty client against a local
// httptest.Server — no external network calls, ever.
type LawClientSuite struct {
	suite.Suite
}

func TestLawClientSuite(t *testing.T) {
	suite.Run(t, new(LawClientSuite))
}

func (s *LawClientSuite) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testResources builds the contract pointing at the local test server.
// Defaults: 1 retry, tiny backoff, high breaker threshold (no interference).
func (s *LawClientSuite) testResources(serverURL string) *config.Resources {
	return &config.Resources{
		Version: 1,
		Resources: map[string]config.Resource{
			resourceSearchLaws: {
				URL:     serverURL,
				Path:    "/buscarjson",
				Method:  "GET",
				Timeout: config.Duration(2 * time.Second),
				Retry: config.Retry{
					Attempts:   1,
					Backoff:    config.Duration(time.Millisecond),
					MaxBackoff: config.Duration(2 * time.Millisecond),
				},
				CircuitBreaker: config.CircuitBreaker{
					FailureThreshold: 100,
					SuccessThreshold: 1,
					ResetTimeout:     config.Duration(time.Second),
				},
			},
			resourceGetLaw: {
				URL:     serverURL,
				Path:    "/get_norma_json",
				Method:  "GET",
				Timeout: config.Duration(2 * time.Second),
				Retry: config.Retry{
					Attempts:   1,
					Backoff:    config.Duration(time.Millisecond),
					MaxBackoff: config.Duration(2 * time.Millisecond),
				},
				CircuitBreaker: config.CircuitBreaker{
					FailureThreshold: 100,
					SuccessThreshold: 1,
					ResetTimeout:     config.Duration(time.Second),
				},
			},
			resourceGetLawHist: {
				URL:     serverURL,
				Path:    "/get_historias_de_ley",
				Method:  "GET",
				Timeout: config.Duration(2 * time.Second),
				Retry: config.Retry{
					Attempts:   1,
					Backoff:    config.Duration(time.Millisecond),
					MaxBackoff: config.Duration(2 * time.Millisecond),
				},
				CircuitBreaker: config.CircuitBreaker{
					FailureThreshold: 100,
					SuccessThreshold: 1,
					ResetTimeout:     config.Duration(time.Second),
				},
			},
		},
	}
}

func (s *LawClientSuite) fixture(name string) []byte {
	data, err := os.ReadFile(filepath.Join("testdata", name))
	s.Require().NoError(err)
	return data
}

func (s *LawClientSuite) TestSearchParsesRealResponse() {
	searchJSON := s.fixture("search_response.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("/buscarjson", r.URL.Path)
		s.Equal("Ley 21.600", r.URL.Query().Get("cadena"))
		s.Equal("1", r.URL.Query().Get("npagina"))
		s.Equal("3", r.URL.Query().Get("itemsporpagina"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(searchJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	result, err := client.Search(context.Background(), SearchParams{
		Query: "Ley 21.600", Page: 1, PageSize: 3,
	})
	s.Require().NoError(err)
	s.Require().Len(result.Results, 3)
	s.Equal(FlexInt(140), result.Pagination.TotalItems)

	first := result.Results[0]
	s.Equal(int64(1195666), first.IDNorma)
	s.Equal("Ley", first.Tipo)
	s.Contains(first.TituloNorma, "BIODIVERSIDAD")
	// The XML wrapper and its indentation are gone; entities are decoded.
	s.NotContains(first.Resumen, "<RESUMENES>")
	s.Contains(first.Resumen, "Servicio de Biodiversidad")
	s.NotContains(first.Resumen, " ")
}

func (s *LawClientSuite) TestSearchParsesNumericPagination() {
	// Real wire captured from buscarjson for "Ley 21461": the pagination
	// block mixes shapes in ONE response — npagina arrives as a string
	// while itemsporpagina and totalitems arrive as numbers. The old
	// string-typed Pagination crashed here with "cannot unmarshal number".
	searchJSON := s.fixture("search_response_numeric.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(searchJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	result, err := client.Search(context.Background(), SearchParams{
		Query: "Ley 21461", Page: 1, PageSize: 10,
	})
	s.Require().NoError(err)

	// Mixed shapes decode field by field.
	s.Equal(FlexInt(4), result.Pagination.TotalItems)
	s.Equal(FlexInt(1), result.Pagination.Page)
	s.Equal(FlexInt(4), result.Pagination.PageSize)
	s.Equal("Ley 21461", result.Pagination.Query)

	// Ley 21461 is the first result, with the id the tools depend on.
	s.Require().NotEmpty(result.Results)
	s.Equal(int64(1178004), result.Results[0].IDNorma)
}

func (s *LawClientSuite) TestSearchParsesEmptyAndNullPagination() {
	// Empty string and null numeric fields decode to 0 instead of failing
	// the whole search; the string-shaped fixture above stays as the
	// regression for the all-strings wire.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[[], {"npagina": null, "itemsporpagina": "10", "totalitems": ""}, []]`))
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	result, err := client.Search(context.Background(), SearchParams{
		Query: "x", Page: 1, PageSize: 10,
	})
	s.Require().NoError(err)
	s.Equal(FlexInt(0), result.Pagination.TotalItems)
	s.Equal(FlexInt(0), result.Pagination.Page)
	s.Equal(FlexInt(10), result.Pagination.PageSize)
}

func (s *LawClientSuite) TestSearchRetriesTransient5xx() {
	searchJSON := s.fixture("search_response.json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(searchJSON)
	}))
	defer server.Close()

	res := s.testResources(server.URL)
	searchRes := res.Resources[resourceSearchLaws]
	searchRes.Retry.Attempts = 3 // 2 failures + 1 success
	res.Resources[resourceSearchLaws] = searchRes
	client := NewClient(res, s.logger())

	result, err := client.Search(context.Background(), SearchParams{
		Query: "Ley 21.600", Page: 1, PageSize: 3,
	})
	s.Require().NoError(err)
	s.Len(result.Results, 3)
	s.Equal(int32(3), calls.Load())
}

func (s *LawClientSuite) TestCircuitBreakerOpensAfterFailures() {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	res := s.testResources(server.URL)
	searchRes := res.Resources[resourceSearchLaws]
	searchRes.Retry.Attempts = 0 // no retries
	searchRes.CircuitBreaker.FailureThreshold = 2
	res.Resources[resourceSearchLaws] = searchRes
	client := NewClient(res, s.logger())

	params := SearchParams{Query: "x", Page: 1, PageSize: 1}
	_, err1 := client.Search(context.Background(), params)
	s.Require().Error(err1)
	_, err2 := client.Search(context.Background(), params)
	s.Require().Error(err2)
	s.Equal(int32(2), calls.Load(), "two requests reached the server")

	// The breaker is open: the third call fails without touching the server.
	_, err3 := client.Search(context.Background(), params)
	s.Require().Error(err3)
	s.True(errors.Is(err3, resty.ErrCircuitBreakerOpen), "expected breaker-open error, got: %v", err3)
	s.Equal(int32(2), calls.Load(), "no third request reached the server")
}

func (s *LawClientSuite) TestGetNormaParsesRealResponse() {
	normaJSON := s.fixture("norma_full.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("/get_norma_json", r.URL.Path)
		s.Equal("1195666", r.URL.Query().Get("idNorma"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	norma, err := client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)

	// Metadata.
	s.Equal("CREA EL SERVICIO DE BIODIVERSIDAD Y ÁREAS PROTEGIDAS Y EL SISTEMA NACIONAL DE ÁREAS PROTEGIDAS", norma.Metadatos.TituloNorma)
	s.Equal("Ley", norma.Metadatos.TiposNumeros[0].Descripcion)
	s.Equal("21600", norma.Metadatos.TiposNumeros[0].Numero)
	s.Contains(norma.Metadatos.Organismos, "MINISTERIO DEL MEDIO AMBIENTE")
	s.False(norma.Metadatos.Derogado)
	s.Equal("2023-09-06", norma.Metadatos.FechaPublicacion)
	s.Equal("2025-09-29", norma.Metadatos.Vigencia.InicioVigencia)
	s.NotEmpty(norma.Metadatos.Materias)

	// Structure and content: 10 top-level blocks, all converted to markdown.
	s.Require().Len(norma.Html, 10)
	s.Require().Len(norma.Estructura, 10)

	header := norma.Html[0]
	s.Equal("Encabezado", header.SectionName)
	// Entities decoded, nbsp indentation gone, no raw HTML left.
	s.Contains(header.Markdown, "LEY NÚM. 21.600")
	s.NotContains(header.Markdown, "&#xDA;")
	s.NotContains(header.Markdown, " ")
	s.NotContains(header.Markdown, "<div")

	// A titled section pairs structure[i] ↔ html[i].
	idx := slices.IndexFunc(norma.Html, func(block HtmlBlock) bool {
		return strings.Contains(block.SectionName, "TÍTULO I")
	})
	s.Require().NotEqual(-1, idx, "TÍTULO I section with content expected")
	s.NotEmpty(norma.Html[idx].Markdown)
}

func (s *LawClientSuite) TestGetNormaParsesNestedArticles() {
	normaJSON := s.fixture("norma_full.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	norma, err := client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)

	// The API nests articles under titles (field "h"): TÍTULO I has 3
	// article children, and the article text must be converted.
	titulo := norma.Html[1]
	s.Require().Len(titulo.H, 3, "TÍTULO I must carry its nested articles")
	s.Equal("Artículo 1°", titulo.H[0].SectionName)
	s.Contains(titulo.H[0].Markdown, "Artículo 1°.- Objeto")
	s.NotContains(titulo.H[0].Markdown, "<div", "nested article must be converted to markdown")

	// The structure tree mirrors the nesting.
	s.Equal("Artículo 1°", norma.Estructura[1].H[0].N)

	// Deeper nesting: TÍTULO II → Párrafo 1° → Artículo 4°.
	tituloII := norma.Html[2]
	s.Require().NotEmpty(tituloII.H, "TÍTULO II must have paragraphs")
	parrafo := tituloII.H[0]
	s.Require().NotEmpty(parrafo.H, "Párrafo must have articles")
	s.Equal("Artículo 4°", parrafo.H[0].SectionName)
}

func (s *LawClientSuite) TestGetNormaServes304FromCache() {
	normaJSON := s.fixture("norma_full.json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `W/"abc123"`)
			_, _ = w.Write(normaJSON)
			return
		}
		s.Equal(`W/"abc123"`, r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	first, err := client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)
	second, err := client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)

	// The second call was served from cache: only ONE 200 download happened
	// and the content is identical (already converted, no re-fetch).
	s.Equal(int32(2), calls.Load(), "one download + one revalidation")
	s.Equal(first.Metadatos.TituloNorma, second.Metadatos.TituloNorma)
	s.Equal(first.Html[0].Markdown, second.Html[0].Markdown)
}

func (s *LawClientSuite) TestGetNormaReplacesCacheOnNewETag() {
	normaJSON := s.fixture("norma_full.json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.Header().Set("ETag", `W/"old"`)
		} else {
			w.Header().Set("ETag", `W/"new"`)
		}
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)
	// Server always answers 200: the cache entry must be replaced, not served.
	_, err = client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)
	s.Equal(int32(2), calls.Load())
	entry, ok := client.normas.get(normaCacheKey(NormaQuery{NormID: 1195666}))
	s.Require().True(ok)
	s.Equal(`W/"new"`, entry.etag)
}

func (s *LawClientSuite) TestGetNormaSummaryParsesRealResponse() {
	normaJSON := s.fixture("norma_full.json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	summary, err := client.GetNormaSummary(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)
	s.Equal("CREA EL SERVICIO DE BIODIVERSIDAD Y ÁREAS PROTEGIDAS Y EL SISTEMA NACIONAL DE ÁREAS PROTEGIDAS", summary.TituloNorma)
	s.Equal("Diario Oficial", summary.Fuente)
	s.NotEmpty(summary.Materias)
	s.NotEmpty(summary.Resumenes)
	// Sanitized: no leading indentation from the API.
	s.False(strings.HasPrefix(summary.Resumenes[0], " "), "resumen must not start with spaces")
}

func (s *LawClientSuite) TestGetNormaSummaryServesFromCacheWithoutHTTP() {
	normaJSON := s.fixture("norma_full.json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	first, err := client.GetNormaSummary(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)
	second, err := client.GetNormaSummary(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)

	// The second summary was derived from the cache: ONE request total.
	s.Equal(int32(1), calls.Load(), "second summary must not touch the network")
	s.Equal(first.TituloNorma, second.TituloNorma)
}

func (s *LawClientSuite) TestGetNormaSummaryNotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("El id de la Norma proporcionado (999999999) no se encuentra en nuestra Base de Datos."))
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetNormaSummary(context.Background(), NormaQuery{NormID: 999999999})
	s.Require().Error(err)
	s.True(errors.Is(err, ErrNormaNotFound), "expected ErrNormaNotFound, got: %v", err)
}

func (s *LawClientSuite) TestGetNormaHistoricalVersion() {
	latestJSON := s.fixture("norma_full.json")
	oldJSON := s.fixture("norma_2010.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("idVersion") == "2010-01-01" {
			_, _ = w.Write(oldJSON)
			return
		}
		s.Equal("", r.URL.Query().Get("idVersion"), "no version_date must not send idVersion")
		_, _ = w.Write(latestJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	latest, err := client.GetNorma(context.Background(), NormaQuery{NormID: 141599})
	s.Require().NoError(err)
	old, err := client.GetNorma(context.Background(), NormaQuery{NormID: 141599, VersionDate: "2010-01-01"})
	s.Require().NoError(err)

	// The API returns different content per version; both are cached apart.
	s.NotEqual(latest.Metadatos.Vigencia, old.Metadatos.Vigencia, "versions must differ")
	s.Equal(latest.Metadatos.Vigencia, latest.Metadatos.Vigencia)
}

func (s *LawClientSuite) TestGetNormaVersionsDoNotMixInCache() {
	latestJSON := s.fixture("norma_full.json")
	oldJSON := s.fixture("norma_2010.json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", r.URL.Query().Get("idVersion"))
		if r.URL.Query().Get("idVersion") == "2010-01-01" {
			_, _ = w.Write(oldJSON)
			return
		}
		_, _ = w.Write(latestJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetNorma(context.Background(), NormaQuery{NormID: 141599})
	s.Require().NoError(err)
	_, err = client.GetNorma(context.Background(), NormaQuery{NormID: 141599, VersionDate: "2010-01-01"})
	s.Require().NoError(err)

	// Two DIFFERENT cache entries: the second call was a cache miss and
	// hit the network (not served from the other version's entry).
	s.Equal(int32(2), calls.Load(), "versions must be cached separately")
}

func (s *LawClientSuite) TestGetNormaEnrichesCanonicalTypes() {
	normaJSON := s.fixture("norma_full.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	norma, err := client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().NoError(err)
	tipo := norma.Metadatos.TiposNumeros[0]
	s.Equal("Ley", tipo.CanonicalType)
	s.Equal("LEY", tipo.CanonicalAbbr)
	// Raw values stay intact (append, never replace).
	s.Equal("1", tipo.Tipo)
	s.Equal("Ley", tipo.Descripcion)
}

func (s *LawClientSuite) TestGetLawHistoryParsesRealResponse() {
	historyJSON := s.fixture("history_full.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("/get_historias_de_ley", r.URL.Path)
		s.Equal("1195666", r.URL.Query().Get("idNorma"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(historyJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	grupos, err := client.GetLawHistory(context.Background(), 1195666)
	s.Require().NoError(err)
	s.Require().Len(grupos, 3)

	s.Equal(1, grupos[0].TipoCod)
	s.Len(grupos[0].Hls, 1)
	s.Equal(3, grupos[1].TipoCod) // modificatorias
	s.Len(grupos[1].Hls, 3)
	s.Equal(4, grupos[2].TipoCod) // modificadas
	s.Len(grupos[2].Hls, 9)

	// The entry for Ley 21.770: id_norma_hl is the record's norm id.
	modificatoria := grupos[1].Hls[0]
	s.Equal(int64(1216930), modificatoria.IDNormaHL)
	s.Equal(int64(1195666), modificatoria.IDNorma, "id_norma points to the related norm")
}

func (s *LawClientSuite) TestGetLawHistoryServes304FromCache() {
	historyJSON := s.fixture("history_full.json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `W/"hist1"`)
			_, _ = w.Write(historyJSON)
			return
		}
		s.Equal(`W/"hist1"`, r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	first, err := client.GetLawHistory(context.Background(), 1195666)
	s.Require().NoError(err)
	second, err := client.GetLawHistory(context.Background(), 1195666)
	s.Require().NoError(err)

	s.Equal(int32(2), calls.Load(), "one download + one revalidation")
	s.Len(first, 3)
	s.Len(second, 3)
}

func (s *LawClientSuite) TestGetLawHistoryEmptyResult() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	grupos, err := client.GetLawHistory(context.Background(), 999999999)
	s.Require().NoError(err)
	s.Empty(grupos)
}

func (s *LawClientSuite) TestGetNormaNotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("El id de la Norma proporcionado (999999999) no se encuentra en nuestra Base de Datos."))
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetNorma(context.Background(), NormaQuery{NormID: 999999999})
	s.Require().Error(err)
	s.True(errors.Is(err, ErrNormaNotFound), "expected ErrNormaNotFound, got: %v", err)
}

func (s *LawClientSuite) TestGetNormaRejectsOversizedResponse() {
	// The body cap is a hard guard: a misbehaving upstream must error, never
	// OOM the process (GOMEMLIMIT is 256MiB in the image).
	big := strings.Repeat("x", maxNormResponseBytes+100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(big))
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
	s.Require().Error(err)
	s.Contains(err.Error(), "exceeds")
}

func (s *LawClientSuite) TestGetNormaCoalescesConcurrentRequests() {
	// singleflight: 8 concurrent requests for the same (norm, version) must
	// share ONE call to BCN. The server holds the response so every goroutine
	// piles up on the same in-flight call before it completes.
	normaJSON := s.fixture("norma_full.json")
	var calls atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())

	var started atomic.Int32
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			started.Add(1)
			_, errs[i] = client.GetNorma(context.Background(), NormaQuery{NormID: 1195666})
			wg.Done()
		}(i)
	}

	// Deterministic enough: wait until every goroutine has entered GetNorma
	// and a small grace beat for the last ones to join the flight, then
	// release the leader's response.
	for started.Load() < 8 {
		time.Sleep(time.Millisecond)
	}
	s.Require().Equal(int32(1), calls.Load(), "only the flight leader may hit the server")
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	s.Equal(int32(1), calls.Load(), "concurrent requests must share ONE call to BCN")
	for _, err := range errs {
		s.Require().NoError(err)
	}
}

func (s *LawClientSuite) TestGetNormaConvertsConcurrentlyWithPool() {
	// 8 DIFFERENT norms in flight exercise the converter pool: every
	// conversion gets its own converter (no shared mutex), no deadlock, and
	// each result is converted correctly.
	normaJSON := s.fixture("norma_full.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(normaJSON)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())

	var wg sync.WaitGroup
	errs := make([]error, 8)
	headers := make([]string, 8)
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			norma, err := client.GetNorma(context.Background(), NormaQuery{NormID: int64(1195666 + i)})
			errs[i] = err
			if err == nil && len(norma.Html) > 0 {
				headers[i] = norma.Html[0].Markdown
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		s.Require().NoError(err, "concurrent conversion %d must succeed", i)
		s.Contains(headers[i], "LEY NÚM. 21.600", "concurrent conversion %d must be correct", i)
	}
}
