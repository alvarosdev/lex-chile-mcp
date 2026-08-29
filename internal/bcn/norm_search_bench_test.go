package bcn

import (
	"encoding/json"
	"os"
	"testing"
)

// benchNorma loads the real Ley 21.600 fixture (largest in testdata,
// ~180K chars of converted markdown) and converts it once.
func benchNorma(tb testing.TB) NormaFull {
	tb.Helper()
	data, err := os.ReadFile("testdata/norma_full.json")
	if err != nil {
		tb.Fatal(err)
	}
	var norma NormaFull
	if err := json.Unmarshal(data, &norma); err != nil {
		tb.Fatal(err)
	}
	norma.ConvertContent(newConverter())
	return norma
}

// BenchmarkSearchNormaName guards the walk cost: a name-only query must
// stay linear in the structure (no content rendering at all).
func BenchmarkSearchNormaName(b *testing.B) {
	norma := benchNorma(b)
	b.ResetTimer()
	for range b.N {
		res := searchNorma(norma, "artículo 1")
		if len(res.Matches) == 0 {
			b.Fatal("expected matches")
		}
	}
}

// BenchmarkSearchNormaContent guards the worst case: a query hitting no
// name and forcing the content-piece walk over every subtree.
func BenchmarkSearchNormaContent(b *testing.B) {
	norma := benchNorma(b)
	b.ResetTimer()
	for range b.N {
		res := searchNorma(norma, "servicio")
		if len(res.Matches) == 0 {
			b.Fatal("expected matches")
		}
	}
}
