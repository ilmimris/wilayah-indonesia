package regionmatcher

import (
	"sort"
	"strings"
	"unicode"
	"sync"

	"github.com/ilmimris/wilayah-indonesia/pkg/utils/ngram"
)

// Level identifies the administrative layer represented by an index entry.
type Level string

const (
	LevelSubdistrict Level = "subdistrict"
	LevelDistrict    Level = "district"
	LevelCity        Level = "city"
	LevelProvince    Level = "province"
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
}

// Suggestion aggregates inferred matches for every level.
type Suggestion struct {
	Subdistrict *Match
	District    *Match
	City        *Match
	Province    *Match
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
	cache   map[string]Suggestion
	cacheMu sync.RWMutex
	cacheSize int
}

type indexEntry struct {
	Level    Level
	Value    string
	Key      string
	RegionID string
	District string
	City     string
	Province string
}

type matcherOption func(*matcherConfig)

type matcherConfig struct {
	thresholds map[Level]float64
	ngramN     int
}

// WithLevelThreshold overrides the default acceptance threshold for a level.
func WithLevelThreshold(level Level, threshold float64) matcherOption {
	return func(cfg *matcherConfig) {
		cfg.thresholds[level] = threshold
	}
}

// WithNGramSize customises the n-gram window size.
func WithNGramSize(n int) matcherOption {
	return func(cfg *matcherConfig) {
		if n > 0 {
			cfg.ngramN = n
		}
	}
}

var defaultThresholds = map[Level]float64{
	LevelSubdistrict: 0.45,
	LevelDistrict:    0.45,
	LevelCity:        0.4,
	LevelProvince:    0.4,
}

// NewMatcher constructs a matcher from the supplied facets.
func NewMatcher(facets []Facet, opts ...matcherOption) (*Matcher, error) {
	cfg := matcherConfig{
		thresholds: make(map[Level]float64, len(defaultThresholds)),
		ngramN:     3,
	}
	for level, value := range defaultThresholds {
		cfg.thresholds[level] = value
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	byLevel := map[Level][]indexEntry{}

	for _, facet := range facets {
		addEntries := func(level Level, value string, alts []string) {
			seen := map[string]struct{}{}
			for _, alias := range append([]string{value}, alts...) {
				normalized := normalize(alias)
				if normalized == "" {
					continue
				}
				key := normalized + "|" + string(level) + "|" + facet.City + "|" + facet.District + "|" + facet.Province
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				entry := indexEntry{
					Level:    level,
					Value:    value,
					Key:      normalized,
					RegionID: facet.RegionID,
					District: facet.District,
					City:     facet.City,
					Province: facet.Province,
				}
				byLevel[level] = append(byLevel[level], entry)
			}
		}

		if facet.Subdistrict != "" {
			addEntries(LevelSubdistrict, facet.Subdistrict, facet.SubdistrictAlts)
		}
		if facet.District != "" {
			addEntries(LevelDistrict, facet.District, facet.DistrictAlts)
		}
		if facet.City != "" {
			addEntries(LevelCity, facet.City, facet.CityAlts)
		}
		if facet.Province != "" {
			addEntries(LevelProvince, facet.Province, facet.ProvinceAlts)
		}
	}

	indexes := make(map[Level]*ngram.NGram[indexEntry], len(byLevel))
	for level, items := range byLevel {
		if len(items) == 0 {
			continue
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

	return &Matcher{
		indexes:    indexes,
		thresholds: thresholds,
		cache:      make(map[string]Suggestion),
		cacheSize:  1000, // Default cache size
	}, nil
}

// Suggest generates matches for each administrative level using the supplied query.
func (m *Matcher) Suggest(query string) Suggestion {
	if len(m.indexes) == 0 {
		return Suggestion{}
	}

	// Check cache first
	m.cacheMu.RLock()
	if cached, found := m.cache[query]; found {
		m.cacheMu.RUnlock()
		return cached
	}
	m.cacheMu.RUnlock()

	candidates := candidateFragments(query)
	suggestion := Suggestion{}
	orderedLevels := []Level{LevelProvince, LevelCity, LevelDistrict, LevelSubdistrict}

	for _, level := range orderedLevels {
		idx := m.indexes[level]
		if idx == nil {
			continue
		}
		threshold := m.thresholds[level]
		if threshold <= 0 {
			threshold = 0.1
		}

		var best *Match
		for _, fragment := range candidates {
			// Limit the number of candidates processed for performance
			if len(fragment) < 2 {
				continue
			}
			
			results := idx.Search(fragment)
			if len(results) == 0 {
				continue
			}
			top := results[0]
			if top.Similarity < threshold {
				continue
			}
			if best == nil || top.Similarity > best.Similarity {
				entry := top.Item
				clone := Match{
					Level:      entry.Level,
					Name:       entry.Value,
					Similarity: top.Similarity,
					RegionID:   entry.RegionID,
					District:   entry.District,
					City:       entry.City,
					Province:   entry.Province,
				}
				best = &clone
			}
		}

		if best != nil {
			suggestion.matchSetter(level, best)
		}
	}

	harmonizeSuggestion(&suggestion)
	
	// Cache result
	m.cacheMu.Lock()
	if len(m.cache) < m.cacheSize {
		m.cache[query] = suggestion
	}
	m.cacheMu.Unlock()

	return suggestion
}

func harmonizeSuggestion(s *Suggestion) {
	if s.City != nil && s.Province != nil && !strings.EqualFold(s.City.Province, s.Province.Name) {
		if s.City.Similarity >= s.Province.Similarity {
			s.Province = &Match{
				Level:      LevelProvince,
				Name:       s.City.Province,
				Similarity: s.City.Similarity,
			}
		}
	}

	if s.District != nil && s.City != nil && !strings.EqualFold(s.District.City, s.City.Name) {
		if s.District.Similarity < s.City.Similarity {
			s.District = nil
		}
	}

	if s.Subdistrict != nil && s.District != nil && !strings.EqualFold(s.Subdistrict.District, s.District.Name) {
		if s.Subdistrict.Similarity < s.District.Similarity {
			s.Subdistrict = nil
		}
	}
}

func candidateFragments(query string) []string {
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

	separators := []string{",", "|", ";", "-", "/", "\n", "	"}
	for _, sep := range separators {
		parts := strings.Split(query, sep)
		for _, part := range parts {
			add(part)
		}
	}

	words := strings.Fields(normalize(query))
	if len(words) > 0 {
		for _, word := range words {
			add(word)
		}
		// Limit n-gram combinations for performance
		for size := 2; size <= 2; size++ { // Reduced from 3 to 2
			if len(words) < size {
				break
			}
			for i := 0; i <= len(words)-size; i++ {
				add(strings.Join(words[i:i+size], " "))
			}
		}
	}

	fragments := make([]string, 0, len(seen))
	for fragment := range seen {
		fragments = append(fragments, fragment)
	}
	sort.Slice(fragments, func(i, j int) bool { return len(fragments[i]) > len(fragments[j]) })
	
	// Limit number of fragments for performance
	if len(fragments) > 10 {
		fragments = fragments[:10]
	}
	
	return fragments
}

func normalize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
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