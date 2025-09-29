package model

import "github.com/ilmimris/wilayah-indonesia/internal/entity"

// RegionResponse is the HTTP representation of a region record.
type RegionResponse struct {
	ID          string              `json:"id"`
	Subdistrict string              `json:"subdistrict"`
	District    string              `json:"district"`
	City        string              `json:"city"`
	Province    string              `json:"province"`
	PostalCode  string              `json:"postal_code"`
	FullText    string              `json:"full_text"`
	BPS         *entity.RegionBPS   `json:"bps,omitempty"`
	Scores      *entity.RegionScore `json:"scores,omitempty"`
}

// SuggestedMatch documents the matcher output for a specific administrative level.
type SuggestedMatch struct {
	Name       string  `json:"name"`
	RegionID   string  `json:"region_id"`
	Similarity float64 `json:"similarity"`
	Fragment   string  `json:"fragment,omitempty"`
}

// Suggestion aggregates matcher results returned alongside search queries.
type Suggestion struct {
	Strategy    string          `json:"strategy"`
	Score       float64         `json:"score"`
	Province    *SuggestedMatch `json:"province,omitempty"`
	City        *SuggestedMatch `json:"city,omitempty"`
	District    *SuggestedMatch `json:"district,omitempty"`
	Subdistrict *SuggestedMatch `json:"subdistrict,omitempty"`
}

// SearchResponse wraps a list of region responses for transport layers.
type SearchResponse struct {
	Items      []RegionResponse `json:"items"`
	Suggestion *Suggestion      `json:"suggestion,omitempty"`
}
