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

// ParseRegionID validates and splits a region identifier into hierarchical segments.
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

	if segs.Subdistrict == "" && segs.level == LevelSubdistrict {
		segs.Subdistrict = trimmed
	}
	if segs.District == "" && segs.level == LevelDistrict {
		segs.District = trimmed
	}
	if segs.City == "" && segs.level == LevelCity {
		segs.City = trimmed
	}

	return segs, nil
}

// SharePrefix reports whether parent and child codes align on their hierarchical prefix.
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

// IsConsistentHierarchy reports whether the provided codes can coexist within the same administrative chain.
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

// CodeAtLevel flattens a region identifier to the requested hierarchy level.
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
