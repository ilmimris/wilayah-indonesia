// Package service provides business logic for the wilayah-indonesia API.
// It encapsulates the core functionality for searching Indonesian regions
// by various criteria such as name, postal code, etc.
package service

import (
	"database/sql"
	"log/slog"
	"regexp"
	"strings"
)

// Region represents a region in Indonesia with all its administrative divisions.
type Region struct {
	ID          string `json:"id"`
	Subdistrict string `json:"subdistrict"`
	District    string `json:"district"`
	City        string `json:"city"`
	Province    string `json:"province"`
	PostalCode  string `json:"postal_code"`
	FullText    string `json:"full_text"`
}

// Service encapsulates the business logic for region searches.
type Service struct {
	db *sql.DB
}

// New creates a new Service instance with the provided database connection.
func New(db *sql.DB) *Service {
	return &Service{
		db: db,
	}
}

// Search performs a general search across all regions based on the provided query.
func (s *Service) Search(query string) ([]Region, error) {
	if query == "" {
		return nil, NewError(ErrCodeInvalidInput, "query parameter is required")
	}

	slog.Info("Processing search request", "query", query)

	// Sanitize the user's query string
	sanitizedQuery := sanitizeQuery(query)

	// Prepare and execute the SQL query
	sqlQuery := `
		SELECT id, subdistrict, district, city, province, postal_code, full_text
		FROM regions
		WHERE full_text ILIKE '%' || ? || '%'
		ORDER BY full_text
		LIMIT 10
	`

	rows, err := s.db.Query(sqlQuery, sanitizedQuery)
	if err != nil {
		slog.Error("Database query failed", "error", err, "query", query)
		return nil, NewErrorf(ErrCodeDatabaseFailure, "database query failed: %v", err)
	}
	defer rows.Close()

	// Iterate through the results
	results, err := s.scanRegions(rows)
	if err != nil {
		return nil, err
	}

	slog.Info("Search completed", "query", query, "results", len(results))
	return results, nil
}

// SearchByDistrict searches for regions by district name.
func (s *Service) SearchByDistrict(query string) ([]Region, error) {
	if query == "" {
		return nil, NewError(ErrCodeInvalidInput, "query parameter is required")
	}

	slog.Info("Processing district search request", "query", query)

	// Sanitize the user's query string
	sanitizedQuery := sanitizeQuery(query)

	// Prepare and execute the SQL query
	sqlQuery := `
		SELECT id, subdistrict, district, city, province, postal_code, full_text
		FROM regions
		WHERE jaro_winkler_similarity (district, ?) >= 0.8
		ORDER BY jaro_winkler_similarity (district, ?) DESC
		LIMIT 10
	`

	rows, err := s.db.Query(sqlQuery, sanitizedQuery, sanitizedQuery)
	if err != nil {
		slog.Error("Database query failed", "error", err, "query", query)
		return nil, NewErrorf(ErrCodeDatabaseFailure, "database query failed: %v", err)
	}
	defer rows.Close()

	// Iterate through the results
	results, err := s.scanRegions(rows)
	if err != nil {
		return nil, err
	}

	slog.Info("District search completed", "query", query, "results", len(results))
	return results, nil
}

// SearchBySubdistrict searches for regions by subdistrict name.
func (s *Service) SearchBySubdistrict(query string) ([]Region, error) {
	if query == "" {
		return nil, NewError(ErrCodeInvalidInput, "query parameter is required")
	}

	slog.Info("Processing subdistrict search request", "query", query)

	// Sanitize the user's query string
	sanitizedQuery := sanitizeQuery(query)

	// Prepare and execute the SQL query
	sqlQuery := `
		SELECT id, subdistrict, district, city, province, postal_code, full_text
		FROM regions
		WHERE jaro_winkler_similarity (subdistrict, ?) >= 0.8
		ORDER BY jaro_winkler_similarity (subdistrict, ?) DESC
		LIMIT 10
	`

	rows, err := s.db.Query(sqlQuery, sanitizedQuery, sanitizedQuery)
	if err != nil {
		slog.Error("Database query failed", "error", err, "query", query)
		return nil, NewErrorf(ErrCodeDatabaseFailure, "database query failed: %v", err)
	}
	defer rows.Close()

	// Iterate through the results
	results, err := s.scanRegions(rows)
	if err != nil {
		return nil, err
	}

	slog.Info("Subdistrict search completed", "query", query, "results", len(results))
	return results, nil
}

