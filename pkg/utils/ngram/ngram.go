package ngram

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// Result captures the outcome of a fuzzy search.
type Result[T comparable] struct {
	Item       T
	Similarity float64
}

// Option configures an NGram instance during construction.
type Option[T comparable] func(*NGram[T]) error

// WithThreshold sets the minimum similarity required to include a result.
func WithThreshold[T comparable](threshold float64) Option[T] {
	return func(ng *NGram[T]) error {
		if threshold < 0 || threshold > 1 {
			return fmt.Errorf("threshold out of range 0.0 to 1.0: %v", threshold)
		}
		ng.threshold = threshold
		return nil
	}
}

// WithWarp increases the weight of shorter matches when greater than 1.0.
func WithWarp[T comparable](warp float64) Option[T] {
	return func(ng *NGram[T]) error {
		if warp < 1.0 || warp > 3.0 {
			return fmt.Errorf("warp out of range 1.0 to 3.0: %v", warp)
		}
		ng.warp = warp
		return nil
	}
}

// WithN configures the number of characters per n-gram window.
func WithN[T comparable](n int) Option[T] {
	return func(ng *NGram[T]) error {
		if n < 1 {
			return fmt.Errorf("N out of range (should be N >= 1): %d", n)
		}
		ng.N = n
		return nil
	}
}

// WithPadLen returns an Option that configures the number of padding characters applied to each side of indexed keys.
// The provided padLen must be >= 0; if it is negative the Option returns an error.
func WithPadLen[T comparable](padLen int) Option[T] {
	return func(ng *NGram[T]) error {
		if padLen < 0 {
			return fmt.Errorf("pad_len out of range: %d", padLen)
		}
		ng.padLen = padLen
		return nil
	}
}

// WithPadChar returns an Option that sets the rune used to build the padding string for the index.
// WithPadChar returns an Option that sets the padding character used when building
// padded n-gram keys for the index. It validates that padChar is not the zero rune
// and returns an error if validation fails.
func WithPadChar[T comparable](padChar rune) Option[T] {
	return func(ng *NGram[T]) error {
		if padChar == 0 {
			return errors.New("pad_char must be a valid rune")
		}
		ng.padChar = padChar
		return nil
	}
}

// WithKey returns an Option that sets the function used to extract a string key from an item.
// If the provided key function is nil the option will return an error when applied.
// When the item type T is a string, this also configures the index to normalize string queries
// WithKey returns an Option that sets the function used to extract a string key from an item.
// It validates that the provided key is non-nil and assigns it to the index. If the item type T is
// function to incoming query strings via ng.normalizeQuery.
func WithKey[T comparable](key func(T) string) Option[T] {
	return func(ng *NGram[T]) error {
		if key == nil {
			return errors.New("key function cannot be nil")
		}
		ng.key = key
		var zero T
		if _, ok := any(zero).(string); ok {
			ng.normalizeQuery = func(s string) string {
				val, _ := any(s).(T)
				return key(val)
			}
		}
		return nil
	}
}

// NGram implements fuzzy search over a set of comparable items using n-gram similarity.
type NGram[T comparable] struct {
	threshold float64
	warp      float64
	N         int
	padLen    int
	padChar   rune

	key            func(T) string
	normalizeQuery func(string) string

	padding string
	grams   map[string]map[T]int
	length  map[T]int
	items   map[T]struct{}

	// Add cache for frequently searched queries
	cache     map[string][]Result[T]
	cacheMu   sync.RWMutex
	cacheSize int
}

// New creates an NGram index configured by the supplied options and populated with the provided items.
// 
// The function applies each Option in order and returns an error if any option fails. It establishes sensible defaults
// (N = 3, warp = 1.0, threshold = 0, padChar = '$', padLen defaults to N-1 when unset) and initializes internal
// structures including gram buckets, item length/map, item set, and a results cache (default size 1000). If an initial
// New creates an NGram index configured for fuzzy n-gram similarity searches.
// 
// The function constructs an NGram with sensible defaults (threshold 0, warp 1.0, N=3,
// padChar '$', padLen computed as N-1 when unset), initializes internal maps and a
// query cache, applies the provided Option functions in order, and validates options.
// If padLen remains >= N after options, New returns an error. If a non-empty items
// slice is provided, those items are indexed before the constructed index is returned.
func New[T comparable](items []T, options ...Option[T]) (*NGram[T], error) {
	ng := &NGram[T]{
		threshold:      0,
		warp:           1.0,
		N:              3,
		padLen:         -1,
		padChar:        '$',
		key:            defaultKey[T],
		normalizeQuery: func(s string) string { return s },
		grams:          make(map[string]map[T]int),
		length:         make(map[T]int),
		items:          make(map[T]struct{}),
		cache:          make(map[string][]Result[T]),
		cacheSize:      1000, // Default cache size
	}

	for _, opt := range options {
		if err := opt(ng); err != nil {
			return nil, err
		}
	}

	if ng.padLen < 0 {
		ng.padLen = ng.N - 1
	}

	if ng.padLen >= ng.N {
		return nil, fmt.Errorf("pad_len out of range: %d", ng.padLen)
	}

	ng.padding = strings.Repeat(string(ng.padChar), ng.padLen)

	if len(items) > 0 {
		ng.Update(items)
	}

	return ng, nil
}

