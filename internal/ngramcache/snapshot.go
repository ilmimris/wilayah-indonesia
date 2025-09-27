package ngramcache

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ilmimris/wilayah-indonesia/internal/usecase/region/matcher"
	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"
)

// Metadata captures provenance for generated n-gram snapshots.
type Metadata struct {
	SourcePath   string                        `json:"source_path"`
	DatasetHash  string                        `json:"dataset_hash"`
	GeneratedAt  time.Time                     `json:"generated_at"`
	RecordCounts map[regionhierarchy.Level]int `json:"record_counts"`
}

// Snapshot bundles matcher facets with metadata for replaying cached indices.
type Snapshot struct {
	Metadata Metadata              `json:"metadata"`
	Facets   []regionmatcher.Facet `json:"facets"`
}

var entryRegex = regexp.MustCompile(`\('([^']+)',\s*'((?:[^']|'')*)'\)`) // matches SQL tuple rows.

type parsedRecord struct {
	name string
	seg  regionhierarchy.Segments
}

// BuildSnapshotFromWilayah parses a wilayah SQL dump and materialises n-gram facets.
// 
// BuildSnapshotFromWilayah reads the SQL file at the given path, computes a SHA-256
// hash of its contents, extracts region records, and produces a Snapshot that
// includes Metadata (source path, dataset hash, generation time, per-level record
// counts) and a sorted list of regionmatcher.Facet entries.
// 
// The function returns an error if the file cannot be read, if any region code is
// invalid, if referential integrity is broken when resolving parent regions, or if
// the input cannot be scanned.
func BuildSnapshotFromWilayah(path string) (Snapshot, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read wilayah SQL: %w", err)
	}

	hash := sha256.Sum256(contents)
	datasetHash := hex.EncodeToString(hash[:])

	scanner := bufio.NewScanner(bytes.NewReader(contents))
	scanner.Buffer(make([]byte, 0, 1024), 4*1024*1024) // handle long INSERT lines

	records := make(map[string]parsedRecord)
	counts := map[regionhierarchy.Level]int{
		regionhierarchy.LevelProvince:    0,
		regionhierarchy.LevelCity:        0,
		regionhierarchy.LevelDistrict:    0,
		regionhierarchy.LevelSubdistrict: 0,
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		matches := entryRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			code := match[1]
			name := strings.ReplaceAll(match[2], "''", "'")
			segs, err := regionhierarchy.ParseRegionID(code)
			if err != nil {
				return Snapshot{}, fmt.Errorf("invalid region code %q: %w", code, err)
			}
			records[code] = parsedRecord{name: name, seg: segs}
			counts[segs.Level()]++
		}
	}
	if err := scanner.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("scan wilayah SQL: %w", err)
	}

	facets, err := buildFacets(records)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{
		Metadata: Metadata{
			SourcePath:   path,
			DatasetHash:  datasetHash,
			GeneratedAt:  time.Now().UTC(),
			RecordCounts: counts,
		},
		Facets: facets,
	}
	return snapshot, nil
}

// buildFacets converts parsed region records into a sorted slice of regionmatcher.Facet.
// It validates that each subdistrict references an existing province, city, and district and
// returns an error if any required parent region is missing.
func buildFacets(records map[string]parsedRecord) ([]regionmatcher.Facet, error) {
	provinces := make(map[string]string)
	cities := make(map[string]string)
	districts := make(map[string]string)
	subdistricts := make([]parsedRecord, 0)

	for code, rec := range records {
		switch rec.seg.Level() {
		case regionhierarchy.LevelProvince:
			provinces[code] = rec.name
		case regionhierarchy.LevelCity:
			cities[code] = rec.name
		case regionhierarchy.LevelDistrict:
			districts[code] = rec.name
		case regionhierarchy.LevelSubdistrict:
			subdistricts = append(subdistricts, parsedRecord{name: rec.name, seg: rec.seg})
		}
	}

	facets := make([]regionmatcher.Facet, 0, len(subdistricts))
	for _, rec := range subdistricts {
		provinceName, ok := provinces[rec.seg.Province]
		if !ok {
			return nil, fmt.Errorf("missing province %q for region %s", rec.seg.Province, rec.seg.Raw)
		}
		cityName, ok := cities[rec.seg.City]
		if !ok {
			return nil, fmt.Errorf("missing city %q for region %s", rec.seg.City, rec.seg.Raw)
		}
		districtName, ok := districts[rec.seg.District]
		if !ok {
			return nil, fmt.Errorf("missing district %q for region %s", rec.seg.District, rec.seg.Raw)
		}
		facet := regionmatcher.Facet{
			RegionID:    rec.seg.Subdistrict,
			Subdistrict: rec.name,
			District:    districtName,
			City:        cityName,
			Province:    provinceName,
		}
		facets = append(facets, facet)
	}

	sort.Slice(facets, func(i, j int) bool {
		if facets[i].Province != facets[j].Province {
			return facets[i].Province < facets[j].Province
		}
		if facets[i].City != facets[j].City {
			return facets[i].City < facets[j].City
		}
		if facets[i].District != facets[j].District {
			return facets[i].District < facets[j].District
		}
		if facets[i].Subdistrict != facets[j].Subdistrict {
			return facets[i].Subdistrict < facets[j].Subdistrict
		}
		return facets[i].RegionID < facets[j].RegionID
	})

	return facets, nil
}

// WriteSnapshot writes the given Snapshot to dest as indented JSON and atomically replaces any existing file.
// It ensures the destination directory exists before writing and returns an error if dest is empty or if any filesystem or marshal operation fails.
func WriteSnapshot(snapshot Snapshot, dest string) error {
	if dest == "" {
		return fmt.Errorf("destination path is required for matcher snapshot")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("ensure snapshot directory: %w", err)
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return fmt.Errorf("write temporary snapshot: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}

// LoadSnapshot reads and unmarshals a Snapshot JSON file from the given path.
// It returns an error if path is empty, if the file cannot be read, or if the file contents cannot be parsed as a Snapshot.
func LoadSnapshot(path string) (Snapshot, error) {
	if path == "" {
		return Snapshot{}, fmt.Errorf("snapshot path is required")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(contents, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return snapshot, nil
}
