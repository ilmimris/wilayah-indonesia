package regionmatcher

import "testing"

func TestRunPercolatorSelectsBestPath(t *testing.T) {
	levelMatches := map[Level][]Match{
		LevelProvince: {
			{Level: LevelProvince, Name: "Aceh", RegionID: "11", Similarity: 0.92},
		},
		LevelCity: {
			{Level: LevelCity, Name: "Kabupaten Aceh Selatan", RegionID: "11.01", Similarity: 0.88},
		},
		LevelDistrict: {
			{Level: LevelDistrict, Name: "Bakongan", RegionID: "11.01.01", Similarity: 0.85},
		},
		LevelSubdistrict: {
			{Level: LevelSubdistrict, Name: "Keude Bakongan", RegionID: "11.01.01.2001", Similarity: 0.83},
		},
	}

	weights := map[Level]float64{
		LevelProvince:    0.2,
		LevelCity:        0.3,
		LevelDistrict:    0.25,
		LevelSubdistrict: 0.25,
	}

	suggestion, ok := runPercolator(levelMatches, weights, 0.6)
	if !ok {
		t.Fatalf("expected percolator to return suggestion")
	}
	if suggestion.Province == nil || suggestion.Province.Name != "Aceh" {
		t.Fatalf("unexpected province match: %+v", suggestion.Province)
	}
	if suggestion.City == nil || suggestion.City.RegionID != "11.01" {
		t.Fatalf("unexpected city match: %+v", suggestion.City)
	}
	if suggestion.Score <= 0 {
		t.Fatalf("expected positive score, got %f", suggestion.Score)
	}
}

func TestRunPercolatorReplacesInconsistentCity(t *testing.T) {
	levelMatches := map[Level][]Match{
		LevelProvince: {{Level: LevelProvince, Name: "Aceh", RegionID: "11", Similarity: 0.9}},
		LevelCity:     {{Level: LevelCity, Name: "Kota Banda Aceh", RegionID: "11.71", Similarity: 0.88}},
		LevelDistrict: {{Level: LevelDistrict, Name: "Bakongan", RegionID: "11.01.01", Similarity: 0.85}},
	}

	suggestion, ok := runPercolator(levelMatches, map[Level]float64{LevelProvince: 0.4, LevelCity: 0.3, LevelDistrict: 0.3}, 0.5)
	if !ok {
		t.Fatalf("expected percolator to keep district and derive consistent city")
	}
	if suggestion.City == nil || suggestion.City.RegionID != "11.01" {
		t.Fatalf("expected city to be realigned, got %+v", suggestion.City)
	}
	if suggestion.District == nil {
		t.Fatalf("expected district to remain present")
	}
}

func TestRunPercolatorAllowsMissingSubdistrict(t *testing.T) {
	levelMatches := map[Level][]Match{
		LevelProvince: {{Level: LevelProvince, Name: "Sulawesi Tengah", RegionID: "72", Similarity: 0.78}},
		LevelCity:     {{Level: LevelCity, Name: "Kabupaten Banggai", RegionID: "72.01", Similarity: 0.76}},
		LevelSubdistrict: {
			{Level: LevelSubdistrict, Name: "Sindang Sari", RegionID: "18.01.05.2013", Similarity: 0.7},
		},
	}

	suggestion, ok := runPercolator(levelMatches, map[Level]float64{LevelProvince: 0.3, LevelCity: 0.5, LevelSubdistrict: 0.2}, 0.5)
	if !ok {
		t.Fatalf("expected percolator to accept hierarchy without subdistrict")
	}
	if suggestion.Subdistrict != nil {
		t.Fatalf("expected subdistrict to be omitted, got %+v", suggestion.Subdistrict)
	}
	if suggestion.City == nil || suggestion.City.RegionID != "72.01" {
		t.Fatalf("expected city to remain, got %+v", suggestion.City)
	}
}

func TestRunPercolatorDerivesProvinceFromDistrict(t *testing.T) {
	levelMatches := map[Level][]Match{
		LevelCity: {
			{Level: LevelCity, Name: "Kabupaten Banggai", RegionID: "72.01", Similarity: 0.78},
		},
		LevelDistrict: {
			{Level: LevelDistrict, Name: "Toili Barat", RegionID: "72.01.12", Similarity: 0.75},
		},
	}

	suggestion, ok := runPercolator(levelMatches, map[Level]float64{LevelProvince: 0.2, LevelCity: 0.4, LevelDistrict: 0.4}, 0.5)
	if !ok {
		t.Fatalf("expected percolator to accept chain without explicit province")
	}
	if suggestion.District == nil || suggestion.District.RegionID != "72.01.12" {
		t.Fatalf("unexpected district: %+v", suggestion.District)
	}
}
