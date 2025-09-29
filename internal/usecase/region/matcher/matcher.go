package regionmatcher

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"
	"github.com/ilmimris/wilayah-indonesia/pkg/utils/ngram"
)

// Level identifies the administrative layer represented by an index entry.
type Level = regionhierarchy.Level

const (
	LevelSubdistrict Level = regionhierarchy.LevelSubdistrict
	LevelDistrict    Level = regionhierarchy.LevelDistrict
	LevelCity        Level = regionhierarchy.LevelCity
	LevelProvince    Level = regionhierarchy.LevelProvince
)

// Facet captures hierarchical naming information used to build n-gram indices.
type Facet struct {
	RegionID        string
	Subdistrict     string
	SubdistrictAlts []string
	District        string
	DistrictAlts    []string
	City            string
	CityAlts        []string
	Province        string
	ProvinceAlts    []string
}

// Match describes a suggested region candidate.
type Match struct {
	Level      Level
	Name       string
	Similarity float64
	RegionID   string
	District   string
	City       string
	Province   string
	Fragment   string
}

// Suggestion aggregates inferred matches for every level.
type Suggestion struct {
	Subdistrict *Match
	District    *Match
	City        *Match
	Province    *Match
	Score       float64
	Strategy    string
}

// matchSetter updates the suggestion for the provided level.
func (s *Suggestion) matchSetter(level Level, match *Match) {
	switch level {
	case LevelSubdistrict:
		s.Subdistrict = match
	case LevelDistrict:
		s.District = match
	case LevelCity:
		s.City = match
	case LevelProvince:
		s.Province = match
	}
}

// Matcher builds n-gram indices for administrative names and surfaces suggestions.
type Matcher struct {
	indexes    map[Level]*ngram.NGram[indexEntry]
	thresholds map[Level]float64

	// Add cache for frequently searched queries
	cache     map[string]Suggestion
	cacheMu   sync.RWMutex
	cacheSize int

	parallelTopK     int
	timeout          time.Duration
	weights          map[Level]float64
	minCombinedScore float64
	prefixScoreBoost float64
	matchFillBonus   float64
	wordComboSize    int
}

type indexEntry struct {
	Level       Level
	Value       string
	Key         string
	RegionID    string
	District    string
	City        string
	Province    string
	Subdistrict string
}

type Option func(*matcherConfig)

type matcherConfig struct {
	thresholds       map[Level]float64
	ngramN           int
	parallelTopK     int
	timeout          time.Duration
	weights          map[Level]float64
	minScore         float64
	cacheSize        int
	prefixScoreBoost float64
	matchFillBonus   float64
	wordComboSize    int
}

// WithLevelThreshold returns an Option that sets the minimum similarity threshold for the given Level.
// WithLevelThreshold sets the similarity threshold used to accept matches for the given level.
// The threshold is a similarity score (recommended range 0.0–1.0); matches for that level with
// similarity below this value will be rejected when scoring candidates.
func WithLevelThreshold(level Level, threshold float64) Option {
	return func(cfg *matcherConfig) {
		cfg.thresholds[level] = threshold
	}
}

// WithNGramSize sets the n-gram window size used to build per-level indexes.
// A value greater than 0 replaces the default n-gram size in the matcher configuration;
// WithNGramSize returns an Option that sets the n-gram window size used when building per-level indexes.
// WithNGramSize sets the n-gram size used by the matcher when indexing and searching.
// If n is greater than zero, the configuration's ngramN is updated; non-positive values are ignored.
func WithNGramSize(n int) Option {
	return func(cfg *matcherConfig) {
		if n > 0 {
			cfg.ngramN = n
		}
	}
}

// WithParallelTopK returns an Option that sets the maximum number of matches retained per level
// during the parallel percolation pass. If k is not greater than zero, the existing configuration
// WithParallelTopK sets the per-level top-k candidate limit used during parallel searches.
// WithParallelTopK sets the number of top results to request per parallel per-level fragment search.
// If k is not greater than zero the configuration is left unchanged.
func WithParallelTopK(k int) Option {
	return func(cfg *matcherConfig) {
		if k > 0 {
			cfg.parallelTopK = k
		}
	}
}

