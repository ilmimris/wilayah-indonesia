package regionmatcher

import (
	"math"
	"sort"

	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"
)

const maxIterations = 10000 // Safety break to prevent runaway computation

// runPercolator searches the cross-product of candidate matches across hierarchical levels to find the highest-scoring region suggestion that meets a minimum score.
// runPercolator explores the cross-product of candidate matches across hierarchical levels to find the highest-scoring normalized region Suggestion that meets the given minimum score.
// It evaluates combinations using per-level weights and applies prefixScoreBoost and matchFillBonus when computing scores.
// Returns the best Suggestion and true on success; returns an empty Suggestion and false if no valid normalized candidate is found or the best score is below minScore.
func runPercolator(levelMatches map[Level][]Match, weights map[Level]float64, minScore float64, prefixScoreBoost float64, matchFillBonus float64) (Suggestion, bool) {
	sortedLevels := make([]Level, 0, len(levelOrder))
	for _, level := range levelOrder {
		if _, ok := levelMatches[level]; ok {
			sortedLevels = append(sortedLevels, level)
		}
	}

	// Sort levels by weight in descending order to explore higher-impact choices first.
	sort.SliceStable(sortedLevels, func(i, j int) bool {
		return weights[sortedLevels[i]] > weights[sortedLevels[j]]
	})

	choices := make([][]Match, len(sortedLevels))
	for i, level := range sortedLevels {
		matches := levelMatches[level]
		bucket := make([]Match, 1, len(matches)+1)
		bucket[0] = Match{Level: level} // "None" choice
		bucket = append(bucket, matches...)
		choices[i] = bucket
	}

	var best Suggestion
	bestScore := -math.MaxFloat64
	found := false
	iterations := maxIterations

	var dfs func(int, []Match)
	dfs = func(levelIdx int, currentSelection []Match) {
		if iterations <= 0 {
			return
		}

		// Base case
		if levelIdx == len(sortedLevels) {
			iterations--

			selectionByLevel := make(map[Level]Match)
			for _, match := range currentSelection {
				if match.RegionID != "" {
					selectionByLevel[match.Level] = match
				}
			}

			orderedSelection := make([]Match, len(levelOrder))
			for i, level := range levelOrder {
				if match, ok := selectionByLevel[level]; ok {
					orderedSelection[i] = match
				}
			}

			normalized, ok := normalizeSelection(orderedSelection)
			if ok {
				score := evaluateCombination(normalized, weights, prefixScoreBoost, matchFillBonus)
				if score > bestScore {
					candidate := suggestionFromSelection(normalized)
					if candidate.Province != nil || candidate.City != nil || candidate.District != nil {
						best = candidate
						bestScore = score
						found = true
					}
				}
			}
			return
		}

		choiceBucket := choices[levelIdx]

		for _, choice := range choiceBucket {
			newSelection := make([]Match, len(currentSelection)+1)
			copy(newSelection, currentSelection)
			newSelection[len(currentSelection)] = choice
			dfs(levelIdx+1, newSelection)
		}
	}

	dfs(0, []Match{})

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
// normalizeSelection produces a hierarchy-consistent copy of the input matches for province, city, district, and subdistrict.
// It clears or promotes entries to ensure parent/child RegionID consistency, derives missing province/city codes from a present district when possible,
// and delegates to normalizeWithoutDistrict when no district is provided.
// If both province and city are empty after normalization the function returns nil, false.
// If a subdistrict is present it is anchored to the nearest non-empty upper-level region (district, then city, then province) and cleared if it does not share a prefix with that anchor.
// normalizeSelection ensures a selection of province, city, district, and subdistrict matches is hierarchically consistent and derives missing higher-level codes when possible.
// 
// The input slice is expected to contain matches in the order: province, city, district, subdistrict. The function clears matches that conflict with higher- or lower-level regions, populates missing province/city codes when they can be derived from a district match, and anchors the subdistrict to the nearest non-empty upper-level region if prefixes match.
// 
// If normalization succeeds it returns the normalized slice and true. If the selection cannot be made consistent (for example both province and city are empty after normalization) it returns nil and false.
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
// normalizeWithoutDistrict normalizes a selection when there is no district match, ensuring province/city hierarchy consistency, optionally promoting a city to a province, clearing the district slot, and anchoring the subdistrict to an available city or province only if their region codes share a prefix.
//
// normalization leaves both province and city empty.
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
// evaluateCombination computes a weighted similarity score for a selection of Matches using the provided per-level weights.
// It averages per-match similarity weighted by level, adds a small bonus when adjacent matches share a region prefix,
// evaluateCombination computes a normalized score for a selection of matches using per-level weights.
// It ignores matches with empty RegionID or non-positive weight, adds a small bonus when adjacent non-empty
// matches share a region prefix, applies an incremental bonus for each additional filled level, and clamps
// the final score to a maximum of 1.0.
func evaluateCombination(selection []Match, weights map[Level]float64, prefixScoreBoost float64, matchFillBonus float64) float64 {
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
			// Add a small bonus for sharing a prefix with the parent region.
			weightedSum += prefixScoreBoost
		}
		lastCode = match.RegionID
	}
	if totalWeight == 0 || filled == 0 {
		return 0
	}
	score := weightedSum / totalWeight

	// Add a bonus for each filled level to reward more complete matches.
	score += matchFillBonus * float64(filled-1)

	// The final score is capped at 1.0 to maintain a normalized range.
	if score > 1 {
		score = 1
	}
	return score
}

// suggestionFromSelection converts a normalized slice of Match values into a Suggestion.
// For each non-empty Match in selection, the corresponding level field (as defined by levelOrder)
// suggestionFromSelection builds a Suggestion from a normalized slice of Match values by assigning each non-empty match to the Suggestion field corresponding to its level.
// The input selection is expected to be ordered according to levelOrder; each non-empty Match is copied into the suggestion and the result is harmonized before being returned.
// The returned Suggestion contains the populated region fields (province/city/district/subdistrict) derived from the selection.
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