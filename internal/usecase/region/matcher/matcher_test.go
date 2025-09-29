package regionmatcher

import "testing"

func TestCandidateFragmentsCoversThreeWordCombinations(t *testing.T) {
	query := "Kabupaten Aceh Selatan Bakongan Timur"
	fragments := candidateFragments(query, 3)
	if len(fragments) == 0 {
		t.Fatalf("expected fragments to be generated")
	}
	if len(fragments) > 10 {
		t.Fatalf("expected fragments to be capped at 10, got %d", len(fragments))
	}
	want := "aceh selatan bakongan"
	found := false
	for _, fragment := range fragments {
		if fragment == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fragment %q to be present in %v", want, fragments)
	}
}

func TestMergeMatchesPrefersProvinceHint(t *testing.T) {
	provinceAligned := Match{
		Level:      LevelDistrict,
		Name:       "Meureubo",
		Province:   "Aceh Barat",
		Fragment:   "aceh barat meureubo",
		RegionID:   "11.05.04",
		Similarity: 0.9,
	}
	ambiguous := Match{
		Level:      LevelDistrict,
		Name:       "Meureubo",
		Province:   "Aceh Barat Daya",
		Fragment:   "meureubo",
		RegionID:   "11.13.04",
		Similarity: 0.9,
	}

	merged := mergeMatches(nil, []Match{ambiguous, provinceAligned}, 0)
	if len(merged) == 0 {
		t.Fatalf("expected merged matches to be non-empty")
	}
	if merged[0].Province != "Aceh Barat" {
		t.Fatalf("expected province-aware match to win tie-break, got %+v", merged[0])
	}
}