// WithSuggestionTimeout returns an Option that sets the per-query matching timeout budget.
// WithSuggestionTimeout sets the per-query suggestion timeout in the matcher configuration.
// WithSuggestionTimeout returns an Option that sets the per-suggestion timeout used by the Matcher.
// If d is greater than zero the timeout is set to d; non-positive durations leave the timeout unchanged.
func WithSuggestionTimeout(d time.Duration) Option {
	return func(cfg *matcherConfig) {
		if d > 0 {
			cfg.timeout = d
		}
	}
}

// WithPercolatorWeights returns an Option that applies per-level percolator weights to the matcher configuration.
// The provided map's keys are Levels and values are non-negative weights. An empty map is ignored and any
// WithPercolatorWeights returns an Option that sets percolator weights from the provided map.
//
// The provided map's keys are levels and values are weights. Negative weights are ignored;
// non-negative weights overwrite or populate entries in the matcher's configuration. If the
// matcherConfig's weights map is nil, it will be allocated. An empty input map is a no-op.
func WithPercolatorWeights(weights map[Level]float64) Option {
	return func(cfg *matcherConfig) {
		if len(weights) == 0 {
			return
		}
		if cfg.weights == nil {
			cfg.weights = make(map[Level]float64, len(weights))
		}
		for level, weight := range weights {
			if weight < 0 {
				continue
			}
			cfg.weights[level] = weight
		}
	}
}

// WithMinCombinedScore returns an Option that sets the minimum combined percolator score used when evaluating suggestions.
// WithMinCombinedScore sets the minimum combined score threshold used by the matcher.
// If the provided score is less than or equal to zero the option is ignored.
func WithMinCombinedScore(score float64) Option {
	return func(cfg *matcherConfig) {
		if score > 0 {
			cfg.minScore = score
		}
	}
}

// WithPrefixScoreBoost returns an Option that sets the score boost for shared prefixes.
// This boost is a small bonus added when adjacent region matches (e.g., city and district)
// WithPrefixScoreBoost sets the prefix score boost used when matches share a common region code prefix.
// A positive boost value is applied; non-positive values are ignored.
func WithPrefixScoreBoost(boost float64) Option {
	return func(cfg *matcherConfig) {
		if boost > 0 {
			cfg.prefixScoreBoost = boost
		}
	}
}

// WithMatchFillBonus returns an Option that sets the incremental score bonus
// for each filled (non-empty) level in a potential match combination. This bonus
// WithMatchFillBonus sets a positive bonus added to the combined suggestion score to reward matches
// that cover more hierarchical levels. If `bonus` is less than or equal to zero the configuration
// is left unchanged.
func WithMatchFillBonus(bonus float64) Option {
	return func(cfg *matcherConfig) {
		if bonus > 0 {
			cfg.matchFillBonus = bonus
		}
	}
}

// WithCacheSize sets the matcher's cache capacity to the provided size when size is greater than zero.
// WithCacheSize sets the matcher cache size to the provided value.
// If size is zero or negative, the existing cache capacity is left unchanged.
// It returns an Option that applies this configuration.
func WithCacheSize(size int) Option {
	return func(cfg *matcherConfig) {
		if size > 0 {
			cfg.cacheSize = size
		}
	}
}

// WithWordComboSize returns an Option that sets the word combination size used in query fragmentation.
// WithWordComboSize returns an Option that sets the matcher's wordComboSize to the given size.
// Non-positive values are ignored.
func WithWordComboSize(size int) Option {
	return func(cfg *matcherConfig) {
		if size > 0 {
			cfg.wordComboSize = size
		}
	}
}

var defaultThresholds = map[Level]float64{
	LevelSubdistrict: 0.45,
	LevelDistrict:    0.58,
	LevelCity:        0.5,
	LevelProvince:    0.4,
}

var levelOrder = []Level{LevelProvince, LevelCity, LevelDistrict, LevelSubdistrict}

var defaultWeights = map[Level]float64{
	LevelProvince:    0.2,
	LevelCity:        0.3,
	LevelDistrict:    0.25,
	LevelSubdistrict: 0.25,
}

const defaultMinCombinedScore = 0.8
const defaultPrefixScoreBoost = 0.01
const defaultMatchFillBonus = 0.03

