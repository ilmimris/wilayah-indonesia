package regionmatcher

import (
	"math"

	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"
)

// runPercolator searches the cross-product of candidate matches across hierarchical levels to find the highest‑scoring region suggestion that meets a minimum score.
// 
// It examines combinations drawn from levelMatches, scores each normalized and hierarchy-consistent selection using weights, and returns the best Suggestion and true when a non-empty province/city/district candidate meets or exceeds minScore. If no valid candidate is found or the best score is below minScore, it returns an empty Suggestion and false. The returned Suggestion.Score may be adjusted upward by a secondary averaging heuristic before being returned.
func runPercolator(levelMatches map[Level][]Match, weights map[Level]float64, minScore float64) (Suggestion, bool) {
	choices := make([][]Match, len(levelOrder))
	for i, level := range levelOrder {
		matches := levelMatches[level]
		bucket := make([]Match, 1, len(matches)+1)
		bucket[0] = Match{}
		bucket = append(bucket, matches...)
		choices[i] = bucket
	}

	indices := make([]int, len(levelOrder))
	var best Suggestion
	bestScore := -math.MaxFloat64
	found := false

	for {
		selection := make([]Match, len(levelOrder))
		for i, bucket := range choices {
			if len(bucket) == 0 {
				continue
			}
			selection[i] = bucket[indices[i]]
		}

		normalized, ok := normalizeSelection(selection)
		if ok {
			score := evaluateCombination(normalized, weights)
			if score > bestScore {
				candidate := suggestionFromSelection(normalized)
				if candidate.Province != nil || candidate.City != nil || candidate.District != nil {
					best = candidate
					bestScore = score
					found = true
				}
			}
		}

		// increment
		idx := len(levelOrder) - 1
		for idx >= 0 {
			if len(choices[idx]) == 0 {
				idx--
				continue
			}
			indices[idx]++
			if indices[idx] < len(choices[idx]) {
				break
			}
			indices[idx] = 0
			idx--
		}
		if idx < 0 {
			break
		}
	}

	if !found || bestScore < minScore {
		return Suggestion{}, false
	}
	best.Score = bestScore
	if avg := scoreSuggestion(best); avg > best.Score {
		best.Score = avg
	}
	return best, true
}

// normalizeSelection normalizes a slice of Matches to ensure hierarchical consistency across
// province, city, district, and subdistrict levels.
//
// It returns an adjusted copy of the input where inconsistent lower-level matches are cleared,
// missing province/city codes may be derived from a district match, and a subdistrict is kept
// only if it can be anchored to a nearest higher-level region (district, then city, then province).
// If the district is absent the function delegates to normalizeWithoutDistrict to perform
// appropriate promotions and validations. The function returns (nil, false) when the selection
// cannot be normalized into a valid hierarchy (for example when both province and city are empty
// after normalization); otherwise it returns the normalized slice and true.
func normalizeSelection(selection []Match) ([]Match, bool) {
	normalized := make([]Match, len(selection))
	copy(normalized, selection)

	province := &normalized[0]
	city := &normalized[1]
	district := &normalized[2]
	subdistrict := &normalized[3]

	if province.RegionID != "" && city.RegionID != "" && !regionhierarchy.IsConsistentHierarchy(province.RegionID, city.RegionID) {
		*city = Match{}
	}

	if district.RegionID != "" {
		if city.RegionID != "" && !regionhierarchy.IsConsistentHierarchy(city.RegionID, district.RegionID) {
			*city = Match{}
		}
		if province.RegionID != "" && !regionhierarchy.IsConsistentHierarchy(province.RegionID, district.RegionID) {
			*province = Match{}
		}
		if city.RegionID == "" {
			if code, err := regionhierarchy.CodeAtLevel(district.RegionID, regionhierarchy.LevelCity); err == nil && code != "" {
				*city = Match{Level: LevelCity, Name: district.City, RegionID: code, Similarity: district.Similarity, Province: district.Province}
			}
		}
		if province.RegionID == "" {
			if code, err := regionhierarchy.CodeAtLevel(district.RegionID, regionhierarchy.LevelProvince); err == nil && code != "" {
				*province = Match{Level: LevelProvince, Name: district.Province, RegionID: code, Similarity: district.Similarity}
			}
		}
	} else {
		updated, ok := normalizeWithoutDistrict(normalized, normalized[0], normalized[1], normalized[3])
		if !ok {
			return nil, false
		}
		normalized = updated
		province = &normalized[0]
		city = &normalized[1]
		district = &normalized[2]
		subdistrict = &normalized[3]
	}

	if province.RegionID == "" && city.RegionID == "" {
		return nil, false
	}

	if subdistrict.RegionID != "" {
		anchorID := district.RegionID
		if anchorID == "" {
			anchorID = city.RegionID
		}
		if anchorID == "" {
			anchorID = province.RegionID
		}
		if anchorID == "" || (!regionhierarchy.SharePrefix(anchorID, subdistrict.RegionID) && !regionhierarchy.SharePrefix(subdistrict.RegionID, anchorID)) {
			*subdistrict = Match{}
		}
	}

	return normalized, true
}

