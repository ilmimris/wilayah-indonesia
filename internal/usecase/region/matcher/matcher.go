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
	thresholds   map[Level]float64
	ngramN       int
	parallelTopK int
	timeout      time.Duration
	weights      map[Level]float64
	minScore     float64
	cacheSize    int
}

// WithLevelThreshold overrides the default acceptance threshold for a level.
func WithLevelThreshold(level Level, threshold float64) Option {
	return func(cfg *matcherConfig) {
		cfg.thresholds[level] = threshold
	}
}

// WithNGramSize customises the n-gram window size.
func WithNGramSize(n int) Option {
	return func(cfg *matcherConfig) {
		if n > 0 {
			cfg.ngramN = n
		}
	}
}

// WithParallelTopK limits the number of matches retained per level during the parallel pass.
func WithParallelTopK(k int) Option {
	return func(cfg *matcherConfig) {
		if k > 0 {
			cfg.parallelTopK = k
		}
	}
}

// WithSuggestionTimeout overrides the per-query matching timeout budget.
func WithSuggestionTimeout(d time.Duration) Option {
	return func(cfg *matcherConfig) {
		if d > 0 {
			cfg.timeout = d
		}
	}
}

// WithPercolatorWeights customises how each level contributes to the percolator score.
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

// WithMinCombinedScore adjusts the minimum percolator score required to accept the parallel outcome.
func WithMinCombinedScore(score float64) Option {
	return func(cfg *matcherConfig) {
		if score > 0 {
			cfg.minScore = score
		}
	}
}

// WithCacheSize adjusts the matcher cache capacity.
func WithCacheSize(size int) Option {
	return func(cfg *matcherConfig) {
		if size > 0 {
			cfg.cacheSize = size
		}
	}
}

var defaultThresholds = map[Level]float64{
	LevelSubdistrict: 0.45,
	LevelDistrict:    0.45,
	LevelCity:        0.4,
	LevelProvince:    0.4,
}

var levelOrder = []Level{LevelProvince, LevelCity, LevelDistrict, LevelSubdistrict}

var defaultWeights = map[Level]float64{
	LevelProvince:    0.2,
	LevelCity:        0.3,
	LevelDistrict:    0.25,
	LevelSubdistrict: 0.25,
}

const defaultMinCombinedScore = 0.6

// NewMatcher constructs a matcher from the supplied facets.
func NewMatcher(facets []Facet, opts ...Option) (*Matcher, error) {
	cfg := matcherConfig{
		thresholds:   make(map[Level]float64, len(defaultThresholds)),
		ngramN:       3,
		parallelTopK: 5,
		timeout:      100 * time.Millisecond,
		weights:      make(map[Level]float64, len(defaultWeights)),
		minScore:     defaultMinCombinedScore,
		cacheSize:    1000,
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

	fragments := candidateFragments(query)
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

	suggestion, ok := runPercolator(results, m.weights, m.minCombinedScore)
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

func mergeMatches(existing, additions []Match, limit int) []Match {
	combined := make([]Match, 0, len(existing)+len(additions))
	combined = append(combined, existing...)
	combined = append(combined, additions...)
	sort.SliceStable(combined, func(i, j int) bool {
		if combined[i].Similarity == combined[j].Similarity {
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

func consistentHierarchy(codes ...string) bool {
	return regionhierarchy.IsConsistentHierarchy(codes...)
}

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

func candidateFragments(query string) []string {
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
		for size := 2; size <= 2; size++ { // Reduced from 3 to 2
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

	// Limit number of fragments for performance (reduced from 10 to 5)
	if len(fragments) > 5 {
		fragments = fragments[:5]
	}

	return fragments
}

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