// NewMatcher constructs a Matcher from the supplied facets and functional options.
// It builds per-level n-gram indexes from facet names and aliases (province, city,
// district, subdistrict), normalizes and deduplicates index entries, and applies
// configuration provided via options (per-level thresholds, n‑gram size,
// parallel top-K, per-query timeout, percolator weights, minimum combined score,
// and cache size). It returns a configured *Matcher or an error if any region
// NewMatcher constructs a Matcher from the provided region facets and options.
// It builds per-level n-gram indexes from facet names and aliases, applies configuration
// options (thresholds, n-gram size, parallelism, timeouts, weights, cache size) and
// initializes internal thresholds, weights and the suggestion cache.
// It returns a configured *Matcher or an error if any region ID fails to parse or an
// NewMatcher constructs a Matcher populated from the provided facets and configured by the supplied functional options.
// It builds per-level n-gram indexes (province, city, district, subdistrict) from facet names and aliases, applies defaults
// and option overrides (thresholds, n-gram size, parallelism, timeouts, weights, cache size, scoring tweaks, and fragmentation size),
// and initializes internal caches and scoring parameters.
// The function normalizes and deduplicates entries per (normalized name + region ID) and skips facets with empty RegionID.
// It returns an error if a facet's RegionID cannot be parsed or if an underlying n-gram index cannot be created.
func NewMatcher(facets []Facet, opts ...Option) (*Matcher, error) {
	cfg := matcherConfig{
		thresholds:       make(map[Level]float64, len(defaultThresholds)),
		ngramN:           3,
		parallelTopK:     5,
		timeout:          100 * time.Millisecond,
		weights:          make(map[Level]float64, len(defaultWeights)),
		minScore:         defaultMinCombinedScore,
		cacheSize:        1000,
		prefixScoreBoost: defaultPrefixScoreBoost,
		matchFillBonus:   defaultMatchFillBonus,
		wordComboSize:    3,
	}
	for level, value := range defaultThresholds {
		cfg.thresholds[level] = value
	}
	for level, weight := range defaultWeights {
		cfg.weights[level] = weight
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.parallelTopK <= 0 {
		cfg.parallelTopK = 5
	}
	if cfg.timeout <= 0 {
		cfg.timeout = 100 * time.Millisecond
	}
	if cfg.minScore <= 0 {
		cfg.minScore = defaultMinCombinedScore
	}
	for level, weight := range defaultWeights {
		if _, ok := cfg.weights[level]; !ok {
			cfg.weights[level] = weight
		}
	}
	if cfg.cacheSize <= 0 {
		cfg.cacheSize = 1000
	}
	if cfg.wordComboSize <= 0 {
		cfg.wordComboSize = 3
	}

	byLevel := map[Level]map[string]indexEntry{
		LevelProvince:    {},
		LevelCity:        {},
		LevelDistrict:    {},
		LevelSubdistrict: {},
	}

	for _, facet := range facets {
		if facet.RegionID == "" {
			continue
		}
		segs, err := regionhierarchy.ParseRegionID(facet.RegionID)
		if err != nil {
			return nil, err
		}
		levelIDs := map[Level]string{
			LevelProvince:    segs.Province,
			LevelCity:        segs.City,
			LevelDistrict:    segs.District,
			LevelSubdistrict: segs.Subdistrict,
		}

		baseEntry := indexEntry{
			District:    facet.District,
			City:        facet.City,
			Province:    facet.Province,
			Subdistrict: facet.Subdistrict,
		}

		addEntries := func(level Level, value string, alts []string) {
			regionID := levelIDs[level]
			if value == "" || regionID == "" {
				return
			}
			seen := make(map[string]struct{})
			for _, alias := range append([]string{value}, alts...) {
				normalized := normalize(alias)
				if normalized == "" {
					continue
				}
				key := normalized + "|" + regionID
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}

				entry := baseEntry
				entry.Level = level
				entry.RegionID = regionID
				entry.Key = normalized
				entry.Value = value
				switch level {
				case LevelProvince:
					entry.Province = value
					entry.City = ""
					entry.District = ""
				case LevelCity:
					entry.City = value
					entry.District = ""
				case LevelDistrict:
					entry.District = value
				case LevelSubdistrict:
					entry.Subdistrict = value
				}

				entries := byLevel[level]
				if _, exists := entries[key]; exists {
					continue
				}
				entries[key] = entry
			}
		}

		addEntries(LevelSubdistrict, facet.Subdistrict, facet.SubdistrictAlts)
		addEntries(LevelDistrict, facet.District, facet.DistrictAlts)
		addEntries(LevelCity, facet.City, facet.CityAlts)
		addEntries(LevelProvince, facet.Province, facet.ProvinceAlts)
	}

	indexes := make(map[Level]*ngram.NGram[indexEntry], len(byLevel))
	for level, itemsMap := range byLevel {
		if len(itemsMap) == 0 {
			continue
		}
		items := make([]indexEntry, 0, len(itemsMap))
		for _, entry := range itemsMap {
			items = append(items, entry)
		}
		idx, err := ngram.New(items,
			ngram.WithKey(func(entry indexEntry) string { return entry.Key }),
			ngram.WithThreshold[indexEntry](0),
			ngram.WithN[indexEntry](cfg.ngramN),
		)
		if err != nil {
			return nil, err
		}
		indexes[level] = idx
	}

	thresholds := make(map[Level]float64, len(cfg.thresholds))
	for level, threshold := range cfg.thresholds {
		thresholds[level] = threshold
	}

	weights := make(map[Level]float64, len(cfg.weights))
	for level, weight := range cfg.weights {
		weights[level] = weight
	}

	return &Matcher{
		indexes:          indexes,
		thresholds:       thresholds,
		cache:            make(map[string]Suggestion),
		cacheSize:        cfg.cacheSize,
		parallelTopK:     cfg.parallelTopK,
		timeout:          cfg.timeout,
		weights:          weights,
		minCombinedScore: cfg.minScore,
		prefixScoreBoost: cfg.prefixScoreBoost,
		matchFillBonus:   cfg.matchFillBonus,
		wordComboSize:    cfg.wordComboSize,
	}, nil
}

