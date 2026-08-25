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