// Copy returns a shallow copy of the index configuration populated with the supplied items.
func (ng *NGram[T]) Copy(items []T) (*NGram[T], error) {
	clone, err := New[T](nil,
		WithThreshold[T](ng.threshold),
		WithWarp[T](ng.warp),
		WithN[T](ng.N),
		WithPadLen[T](ng.padLen),
		WithPadChar[T](ng.padChar),
		WithKey[T](ng.key),
	)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		items = ng.Items()
	}
	clone.Update(items)
	return clone, nil
}

// Items returns a snapshot of all items stored in the index.
func (ng *NGram[T]) Items() []T {
	items := make([]T, 0, len(ng.items))
	for item := range ng.items {
		items = append(items, item)
	}
	return items
}

// Len reports the number of indexed items.
func (ng *NGram[T]) Len() int {
	return len(ng.items)
}

// Has reports whether the item has been indexed.
func (ng *NGram[T]) Has(item T) bool {
	_, ok := ng.items[item]
	return ok
}

// Add inserts an item into the index if it is not already present.
func (ng *NGram[T]) Add(item T) {
	if ng.Has(item) {
		return
	}

	key := ng.key(item)
	padded := ng.pad(key)
	grams := ng.split(padded)

	ng.items[item] = struct{}{}
	ng.length[item] = len(grams)

	for _, gram := range grams {
		bucket, ok := ng.grams[gram]
		if !ok {
			bucket = make(map[T]int)
			ng.grams[gram] = bucket
		}
		bucket[item]++
	}
}

// Update inserts every item in the slice into the index.
func (ng *NGram[T]) Update(items []T) {
	for _, item := range items {
		ng.Add(item)
	}
}

// Remove deletes the item from the index. It returns true when the item existed.
func (ng *NGram[T]) Remove(item T) bool {
	if !ng.Has(item) {
		return false
	}

	delete(ng.items, item)
	delete(ng.length, item)

	padded := ng.pad(ng.key(item))
	grams := ng.split(padded)
	seen := make(map[string]struct{})

	for _, gram := range grams {
		if _, ok := seen[gram]; ok {
			continue
		}
		seen[gram] = struct{}{}
		if bucket, ok := ng.grams[gram]; ok {
			delete(bucket, item)
			if len(bucket) == 0 {
				delete(ng.grams, gram)
			}
		}
	}

	return true
}

// Discard removes the item when present without reporting an error.
func (ng *NGram[T]) Discard(item T) {
	ng.Remove(item)
}

// Clear removes all indexed items.
func (ng *NGram[T]) Clear() {
	ng.items = make(map[T]struct{})
	ng.grams = make(map[string]map[T]int)
	ng.length = make(map[T]int)

	// Clear cache as well
	ng.cacheMu.Lock()
	ng.cache = make(map[string][]Result[T])
	ng.cacheMu.Unlock()
}

// ItemsSharingNGrams returns the number of shared n-grams for every matching item.
func (ng *NGram[T]) ItemsSharingNGrams(query string) map[T]int {
	normalized := ng.normalizeQuery(query)
	grams := ng.split(ng.pad(normalized))
	return ng.itemsSharingFromGrams(grams)
}