// Suggest generates matches for each administrative level using the supplied query.
func (m *Matcher) Suggest(query string) Suggestion {
	if len(m.indexes) == 0 {
		return Suggestion{}
	}

	m.cacheMu.RLock()
	if cached, found := m.cache[query]; found {
		m.cacheMu.RUnlock()
		return cached
	}
	m.cacheMu.RUnlock()

	if len(query) > 100 {
		return Suggestion{}
	}

	fragments := candidateFragments(query, m.wordComboSize)
	if len(fragments) == 0 {
		return Suggestion{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	suggestion, ok := m.percolateMatches(ctx, fragments)
	if !ok {
		suggestion = m.sequentialFallback(fragments)
	}

	m.cacheMu.Lock()
	if len(m.cache) < m.cacheSize {
		m.cache[query] = suggestion
	}
	m.cacheMu.Unlock()

	return suggestion
}

// ThresholdForLevel returns the similarity threshold configured for the given level.
func (m *Matcher) ThresholdForLevel(level Level) float64 {
	if m == nil {
		return 0
	}
	if threshold, ok := m.thresholds[level]; ok {
		return threshold
	}
	return 0
}

func (m *Matcher) percolateMatches(ctx context.Context, fragments []string) (Suggestion, bool) {
	results := make(map[Level][]Match, len(levelOrder))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, fragment := range fragments {
		if len(fragment) < 2 {
			continue
		}
		for _, level := range levelOrder {
			idx := m.indexes[level]
			if idx == nil {
				continue
			}
			wg.Add(1)
			go func(level Level, fragment string) {
				defer wg.Done()
				matches := m.searchLevel(level, fragment, m.parallelTopK)
				if len(matches) == 0 {
					return
				}
				mu.Lock()
				results[level] = mergeMatches(results[level], matches, m.parallelTopK)
				mu.Unlock()
			}(level, fragment)
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return Suggestion{}, false
	case <-done:
	}

	if len(results) == 0 {
		return Suggestion{}, false
	}

	suggestion, ok := runPercolator(results, m.weights, m.minCombinedScore, m.prefixScoreBoost, m.matchFillBonus)
	if !ok {
		return Suggestion{}, false
	}
	suggestion.Strategy = "percolator"
	return suggestion, true
}

func (m *Matcher) searchLevel(level Level, fragment string, limit int) []Match {
	idx := m.indexes[level]
	if idx == nil {
		return nil
	}
	if limit <= 0 {
		limit = m.parallelTopK
	}
	results := idx.Search(fragment)
	threshold := m.thresholds[level]
	matches := make([]Match, 0, limit)
	for _, res := range results {
		if res.Similarity < threshold {
			continue
		}
		entry := res.Item
		match := Match{
			Level:      level,
			Name:       entry.Value,
			Similarity: res.Similarity,
			RegionID:   entry.RegionID,
			District:   entry.District,
			City:       entry.City,
			Province:   entry.Province,
			Fragment:   fragment,
		}
		if entry.Subdistrict != "" && level == LevelSubdistrict {
			match.Name = entry.Subdistrict
		}
		matches = append(matches, match)
		if len(matches) >= limit {
			break
		}
	}
	return matches
}

// mergeMatches combines existing and addition slices of Match, sorts them by
// similarity descending (tie-broken by RegionID ascending), deduplicates entries
// by RegionID and case-insensitive Name, and returns up to limit results.
// mergeMatches merges two slices of Match, orders the combined results by
// descending Similarity (tie-broken by ascending RegionID), removes duplicates
// (a duplicate has the same RegionID and the same name ignoring case), and
// returns up to limit unique matches. If limit is zero or negative, all unique
// mergeMatches merges existing and additions into a single slice of unique matches.
// It sorts candidates by Similarity descending (ties broken by RegionID ascending),
// removes duplicates where duplicates are defined by the combination of RegionID and
// the lowercased Name (the first occurrence is kept), and returns up to limit entries.
// If limit <= 0 all unique matches are returned.
func mergeMatches(existing, additions []Match, limit int) []Match {
	combined := make([]Match, 0, len(existing)+len(additions))
	combined = append(combined, existing...)
	combined = append(combined, additions...)
	sort.SliceStable(combined, func(i, j int) bool {
		if combined[i].Similarity == combined[j].Similarity {
			pi := provinceHintScore(combined[i])
			pj := provinceHintScore(combined[j])
			if pi != pj {
				return pi > pj
			}
			return combined[i].RegionID < combined[j].RegionID
		}
		return combined[i].Similarity > combined[j].Similarity
	})

	seen := make(map[string]struct{}, len(combined))
	uniq := make([]Match, 0, limit)
	for _, match := range combined {
		key := match.RegionID + "|" + strings.ToLower(match.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		uniq = append(uniq, match)
		if limit > 0 && len(uniq) >= limit {
			break
		}
	}
	return uniq
}

func provinceHintScore(match Match) int {
	if match.Level != LevelDistrict {
		return 0
	}
	if match.Fragment == "" || match.Province == "" {
		return 0
	}
	province := normalize(match.Province)
	if province == "" {
		return 0
	}
	if strings.Contains(match.Fragment, province) {
		return 1
	}
	return 0
}

func (m *Matcher) sequentialFallback(fragments []string) Suggestion {
	suggestion := Suggestion{}
	processed := 0
	for _, fragment := range fragments {
		if len(fragment) < 2 {
			continue
		}
		processed++
		if processed > 5 {
			break
		}
		for _, level := range levelOrder {
			matches := m.searchLevel(level, fragment, 1)
			if len(matches) == 0 {
				continue
			}
			match := matches[0]
			current := suggestion.MatchFor(level)
			if current == nil || match.Similarity > current.Similarity {
				copyMatch := match
				suggestion.matchSetter(level, &copyMatch)
			}
		}
		if suggestion.Subdistrict != nil && suggestion.District != nil && suggestion.City != nil && suggestion.Province != nil {
			if suggestion.Subdistrict.Similarity > 0.9 && suggestion.District.Similarity > 0.9 && suggestion.City.Similarity > 0.9 && suggestion.Province.Similarity > 0.9 {
				break
			}
		}
	}
	harmonizeSuggestion(&suggestion)
	suggestion.Score = scoreSuggestion(suggestion)
	suggestion.Strategy = "fallback"
	return suggestion
}

// harmonizeSuggestion updates a Suggestion in place to ensure hierarchical consistency
// among Province, City, District, and Subdistrict matches.
// It resolves inconsistencies by preferring matches with higher similarity and clears
// less likely conflicting matches; it also attempts to derive missing higher-level
// matches (Province from City, City from District, District from Subdistrict) using
// inconsistent with a stronger parent.
func harmonizeSuggestion(s *Suggestion) {
	if s.City != nil && s.Province != nil && !consistentHierarchy(s.Province.RegionID, s.City.RegionID) {
		if s.City.Similarity >= s.Province.Similarity {
			if provinceID, err := regionhierarchy.CodeAtLevel(s.City.RegionID, LevelProvince); err == nil && provinceID != "" {
				s.Province = &Match{
					Level:      LevelProvince,
					Name:       s.City.Province,
					Province:   s.City.Province,
					RegionID:   provinceID,
					Similarity: s.City.Similarity,
				}
			}
		} else {
			s.City = nil
		}
	}
	if s.City != nil && s.Province == nil {
		if provinceID, err := regionhierarchy.CodeAtLevel(s.City.RegionID, LevelProvince); err == nil && provinceID != "" && s.City.Province != "" {
			s.Province = &Match{
				Level:      LevelProvince,
				Name:       s.City.Province,
				Province:   s.City.Province,
				RegionID:   provinceID,
				Similarity: s.City.Similarity,
			}
		}
	}

	if s.District != nil && s.City != nil && !consistentHierarchy(s.City.RegionID, s.District.RegionID) {
		if s.District.Similarity < s.City.Similarity {
			s.District = nil
		}
	}
	if s.District != nil && s.City == nil {
		if cityID, err := regionhierarchy.CodeAtLevel(s.District.RegionID, LevelCity); err == nil && cityID != "" && s.District.City != "" {
			s.City = &Match{
				Level:      LevelCity,
				Name:       s.District.City,
				City:       s.District.City,
				Province:   s.District.Province,
				RegionID:   cityID,
				Similarity: s.District.Similarity,
			}
		}
	}

	if s.Subdistrict != nil && s.District != nil && !consistentHierarchy(s.District.RegionID, s.Subdistrict.RegionID) {
		if s.Subdistrict.Similarity < s.District.Similarity {
			s.Subdistrict = nil
		}
	}
	if s.Subdistrict != nil && s.District == nil {
		if districtID, err := regionhierarchy.CodeAtLevel(s.Subdistrict.RegionID, LevelDistrict); err == nil && districtID != "" && s.Subdistrict.District != "" {
			s.District = &Match{
				Level:      LevelDistrict,
				Name:       s.Subdistrict.District,
				District:   s.Subdistrict.District,
				City:       s.Subdistrict.City,
				Province:   s.Subdistrict.Province,
				RegionID:   districtID,
				Similarity: s.Subdistrict.Similarity,
			}
		}
	}
}

// consistentHierarchy reports whether the provided region codes form a consistent regional hierarchy.
// consistentHierarchy reports whether the provided region codes form a consistent
// hierarchical chain from province through city, district, to subdistrict.
// Codes should be supplied in that order (province, city, district, subdistrict);
// / and false otherwise.
func consistentHierarchy(codes ...string) bool {
	return regionhierarchy.IsConsistentHierarchy(codes...)
}

// scoreSuggestion computes the average Similarity of the non-nil matches contained in s.
// It returns the mean similarity across Province, City, District, and Subdistrict matches,
// scoreSuggestion computes the average Similarity of non-nil matches in s
// (Province, City, District, Subdistrict). It returns 0 if there are no
// scoreSuggestion computes the average Similarity of the non-nil matches contained in s.
// If s has no matches, it returns 0.
func scoreSuggestion(s Suggestion) float64 {
	total := 0.0
	count := 0.0
	for _, match := range []*Match{s.Province, s.City, s.District, s.Subdistrict} {
		if match == nil {
			continue
		}
		total += match.Similarity
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

// candidateFragments produces a short, ordered list of normalized query fragments suitable for n-gram matching.
//
// For queries longer than 100 characters, it extracts words from the normalized prefix (first 50 characters)
// and returns up to the first 5 words. For shorter queries it adds the normalized whole query, splits by a set
// of common separators to add parts, and also adds individual words and 2-word combinations. The function
// deduplicates fragments, enforces caps to avoid explosion (limits on parts, words and combinations), and
// candidateFragments returns up to ten normalized query fragments used for matching.
// For queries longer than 100 characters it returns up to the first five words from
// the normalized prefix (first 50 characters). For shorter queries it builds a
// deduplicated set of fragments including the normalized whole query, parts split
// by common separators, individual words, and contiguous word combinations up to
// wordComboSize. Construction is capped to avoid explosion (internal cap ≈ 20
// unique fragments), and the final fragments are sorted by length and truncated
// to at most ten entries.
func candidateFragments(query string, wordComboSize int) []string {
	// For very long queries, extract only the most relevant parts
	if len(query) > 100 {
		// Extract only words and numbers, limit to first 50 characters
		words := strings.Fields(normalize(query[:50]))
		if len(words) > 5 {
			words = words[:5] // Take only first 5 words
		}
		return words
	}

	seen := make(map[string]struct{})
	add := func(text string) {
		normalized := normalize(text)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
	}

	add(query)

	separators := []string{",", "|", ";", "-", "/", "\n"}
	for _, sep := range separators {
		parts := strings.Split(query, sep)
		for _, part := range parts {
			add(part)
			// Limit number of parts to prevent explosion
			if len(seen) > 20 {
				break
			}
		}
		if len(seen) > 20 {
			break
		}
	}

	words := strings.Fields(normalize(query))
	if len(words) > 0 {
		for _, word := range words {
			add(word)
			// Limit number of words to prevent explosion
			if len(seen) > 20 {
				break
			}
		}
		// Limit n-gram combinations for performance
		for size := 2; size <= wordComboSize; size++ {
			if len(words) < size {
				break
			}
			for i := 0; i <= len(words)-size; i++ {
				add(strings.Join(words[i:i+size], " "))
				// Limit number of combinations to prevent explosion
				if len(seen) > 20 {
					break
				}
			}
			if len(seen) > 20 {
				break
			}
		}
	}

	fragments := make([]string, 0, len(seen))
	for fragment := range seen {
		fragments = append(fragments, fragment)
	}
	sort.Slice(fragments, func(i, j int) bool { return len(fragments[i]) > len(fragments[j]) })

	// Limit number of fragments for performance while keeping broad coverage.
	if len(fragments) > 10 {
		fragments = fragments[:10]
	}

	return fragments
}

// normalize produces a lowercase, space-separated token string suitable for indexing region names.
//
// It trims surrounding space, limits processing to the first 100 characters, treats any non-letter and
// non-digit characters as separators, collapses consecutive whitespace, and returns at most 10 tokens
// normalize produces a lowercase, space-separated token string suitable for indexing.
// normalize returns a cleaned, lowercased token sequence derived from value.
// It trims surrounding space, truncates to the first 100 characters, treats letters and digits as token characters and any other rune as a separator, collapses consecutive separators and whitespace to single spaces, limits the result to at most 10 space-separated tokens, and returns an empty string if no valid tokens remain.
func normalize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	// For very long strings, only process the first 100 characters
	if len(value) > 100 {
		value = value[:100]
	}

	var builder strings.Builder
	builder.Grow(len(value))
	lastWasSpace := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastWasSpace = false
			continue
		}
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				builder.WriteRune(' ')
				lastWasSpace = true
			}
			continue
		}
		// Treat punctuation as a separator.
		if !lastWasSpace {
			builder.WriteRune(' ')
			lastWasSpace = true
		}
	}

	normalized := strings.TrimSpace(builder.String())
	if normalized == "" {
		return ""
	}

	tokens := strings.Fields(normalized)
	// Limit number of tokens to prevent explosion
	if len(tokens) > 10 {
		tokens = tokens[:10]
	}
	return strings.Join(tokens, " ")
}

func (s Suggestion) MatchFor(level Level) *Match {
	switch level {
	case LevelSubdistrict:
		return s.Subdistrict
	case LevelDistrict:
		return s.District
	case LevelCity:
		return s.City
	case LevelProvince:
		return s.Province
	default:
		return nil
	}
}