// normalizeWithoutDistrict normalizes a selection when no district-level match is present.
// 
// It ensures at least one of province or city remains and that province/city are hierarchically consistent.
// If province is empty but city contains a province-level code, the city is promoted to a province match.
// The district slot is cleared. If a subdistrict is provided, it is retained only when it shares a prefix with an anchor region (city if present, otherwise province); otherwise the subdistrict is cleared.
// 
// Returns the possibly-updated selection and true on success, or nil and false if neither province nor city is present after normalization.
func normalizeWithoutDistrict(selection []Match, province, city, subdistrict Match) ([]Match, bool) {
	if province.RegionID == "" && city.RegionID == "" {
		return nil, false
	}
	if province.RegionID != "" && city.RegionID != "" && !regionhierarchy.IsConsistentHierarchy(province.RegionID, city.RegionID) {
		city = Match{}
	}

	if city.RegionID != "" && province.RegionID == "" {
		if code, err := regionhierarchy.CodeAtLevel(city.RegionID, regionhierarchy.LevelProvince); err == nil && code != "" {
			province = Match{Level: LevelProvince, Name: city.Province, RegionID: code, Similarity: city.Similarity}
		}
	}

	selection[0] = province
	selection[1] = city
	selection[2] = Match{}

	if selection[0].RegionID == "" && selection[1].RegionID == "" {
		return nil, false
	}

	if subdistrict.RegionID != "" {
		anchorID := selection[1].RegionID
		if anchorID == "" {
			anchorID = selection[0].RegionID
		}
		if anchorID == "" || (!regionhierarchy.SharePrefix(anchorID, subdistrict.RegionID) && !regionhierarchy.SharePrefix(subdistrict.RegionID, anchorID)) {
			selection[3] = Match{}
		}
	}

	return selection, true
}

// evaluateCombination computes a weighted score for the given selection using the provided per-level weights.
// It returns a score in the range [0, 1] where higher values indicate better overall match quality.
// Matches with empty RegionID or non-positive weight are ignored. Selections that contain multiple matched
// levels receive small bonuses when adjacent matches share a region prefix and for each additional filled level.
func evaluateCombination(selection []Match, weights map[Level]float64) float64 {
	var weightedSum float64
	var totalWeight float64
	filled := 0
	lastCode := ""
	for i, match := range selection {
		if match.RegionID == "" {
			continue
		}
		weight := weights[levelOrder[i]]
		if weight <= 0 {
			continue
		}
		weightedSum += weight * match.Similarity
		totalWeight += weight
		filled++
		if lastCode != "" && regionhierarchy.SharePrefix(lastCode, match.RegionID) {
			weightedSum += 0.01
		}
		lastCode = match.RegionID
	}
	if totalWeight == 0 || filled == 0 {
		return 0
	}
	score := weightedSum / totalWeight
	score += 0.03 * float64(filled-1)
	if score > 1 {
		score = 1
	}
	return score
}

// suggestionFromSelection converts a normalized slice of Match values into a Suggestion.
// For each non-empty Match in selection, the corresponding level field (as defined by levelOrder)
// is populated with that match. The assembled Suggestion is then harmonized before being returned.
func suggestionFromSelection(selection []Match) Suggestion {
	suggestion := Suggestion{}
	for i, match := range selection {
		if match.RegionID == "" {
			continue
		}
		clone := match
		suggestion.matchSetter(levelOrder[i], &clone)
	}
	harmonizeSuggestion(&suggestion)
	return suggestion
}