// SearchByCity searches for regions by city name.
func (s *Service) SearchByCity(query string) ([]Region, error) {
	if query == "" {
		return nil, NewError(ErrCodeInvalidInput, "query parameter is required")
	}

	slog.Info("Processing city search request", "query", query)

	// Sanitize the user's query string
	sanitizedQuery := sanitizeQuery(query)

	// Prepare and execute the SQL query
	sqlQuery := `
		SELECT id, subdistrict, district, city, province, postal_code, full_text
		FROM regions
		WHERE
		    jaro_winkler_similarity (city, 'Kota ' || ?) >= 0.8
			OR jaro_winkler_similarity (city, 'Kabupaten ' || ?) >= 0.8
		ORDER BY jaro_winkler_similarity (city, 'Kota ' || ?) DESC, jaro_winkler_similarity (city, 'Kabupaten ' || ?) DESC
		LIMIT 10
	`

	rows, err := s.db.Query(sqlQuery, sanitizedQuery, sanitizedQuery, sanitizedQuery, sanitizedQuery)
	if err != nil {
		slog.Error("Database query failed", "error", err, "query", query)
		return nil, NewErrorf(ErrCodeDatabaseFailure, "database query failed: %v", err)
	}
	defer rows.Close()

	// Iterate through the results
	results, err := s.scanRegions(rows)
	if err != nil {
		return nil, err
	}

	slog.Info("City search completed", "query", query, "results", len(results))
	return results, nil
}

// SearchByProvince searches for regions by province name.
func (s *Service) SearchByProvince(query string) ([]Region, error) {
	if query == "" {
		return nil, NewError(ErrCodeInvalidInput, "query parameter is required")
	}

	slog.Info("Processing province search request", "query", query)

	// Sanitize the user's query string
	sanitizedQuery := sanitizeQuery(query)

	// Prepare and execute the SQL query
	sqlQuery := `
		SELECT id, subdistrict, district, city, province, postal_code, full_text
		FROM regions
		WHERE jaro_winkler_similarity (province, ?) >= 0.8
		ORDER BY jaro_winkler_similarity (province, ?) DESC
		LIMIT 10
	`

	rows, err := s.db.Query(sqlQuery, sanitizedQuery, sanitizedQuery)
	if err != nil {
		slog.Error("Database query failed", "error", err, "query", query)
		return nil, NewErrorf(ErrCodeDatabaseFailure, "database query failed: %v", err)
	}
	defer rows.Close()

	// Iterate through the results
	results, err := s.scanRegions(rows)
	if err != nil {
		return nil, err
	}

	slog.Info("Province search completed", "query", query, "results", len(results))
	return results, nil
}

// SearchByPostalCode searches for regions by postal code.
func (s *Service) SearchByPostalCode(postalCode string) ([]Region, error) {
	if postalCode == "" {
		return nil, NewError(ErrCodeInvalidInput, "postal code parameter is required")
	}

	// Validate that postal code is a 5-digit number
	if len(postalCode) != 5 || !isNumeric(postalCode) {
		return nil, NewError(ErrCodeInvalidInput, "postal code must be a 5-digit number")
	}

	slog.Info("Processing postal code search request", "postalCode", postalCode)

	// Prepare and execute the SQL query
	sqlQuery := `
		SELECT id, subdistrict, district, city, province, postal_code, full_text
		FROM regions
		WHERE postal_code = ?
		ORDER BY full_text
		LIMIT 10
	`

	rows, err := s.db.Query(sqlQuery, postalCode)
	if err != nil {
		slog.Error("Database query failed", "error", err, "postalCode", postalCode)
		return nil, NewErrorf(ErrCodeDatabaseFailure, "database query failed: %v", err)
	}
	defer rows.Close()

	// Iterate through the results
	results, err := s.scanRegions(rows)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		slog.Info("No results found for postal code", "postalCode", postalCode)
		return nil, NewError(ErrCodeNotFound, "no regions found for the provided postal code")
	}

	slog.Info("Postal code search completed", "postalCode", postalCode, "results", len(results))
	return results, nil
}

// scanRegions iterates through the SQL rows and converts them to Region structs.
func (s *Service) scanRegions(rows *sql.Rows) ([]Region, error) {
	var results []Region
	for rows.Next() {
		var region Region
		err := rows.Scan(
			&region.ID,
			&region.Subdistrict,
			&region.District,
			&region.City,
			&region.Province,
			&region.PostalCode,
			&region.FullText,
		)
		if err != nil {
			slog.Error("Failed to scan row", "error", err)
			return nil, NewErrorf(ErrCodeDatabaseFailure, "failed to scan row: %v", err)
		}
		results = append(results, region)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		slog.Error("Error iterating rows", "error", err)
		return nil, NewErrorf(ErrCodeDatabaseFailure, "error iterating rows: %v", err)
	}

	return results, nil
}

// sanitizeQuery sanitizes the user's query string (lowercase, remove punctuation).
func sanitizeQuery(query string) string {
	// Convert to lowercase
	lowerQuery := strings.ToLower(query)

	// Remove punctuation
	reg := regexp.MustCompile("[^a-zA-Z0-9 ]+")
	sanitized := reg.ReplaceAllString(lowerQuery, "")

	// Convert to sentence case
	sanitized = strings.Title(sanitized)

	// Trim whitespace
	return strings.TrimSpace(sanitized)
}

// isNumeric checks if a string contains only numeric characters.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
