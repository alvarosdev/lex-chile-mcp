package cgr

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/alvarosdev/lex-chile-mcp/internal/config"
)

type CgrClientSuite struct {
	suite.Suite
}

func TestCgrClientSuite(t *testing.T) {
	suite.Run(t, new(CgrClientSuite))
}

func (s *CgrClientSuite) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (s *CgrClientSuite) testResources(serverURL string) *config.Resources {
	return &config.Resources{
		Version: 1,
		Resources: map[string]config.Resource{
			resourceCgrSearch: {
				URL:     serverURL,
				Path:    "/apibusca/search/dictamenes",
				Method:  "POST",
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
			resourceCgrCount: {
				URL:     serverURL,
				Path:    "/apibusca/count/dictamenes",
				Method:  "POST",
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

func (s *CgrClientSuite) TestSearch_Success() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("POST", r.Method)
		s.Equal("/apibusca/search/dictamenes", r.URL.Path)
		s.Equal("https://www.contraloria.cl", r.Header.Get("Origin"))
		data, _ := os.ReadFile(filepath.Join("testdata", "search_response.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	res, err := client.SearchDictamenes(context.Background(), SearchParams{Query: "quillota", Order: "date", Page: 1})
	s.Require().NoError(err)
	s.Len(res.Results, 2)
	s.Equal(312, res.Pagination.Total)
	s.Equal(1, res.Pagination.Page)
	s.Equal(20, res.Pagination.PageSize)
	s.Equal(16, res.Pagination.TotalPages)
	s.True(res.Pagination.HasMore)
	s.Equal("OF80660N26", res.Results[0].DictamenID)
	s.Equal("https://www.contraloria.cl/buscadorpdf/dictamenes/OF80660N26/html", res.Results[0].URL)
	s.Equal("https://www.contraloria.cl/buscadorpdf/dictamenes/OF80660N26/pdf", res.Results[0].PDFURL)
}

func (s *CgrClientSuite) TestSearch_PageBeyond() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":312,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	res, err := client.SearchDictamenes(context.Background(), SearchParams{Query: "quillota", Page: 99})
	s.Require().NoError(err)
	s.Len(res.Results, 0)
	s.Equal(312, res.Pagination.Total)
	s.False(res.Pagination.HasMore)
}

func (s *CgrClientSuite) TestSearch_OrderValidation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.SearchDictamenes(context.Background(), SearchParams{Query: "x", Order: "invalid"})
	s.Error(err)
	s.Contains(err.Error(), "invalid order")
}

func (s *CgrClientSuite) TestSearch_RetryOn5xx() {
	// Verify 5xx is handled as retryable: server returns 500, client should return error after retries exhausted.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	resources := s.testResources(server.URL)
	client := NewClient(resources, s.logger())
	_, err := client.SearchDictamenes(context.Background(), SearchParams{Query: "quillota", Page: 1})
	s.Error(err)
	s.Contains(err.Error(), "unexpected status 500")
}
func (s *CgrClientSuite) TestGetDictamen_Success() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GetDictamen uses POST /search with exact_search
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.Equal(true, body["exact_search"])
		data, _ := os.ReadFile(filepath.Join("testdata", "search_response.json"))
		// Wrap to return single hit for Get: filter to first hit
		var wire cgrSearchResponse
		_ = json.Unmarshal(data, &wire)
		wire.Hits.Total.Value = 1
		wire.Hits.Hits = wire.Hits.Hits[:1]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wire)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	full, err := client.GetDictamen(context.Background(), "E179593N25")
	s.Require().NoError(err)
	s.Equal("OF80660N26", full.DictamenID) // first hit in fixture is OF80660
	s.NotEmpty(full.Documento)
	s.Greater(full.CharCount, 0)
	s.Contains(full.URL, "OF80660N26/html")
	s.Contains(full.PDFURL, "OF80660N26/pdf")
}

func (s *CgrClientSuite) TestGetDictamen_NotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetDictamen(context.Background(), "E999999N99")
	s.ErrorIs(err, ErrDictamenNotFound)
}

func (s *CgrClientSuite) TestGetDictamen_Validation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.GetDictamen(context.Background(), "")
	s.Error(err)
	s.Contains(err.Error(), "dictamen_id is required")
	_, err = client.GetDictamen(context.Background(), "invalid")
	s.Error(err)
}

