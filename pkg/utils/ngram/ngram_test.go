package ngram

import (
	"math"
	"strings"
	"testing"
)

func TestSearchExamples(t *testing.T) {
	ng, err := New([]string{"SPAM", "SPAN", "EG"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	results := ng.Search("SPA")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Item != "SPAM" || !almostEqual(results[0].Similarity, 0.375) {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
	if results[1].Item != "SPAN" || !almostEqual(results[1].Similarity, 0.375) {
		t.Fatalf("unexpected second result: %+v", results[1])
	}

	results = ng.Search("M")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Item != "SPAM" || !almostEqual(results[0].Similarity, 0.125) {
		t.Fatalf("unexpected result for query M: %+v", results[0])
	}

	results = ng.Search("EG")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for EG, got %d", len(results))
	}
	if results[0].Item != "EG" || !almostEqual(results[0].Similarity, 1.0) {
		t.Fatalf("unexpected result for query EG: %+v", results[0])
	}
}

func TestCompareExamples(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		opts []Option[string]
		want float64
	}{
		{name: "spa-spam", a: "spa", b: "spam", want: 0.375},
		{name: "ham-bam", a: "ham", b: "bam", want: 0.25},
		{name: "spam-pam", a: "spam", b: "pam", opts: []Option[string]{WithN[string](2)}, want: 0.5},
		{name: "ham-ams", a: "ham", b: "ams", opts: []Option[string]{WithN[string](1)}, want: 0.5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compare(tc.a, tc.b, tc.opts...)
			if err != nil {
				t.Fatalf("Compare returned error: %v", err)
			}
			if !almostEqual(got, tc.want) {
				t.Fatalf("Compare(%q, %q) = %f, want %f", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestFindExamples(t *testing.T) {
	ng, err := New([]string{"Spam", "Eggs", "Ham"}, WithKey[string](func(s string) string {
		return strings.ToLower(s)
	}), WithN[string](1))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if got, ok := ng.Find("Hom"); !ok || got != "Ham" {
		t.Fatalf("Find(\"Hom\") = %q, %v", got, ok)
	}

	if got, ok := ng.Find("Spom"); !ok || got != "Spam" {
		t.Fatalf("Find(\"Spom\") = %q, %v", got, ok)
	}

	if _, ok := ng.Find("Spom", 0.8); ok {
		t.Fatalf("expected Find with threshold 0.8 to fail")
	}
}

func TestSearchItemWithStructKey(t *testing.T) {
	type entry struct {
		id   int
		name string
	}

	items := []entry{
		{id: 1, name: "SPAM"},
		{id: 2, name: "SPAN"},
		{id: 3, name: "EG"},
	}

	ng, err := New(items, WithKey[entry](func(e entry) string { return e.name }))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	results := ng.SearchItem(entry{name: "SPA"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Item.id != 1 || results[1].Item.id != 2 {
		t.Fatalf("unexpected item order: %+v", results)
	}

	if found, ok := ng.FindItem(entry{name: "EG"}); !ok || found.id != 3 {
		t.Fatalf("FindItem returned unexpected result: %+v, %v", found, ok)
	}
}

func TestRemoveAndClear(t *testing.T) {
	ng, err := New([]string{"spam", "eggs"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if removed := ng.Remove("spam"); !removed {
		t.Fatalf("expected spam to be removed")
	}

	if results := ng.Search("spam"); len(results) != 0 {
		t.Fatalf("expected no results after removal, got %+v", results)
	}

	ng.Clear()
	if ng.Len() != 0 {
		t.Fatalf("expected Len() = 0 after Clear(), got %d", ng.Len())
	}
}

func TestItemsSharingNGrams(t *testing.T) {
	ng, err := New([]string{"ham", "spam", "eggs"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	shared := ng.ItemsSharingNGrams("mam")
	if len(shared) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(shared))
	}

	if shared["ham"] != 2 {
		t.Fatalf("expected ham to share 2 n-grams, got %d", shared["ham"])
	}
	if shared["spam"] != 2 {
		t.Fatalf("expected spam to share 2 n-grams, got %d", shared["spam"])
	}
}

func TestOptionValidation(t *testing.T) {
	if _, err := New([]string{"a"}, WithThreshold[string](-0.1)); err == nil {
		t.Fatalf("expected WithThreshold to reject negative value")
	}

	if _, err := New([]string{"a"}, WithWarp[string](0.5)); err == nil {
		t.Fatalf("expected WithWarp to reject value below 1")
	}

	if _, err := New([]string{"a"}, WithN[string](0)); err == nil {
		t.Fatalf("expected WithN to reject value below 1")
	}

	if _, err := New([]string{"a"}, WithPadLen[string](-1)); err == nil {
		t.Fatalf("expected WithPadLen to reject negative value")
	}

	if _, err := New([]string{"a"}, WithPadChar[string](0)); err == nil {
		t.Fatalf("expected WithPadChar to reject zero rune")
	}

	if _, err := New([]string{"a"}, WithKey[string](nil)); err == nil {
		t.Fatalf("expected WithKey to reject nil function")
	}
}

func TestSimilarityWarp(t *testing.T) {
	cases := []struct {
		same int
		all  int
		warp float64
		want float64
	}{
		{same: 5, all: 10, warp: 1, want: 0.5},
		{same: 5, all: 10, warp: 2, want: 0.75},
		{same: 5, all: 10, warp: 3, want: 0.875},
		{same: 2, all: 4, warp: 2, want: 0.75},
		{same: 3, all: 4, warp: 1, want: 0.75},
	}

	for _, tc := range cases {
		got := Similarity(tc.same, tc.all, tc.warp)
		if !almostEqual(got, tc.want) {
			t.Fatalf("Similarity(%d, %d, %f) = %f, want %f", tc.same, tc.all, tc.warp, got, tc.want)
		}
	}
}

func TestSearchRespectsMaxResults(t *testing.T) {
	items := []string{"alpha", "alpine", "alps", "albatross"}
	ng, err := New(items, WithN[string](1), WithMaxResults[string](2))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	results := ng.Search("al")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Similarity < results[1].Similarity {
		t.Fatalf("expected results to be sorted by similarity desc: %+v", results)
	}
}

func TestShortQueriesBypassCache(t *testing.T) {
	ng, err := New([]string{"bandung"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if len(ng.cache) != 0 {
		t.Fatalf("expected cache to be empty initially")
	}

	ng.Search("ba")

	if len(ng.cache) != 0 {
		t.Fatalf("expected short query to bypass cache, got %+v", ng.cache)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
