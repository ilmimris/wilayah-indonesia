package regionhierarchy

import (
	"fmt"
	"regexp"
	"strings"
)

// Level enumerates Indonesian administrative hierarchy layers.
type Level string

const (
	LevelUnknown     Level = "unknown"
	LevelProvince    Level = "province"
	LevelCity        Level = "city"
	LevelDistrict    Level = "district"
	LevelSubdistrict Level = "subdistrict"
)

var (
	regionIDPattern = regexp.MustCompile(`^\d{2}(?:\.\d{2}(?:\.\d{2}(?:\.\d{4})?)?)?$`)
)

// Segments captures canonical identifiers for each hierarchy level derived from a region ID.
type Segments struct {
	Raw         string
	Province    string
	City        string
	District    string
	Subdistrict string
	level       Level
}

// Level reports the specific hierarchy depth represented by the region ID.
func (s Segments) Level() Level {
	return s.level
}

// CodeForLevel returns the identifier corresponding to the requested hierarchy level.
func (s Segments) CodeForLevel(level Level) string {
	switch level {
	case LevelProvince:
		return s.Province
	case LevelCity:
		return s.City
	case LevelDistrict:
		return s.District
	case LevelSubdistrict:
		return s.Subdistrict
	default:
		return ""
	}
}

// ParseRegionID validates an Indonesian region identifier and returns its hierarchical segments.
// 
// The function trims whitespace, enforces the expected region ID pattern (province and optional
// dot-separated city, district, and subdistrict components), and splits the input into a Segments
// value with Province, City, District, Subdistrict fields and an internal level set to the
// corresponding depth. If a segment for the reported depth is empty, that segment is set to the
// trimmed input to preserve boundary consistency.
// 
// the identifier has unsupported depth (more than four dot-separated parts).
func ParseRegionID(id string) (Segments, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return Segments{}, fmt.Errorf("region ID cannot be empty")
	}
	if !regionIDPattern.MatchString(trimmed) {
		return Segments{}, fmt.Errorf("region ID %q does not match expected pattern VV(.XX(.YY(.ZZZZ)?)?)", id)
	}

	parts := strings.Split(trimmed, ".")
	var segs Segments
	segs.Raw = trimmed

	switch len(parts) {
	case 1:
		segs.Province = parts[0]
		segs.level = LevelProvince
	case 2:
		segs.Province = parts[0]
		segs.City = strings.Join(parts[:2], ".")
		segs.level = LevelCity
	case 3:
		segs.Province = parts[0]
		segs.City = strings.Join(parts[:2], ".")
		segs.District = strings.Join(parts[:3], ".")
		segs.level = LevelDistrict
	case 4:
		segs.Province = parts[0]
		segs.City = strings.Join(parts[:2], ".")
		segs.District = strings.Join(parts[:3], ".")
		segs.Subdistrict = strings.Join(parts[:4], ".")
		segs.level = LevelSubdistrict
	default:
		return Segments{}, fmt.Errorf("region ID %q has unsupported depth", id)
	}

        return segs, nil

	return segs, nil
}

// SharePrefix reports whether the child region code has the parent code as its hierarchical prefix,
// using '.' as the segment separator.
// 
// It returns true for exact equality and for cases where the child begins with the parent
// than parent requires the next character to be '.' to ensure a proper boundary.
func SharePrefix(parent, child string) bool {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)
	if parent == "" || child == "" {
		return false
	}
	if parent == child {
		return true
	}
	if !strings.HasPrefix(child, parent) {
		return false
	}
	// Ensure boundary matches administrative separators.
	if len(child) > len(parent) {
		return child[len(parent)] == '.'
	}
	return false
}

// IsConsistentHierarchy reports whether the provided region codes can coexist in a single
// administrative chain by verifying hierarchical prefix compatibility.
//
// It trims whitespace and ignores empty strings. The first non-empty code becomes the
// anchor; each subsequent non-empty code must share a valid hierarchical prefix with the
// anchor in either direction. The anchor is updated to the longest code seen so far.
// IsConsistentHierarchy reports whether the provided region codes can coexist in a single hierarchical chain.
// It returns true if every non-empty code (after trimming whitespace) is either an exact match or a dot-delimited hierarchical prefix of another code, and false if any pair is incompatible.
func IsConsistentHierarchy(codes ...string) bool {
	var anchor string
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if anchor == "" {
			anchor = code
			continue
		}
		if !SharePrefix(code, anchor) && !SharePrefix(anchor, code) {
			return false
		}
		if len(code) > len(anchor) {
			anchor = code
		}
	}
	return true
}

// CodeAtLevel returns the region code corresponding to the specified hierarchy level for the given region identifier.
// It parses and validates the provided id and returns the code for LevelProvince, LevelCity, LevelDistrict, or LevelSubdistrict.
// CodeAtLevel retrieves the region code corresponding to the specified hierarchy level from the provided region ID.
// It returns the code for the requested level or an error.
// An error is returned if parsing the region ID fails or if the provided level is unknown.
func CodeAtLevel(id string, level Level) (string, error) {
	segs, err := ParseRegionID(id)
	if err != nil {
		return "", err
	}
	switch level {
	case LevelProvince:
		return segs.Province, nil
	case LevelCity:
		return segs.City, nil
	case LevelDistrict:
		return segs.District, nil
	case LevelSubdistrict:
		return segs.Subdistrict, nil
	default:
		return "", fmt.Errorf("unknown level %q", level)
	}
}