func (s *CgrClientSuite) TestCount_Success() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("/apibusca/count/dictamenes", r.URL.Path)
		data, _ := os.ReadFile(filepath.Join("testdata", "count_response.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	res, err := client.CountJurisprudencia(context.Background(), "quillota", false)
	s.Require().NoError(err)
	s.Equal(1255, res.Total)
	s.Len(res.Buckets, 3)
	s.Equal("dictamenes", res.Buckets[1].Type)
	s.Equal(312, res.Buckets[1].Count)
}

func (s *CgrClientSuite) TestSingleflight_Coalescing() {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		data, _ := os.ReadFile(filepath.Join("testdata", "search_response.json"))
		var wire cgrSearchResponse
		_ = json.Unmarshal(data, &wire)
		wire.Hits.Total.Value = 1
		wire.Hits.Hits = wire.Hits.Hits[:1]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wire)
	}))
	defer server.Close()

	client := NewClient(s.testResources(server.URL), s.logger())
	var wg sync.WaitGroup
	results := make([]DictamenFull, 10)
	errs := make([]error, 10)
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = client.GetDictamen(context.Background(), "E179593N25")
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		s.NoError(err)
	}
	s.Equal(int32(1), atomic.LoadInt32(&calls), "singleflight should coalesce to 1 call")
}

func (s *CgrClientSuite) TestLRU_Eviction() {
	cache := newLRUCache[int]()
	for i := range cacheMax + 5 {
		cache.put(string(rune('a'+i%26))+string(rune('0'+i%10)), i)
	}
	_, ok := cache.get("a0")
	s.False(ok)
}
func (s *CgrClientSuite) TestSearch_InvalidSource_NoNetwork() {
	var hit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(200)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	// path traversal
	_, err := client.Search(context.Background(), SearchParams{Query: "x", Source: "../count/todos", Page: 1})
	s.Error(err)
	s.Contains(err.Error(), "invalid source")
	s.False(hit, "invalid source must not perform I/O")
	hit = false
	_, err = client.Search(context.Background(), SearchParams{Query: "x", Source: "web%2fadmin", Page: 1})
	s.Error(err)
	s.Contains(err.Error(), "invalid source")
	s.False(hit)
	hit = false
	_, err = client.Search(context.Background(), SearchParams{Query: "x", Source: "invalido", Page: 1})
	s.Error(err)
	s.Contains(err.Error(), "invalid source")
	s.False(hit)
	// len >20
	_, err = client.Search(context.Background(), SearchParams{Query: "x", Source: strings.Repeat("a", 21), Page: 1})
	s.Error(err)
	s.Contains(err.Error(), "invalid source")
	s.False(hit)
}

func (s *CgrClientSuite) TestSearch_QueryTruncatedTo500() {
	long := strings.Repeat("a", 800)
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cgrSearchRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		received = body.Search
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	res, err := client.Search(context.Background(), SearchParams{Query: long, Source: "dictamenes", Page: 1, Order: "date"})
	s.Require().NoError(err)
	s.Equal(0, res.Pagination.Total)
	s.Equal(500, len([]rune(received)), "query should be truncated to 500 runes")
}

func (s *CgrClientSuite) TestSearch_PageOverflow() {
	var hit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(200)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.Search(context.Background(), SearchParams{Query: "x", Source: "dictamenes", Page: 501})
	s.Error(err)
	s.Contains(err.Error(), "page must be <= 500")
	s.False(hit)
	_, err = client.Search(context.Background(), SearchParams{Query: "x", Source: "dictamenes", Page: 9999})
	s.Error(err)
	s.Contains(err.Error(), "page must be <= 500")
	s.False(hit)
}