// Search returns the items whose similarity with the query is at least the provided threshold.
func (ng *NGram[T]) Search(query string, threshold ...float64) []Result[T] {
	if len(ng.items) == 0 {
		return nil
	}

	query = ng.normalizeQuery(query)
	useCache := len(threshold) == 0

	// Check cache first
	if useCache {
		ng.cacheMu.RLock()
		if cached, found := ng.cache[query]; found {
			ng.cacheMu.RUnlock()
			return cached
		}
		ng.cacheMu.RUnlock()
	}

	min := ng.threshold
	if len(threshold) > 0 {
		min = threshold[0]
	}

	padded := ng.pad(query)
	grams := ng.split(padded)
	shared := ng.itemsSharingFromGrams(grams)
	if len(shared) == 0 {
		if useCache {
			ng.cacheMu.Lock()
			if _, ok := ng.cache[query]; ok || len(ng.cache) < ng.cacheSize {
				ng.cache[query] = nil
			}
			ng.cacheMu.Unlock()
		}
		return nil
	}

	queryGramCount := len(grams)
	results := make([]Result[T], 0, len(shared))

	for item, samegrams := range shared {
		allgrams := queryGramCount + ng.length[item] - samegrams
		if allgrams <= 0 {
			continue
		}
		similarity := Similarity(samegrams, allgrams, ng.warp)
		if similarity >= min {
			results = append(results, Result[T]{Item: item, Similarity: similarity})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Similarity == results[j].Similarity {
			return ng.key(results[i].Item) < ng.key(results[j].Item)
		}
		return results[i].Similarity > results[j].Similarity
	})

	if useCache {
		ng.cacheMu.Lock()
		if _, ok := ng.cache[query]; ok || len(ng.cache) < ng.cacheSize {
			ng.cache[query] = results
		}
		ng.cacheMu.Unlock()
	}

	return results
}

// SearchItem searches for items similar to the provided item.
func (ng *NGram[T]) SearchItem(item T, threshold ...float64) []Result[T] {
	return ng.Search(ng.key(item), threshold...)
}

// Find returns the closest match for the query along with a success flag.
func (ng *NGram[T]) Find(query string, threshold ...float64) (T, bool) {
	results := ng.Search(query, threshold...)
	if len(results) == 0 {
		var zero T
		return zero, false
	}
	return results[0].Item, true
}

// FindItem returns the closest match for the item along with a success flag.
func (ng *NGram[T]) FindItem(item T, threshold ...float64) (T, bool) {
	return ng.Find(ng.key(item), threshold...)
}

// Compare returns the similarity between two strings using a temporary index.
func Compare(s1, s2 string, options ...Option[string]) (float64, error) {
	if s1 == s2 {
		return 1.0, nil
	}

	ng, err := New[string]([]string{s1}, options...)
	if err != nil {
		return 0, err
	}

	results := ng.Search(s2)
	if len(results) == 0 {
		return 0, nil
	}
	return results[0].Similarity, nil
}

// Similarity computes the n-gram similarity score.
func Similarity(samegrams, allgrams int, warp float64) float64 {
	if allgrams <= 0 {
		return 0
	}

	if math.Abs(warp-1.0) < 1e-9 {
		return float64(samegrams) / float64(allgrams)
	}

	diffgrams := float64(allgrams - samegrams)
	numerator := math.Pow(float64(allgrams), warp) - math.Pow(diffgrams, warp)
	denominator := math.Pow(float64(allgrams), warp)
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func (ng *NGram[T]) pad(s string) string {
	if ng.padLen == 0 {
		return s
	}
	return ng.padding + s + ng.padding
}

func (ng *NGram[T]) split(s string) []string {
	runes := []rune(s)
	if len(runes) < ng.N {
		return nil
	}

	grams := make([]string, 0, len(runes)-ng.N+1)
	for i := 0; i <= len(runes)-ng.N; i++ {
		grams = append(grams, string(runes[i:i+ng.N]))
	}
	return grams
}

func (ng *NGram[T]) itemsSharingFromGrams(grams []string) map[T]int {
	if len(grams) == 0 {
		return nil
	}

	queryCounts := make(map[string]int, len(grams))
	for _, gram := range grams {
		queryCounts[gram]++
	}

	shared := make(map[T]int, len(queryCounts))
	for gram, queryCount := range queryCounts {
		bucket, ok := ng.grams[gram]
		if !ok {
			continue
		}
		for item, itemCount := range bucket {
			if queryCount < itemCount {
				shared[item] += queryCount
				continue
			}
			shared[item] += itemCount
		}
	}

	if len(shared) == 0 {
		return nil
	}

	return shared
}

// defaultKey returns a string representation for item.
// defaultKey returns the string representation of item.
// defaultKey produces a string key for the given item.
// If the item is a string, it is returned unchanged; if it implements fmt.Stringer its String method is used; otherwise fmt.Sprint is used.
func defaultKey[T comparable](item T) string {
	switch v := any(item).(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}