func (s *CgrClientSuite) TestSearch_ResponseTooLarge() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "7340032")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.Search(context.Background(), SearchParams{Query: "x", Source: "auditoria", Page: 1})
	s.Error(err)
	s.Contains(err.Error(), "response too large")
}

func (s *CgrClientSuite) TestSearch_NoInternalURLLeak() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile(filepath.Join("testdata", "search_cuentas_municipalidad.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	res, err := client.Search(context.Background(), SearchParams{Query: "municipalidad", Source: "cuentas", Page: 1})
	s.Require().NoError(err)
	s.Require().Len(res.Results, 1)
	s.NotContains(res.Results[0].URL, "172.30")
	s.NotContains(res.Results[0].PDFURL, "172.30")
	s.NotContains(res.Results[0].Materia, "172.30")
	// also ensure old_url not leaked via json marshal of result
	b, _ := json.Marshal(res)
	s.NotContains(string(b), "172.30")
	s.NotContains(string(b), "old_url")
}

func (s *CgrClientSuite) TestSearch_HitIndexMismatch() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a hit with wrong index cgr-dictamenes while searching instructivos
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"IN23N26","_source":{"doc_id":"IN23N26","n_dictamen":"IN23","materia":"X","carácter":"NNN","documento_completo":"doc","old_url":"http://172.30.21.160/x"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	res, err := client.Search(context.Background(), SearchParams{Query: "municipalidad", Source: "instructivos", Page: 1})
	s.Require().NoError(err)
	s.Len(res.Results, 0, "hit with mismatched _index should be filtered")
	s.Equal(1, res.Pagination.Total)
}

func (s *CgrClientSuite) TestCount_TodosBuckets() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("POST", r.Method)
		s.Equal("/apibusca/count/todos", r.URL.Path)
		data, _ := os.ReadFile(filepath.Join("testdata", "count_todos_municipalidad.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	resources := &config.Resources{
		Version: 1,
		Resources: map[string]config.Resource{
			resourceCgrSearch: {
				URL:            server.URL,
				Path:           "/apibusca/search",
				Method:         "POST",
				Timeout:        config.Duration(2 * time.Second),
				Retry:          config.Retry{Attempts: 1, Backoff: config.Duration(time.Millisecond), MaxBackoff: config.Duration(2 * time.Millisecond)},
				CircuitBreaker: config.CircuitBreaker{FailureThreshold: 100, SuccessThreshold: 1, ResetTimeout: config.Duration(time.Second)},
			},
			resourceCgrCount: {
				URL:            server.URL,
				Path:           "/apibusca/count/todos",
				Method:         "POST",
				Timeout:        config.Duration(2 * time.Second),
				Retry:          config.Retry{Attempts: 1, Backoff: config.Duration(time.Millisecond), MaxBackoff: config.Duration(2 * time.Millisecond)},
				CircuitBreaker: config.CircuitBreaker{FailureThreshold: 100, SuccessThreshold: 1, ResetTimeout: config.Duration(time.Second)},
			},
		},
	}
	client := NewClient(resources, s.logger())
	res, err := client.CountJurisprudencia(context.Background(), "municipalidad", false)
	s.Require().NoError(err)
	s.Equal(11961, res.Total)
	s.Len(res.Buckets, 8)
	// check specific buckets
	m := map[string]int{}
	for _, b := range res.Buckets {
		m[b.Type] = b.Count
	}
	s.Equal(9557, m["legislacion"])
	s.Equal(18, m["instructivos"])
	s.Equal(11, m["consolidados"])
}

func (s *CgrClientSuite) Test429_RetryAfter() {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"OF80660N26","_source":{"doc_id":"OF80660N26","n_dictamen":"OF80660","materia":"m","carácter":"NNN","documento_completo":"doc"}}]}}`))
	}))
	defer server.Close()
	resources := s.testResources(server.URL)
	// ensure retry attempts allow one retry
	resources.Resources[resourceCgrSearch] = config.Resource{
		URL:            server.URL,
		Path:           "/apibusca/search/dictamenes",
		Method:         "POST",
		Timeout:        config.Duration(3 * time.Second),
		Retry:          config.Retry{Attempts: 2, Backoff: config.Duration(time.Millisecond), MaxBackoff: config.Duration(10 * time.Millisecond)},
		CircuitBreaker: config.CircuitBreaker{FailureThreshold: 100, SuccessThreshold: 1, ResetTimeout: config.Duration(time.Second)},
	}
	client := NewClient(resources, s.logger())
	res, err := client.Search(context.Background(), SearchParams{Query: "quillota", Source: "dictamenes", Page: 1})
	s.Require().NoError(err)
	s.Len(res.Results, 1)
	s.Equal(int32(2), atomic.LoadInt32(&calls), "should have retried once after 429")
}

func (s *CgrClientSuite) TestGetInstructivo_ValidID() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cgrSearchRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.Equal("IN23N26", body.Search)
		s.True(body.ExactSearch)
		s.Equal("instructivos", body.Source)
		data, _ := os.ReadFile(filepath.Join("testdata", "search_instructivos_municipal.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	full, err := client.GetInstructivo(context.Background(), "IN23N26")
	s.Require().NoError(err)
	s.Equal("IN23N26", full.InstructivoID)
	s.Equal("IN23", full.NDictamen)
	s.Contains(full.Materia, "Día funcionario municipal")
	s.NotEmpty(full.Documento)
	s.Greater(full.CharCount, 0)
	s.NotContains(full.Documento, "172.30")
}

func (s *CgrClientSuite) TestGetInstructivo_Validation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.GetInstructivo(context.Background(), "")
	s.Error(err)
	_, err = client.GetInstructivo(context.Background(), strings.Repeat("A", 41))
	s.Error(err)
	s.Contains(err.Error(), "too long")
	_, err = client.GetInstructivo(context.Background(), "IN23/../count")
	s.Error(err)
	s.Contains(err.Error(), "invalid instructivo_id")
	_, err = client.GetInstructivo(context.Background(), "invalid")
	s.Error(err)
}

func (s *CgrClientSuite) TestGetInstructivo_NotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetInstructivo(context.Background(), "IN999999N99")
	s.ErrorIs(err, ErrInstructivoNotFound)
}

func (s *CgrClientSuite) TestGetInstructivo_CrossIndex() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"IN23N26","_source":{"doc_id":"IN23N26","n_dictamen":"IN23","materia":"x","carácter":"NNN","documento_completo":"doc"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetInstructivo(context.Background(), "IN23N26")
	s.ErrorIs(err, ErrInstructivoNotFound)
}

func (s *CgrClientSuite) TestGetContable_ValidID() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cgrSearchRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.Equal("contable", body.Source)
		data, _ := os.ReadFile(filepath.Join("testdata", "search_contable_municipalidad.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	full, err := client.GetContable(context.Background(), "E080961")
	s.Require().NoError(err)
	s.Equal("E080961", full.Numero)
	s.Equal("OFE0809612600", full.NormativaContable)
	s.Contains(full.Parte, "La Pintana")
	s.NotEmpty(full.Texto)
	s.NotContains(full.Texto, "172.30")
}

func (s *CgrClientSuite) TestGetContable_Validation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.GetContable(context.Background(), "")
	s.Error(err)
	_, err = client.GetContable(context.Background(), strings.Repeat("E", 41))
	s.Error(err)
	s.Contains(err.Error(), "too long")
	_, err = client.GetContable(context.Background(), "E080961/../count")
	s.Error(err)
	s.Contains(err.Error(), "invalid contable_id")
	_, err = client.GetContable(context.Background(), "E080961\n")
	s.Error(err)
	_, err = client.GetContable(context.Background(), "bad!!")
	s.Error(err)
}

func (s *CgrClientSuite) TestGetContable_NotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetContable(context.Background(), "E999999")
	s.ErrorIs(err, ErrContableNotFound)
}

func (s *CgrClientSuite) TestGetContable_CrossIndex() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"E080961","_source":{"número":"E080961","tipo":"Oficio","parte":"x","texto":"y"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetContable(context.Background(), "E080961")
	s.ErrorIs(err, ErrContableNotFound)
}

func (s *CgrClientSuite) TestGetAuditoria_ValidID() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cgrSearchRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.Equal("auditoria", body.Source)
		data, _ := os.ReadFile(filepath.Join("testdata", "search_auditoria_licencias.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	full, err := client.GetAuditoria(context.Background(), "371/2026")
	s.Require().NoError(err)
	s.Equal("371/2026", full.Numero)
	s.Contains(full.Nombre, "QUILLOTA")
	s.Contains(full.Objetivo, "Banco Estado")
	s.NotContains(full.ContenidoPDF, "<?xml")
	s.NotContains(full.ContenidoPDF, "pdf:PDFVersion")
	s.NotContains(full.ContenidoPDF, "172.30")
	s.Greater(full.CharCount, 0)
}

func (s *CgrClientSuite) TestGetAuditoria_Validation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.GetAuditoria(context.Background(), "")
	s.Error(err)
	_, err = client.GetAuditoria(context.Background(), strings.Repeat("1", 41))
	s.Error(err)
	s.Contains(err.Error(), "too long")
	_, err = client.GetAuditoria(context.Background(), "371/2026%00")
	s.Error(err)
	_, err = client.GetAuditoria(context.Background(), "bad!!")
	s.Error(err)
}

func (s *CgrClientSuite) TestGetAuditoria_NotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetAuditoria(context.Background(), "999/2026")
	s.ErrorIs(err, ErrAuditoriaNotFound)
}

func (s *CgrClientSuite) TestGetAuditoria_CrossIndex() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"371/2026","_source":{"número":"371/2026","nombre":"x"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetAuditoria(context.Background(), "371/2026")
	s.ErrorIs(err, ErrAuditoriaNotFound)
}

func (s *CgrClientSuite) TestGetConsolidado_ValidID() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cgrSearchRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.Equal("consolidados", body.Source)
		data, _ := os.ReadFile(filepath.Join("testdata", "search_consolidado_licencias.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	full, err := client.GetConsolidado(context.Background(), "CIC21/2026")
	s.Require().NoError(err)
	s.Equal("CIC21/2026", full.Numero)
	s.Contains(full.Resena, "licencia")
	s.NotEmpty(full.ContenidoExtraido)
	s.NotContains(full.ContenidoExtraido, "172.30")
	s.Greater(full.CharCount, 0)
}

func (s *CgrClientSuite) TestGetConsolidado_Validation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.GetConsolidado(context.Background(), "")
	s.Error(err)
	_, err = client.GetConsolidado(context.Background(), strings.Repeat("A", 21))
	s.Error(err)
	s.Contains(err.Error(), "too long")
	_, err = client.GetConsolidado(context.Background(), "CIC21/2026\n")
	s.Error(err)
	_, err = client.GetConsolidado(context.Background(), "bad")
	s.Error(err)
	_, err = client.GetConsolidado(context.Background(), "21/2026")
	s.Error(err)
}

func (s *CgrClientSuite) TestGetConsolidado_NotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetConsolidado(context.Background(), "CIC99/2026")
	s.ErrorIs(err, ErrConsolidadoNotFound)
}

func (s *CgrClientSuite) TestGetConsolidado_CrossIndex() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"CIC21/2026","_source":{"numero":"CIC21/2026","nombre":"x"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetConsolidado(context.Background(), "CIC21/2026")
	s.ErrorIs(err, ErrConsolidadoNotFound)
}

func (s *CgrClientSuite) TestGetCuenta_ValidID() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cgrSearchRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.Equal("cuentas", body.Source)
		data, _ := os.ReadFile(filepath.Join("testdata", "search_cuentas_municipalidad.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	full, err := client.GetCuenta(context.Background(), "2982331")
	s.Require().NoError(err)
	s.Equal("2982331", full.NumeroSentencia)
	s.Contains(full.Texto, "Juzgado de Cuentas")
	s.NotContains(full.Texto, "172.30")
	s.Greater(full.CharCount, 0)
}

func (s *CgrClientSuite) TestGetCuenta_Validation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.GetCuenta(context.Background(), "")
	s.Error(err)
	_, err = client.GetCuenta(context.Background(), strings.Repeat("1", 21))
	s.Error(err)
	s.Contains(err.Error(), "too long")
	_, err = client.GetCuenta(context.Background(), "2982331/../count")
	s.Error(err)
	s.Contains(err.Error(), "invalid cuenta_id")
	_, err = client.GetCuenta(context.Background(), "abc")
	s.Error(err)
}

func (s *CgrClientSuite) TestGetCuenta_NotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetCuenta(context.Background(), "999999")
	s.ErrorIs(err, ErrCuentaNotFound)
}

func (s *CgrClientSuite) TestGetCuenta_CrossIndex() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"2982331","_source":{"numero_sentencia":"2982331","texto":"x"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetCuenta(context.Background(), "2982331")
	s.ErrorIs(err, ErrCuentaNotFound)
}

func (s *CgrClientSuite) TestGetLegislacion_ValidID() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cgrSearchRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.Equal("legislacion", body.Source)
		data, _ := os.ReadFile(filepath.Join("testdata", "search_legislacion_toma.json"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	full, err := client.GetLegislacion(context.Background(), "RZA005691400")
	s.Require().NoError(err)
	s.Equal("569", full.Numero)
	s.Equal("RESOLUCION", full.Tipo)
	s.Contains(full.Materias, "toma de razón")
	s.NotEmpty(full.Texto)
	s.NotContains(full.Texto, "172.30")
	s.Greater(full.CharCount, 0)
}

func (s *CgrClientSuite) TestGetLegislacion_Validation() {
	client := NewClient(s.testResources("http://example.com"), s.logger())
	_, err := client.GetLegislacion(context.Background(), "")
	s.Error(err)
	_, err = client.GetLegislacion(context.Background(), strings.Repeat("R", 41))
	s.Error(err)
	s.Contains(err.Error(), "too long")
	_, err = client.GetLegislacion(context.Background(), "RZA005691400/../count")
	s.Error(err)
	s.Contains(err.Error(), "invalid legislacion_id")
	_, err = client.GetLegislacion(context.Background(), "bad")
	s.Error(err)
}

func (s *CgrClientSuite) TestGetLegislacion_NotFound() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetLegislacion(context.Background(), "RZA999999999")
	s.ErrorIs(err, ErrLegislacionNotFound)
}

func (s *CgrClientSuite) TestGetLegislacion_CrossIndex() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"cgr-dictamenes","_id":"RZA005691400","_source":{"tipo":"RESOLUCION","número":"569","texto_":"x"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(s.testResources(server.URL), s.logger())
	_, err := client.GetLegislacion(context.Background(), "RZA005691400")
	s.ErrorIs(err, ErrLegislacionNotFound)
}

func (s *CgrClientSuite) TestSanitize_StripTags() {
	// Ensure sanitize strips tags and decodes entities
	out := SanitizeTexto("<td>hello&nbsp;world</td>")
	s.Equal("hello world", out)
	s.NotContains(out, "<")
}
