package api

import (
	"database/sql"
	"log/slog"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Region represents the JSON response structure for a region
type Region struct {
	ID          string `json:"id"`
	Subdistrict string `json:"subdistrict"`
	District    string `json:"district"`
	City        string `json:"city"`
	Province    string `json:"province"`
	PostalCode  string `json:"postal_code"`
	FullText    string `json:"full_text"`
}

// SearchHandler handles the search endpoint
func SearchHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract and validate the q query parameter
		query := c.Query("q")
		if query == "" {
			slog.Warn("Search query parameter missing", "ip", c.IP())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Query parameter 'q' is required",
			})
		}

		// Log the search query
		slog.Info("Processing search request", "query", query, "ip", c.IP())

		// Sanitize the user's query string (lowercase, remove punctuation)
		sanitizedQuery := sanitizeQuery(query)

		// Prepare and execute the Levenshtein SQL query with a placeholder
		// Using a threshold of 3 for the Levenshtein distance
		sqlQuery := `
			SELECT id, subdistrict, district, city, province, postal_code, full_text
			FROM regions
			WHERE full_text ILIKE '%' || ? || '%'
			ORDER BY full_text
			LIMIT 10
		`

		rows, err := db.Query(sqlQuery, sanitizedQuery, sanitizedQuery)
		if err != nil {
			slog.Error("Database query failed", "error", err, "query", query)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database query failed",
			})
		}
		defer rows.Close()

		// Iterate through the results, scanning each row into a Region struct
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
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan row",
				})
			}
			results = append(results, region)
		}

		// Check for errors during iteration
		if err = rows.Err(); err != nil {
			slog.Error("Error iterating rows", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error iterating rows",
			})
		}

		// Log successful search
		slog.Info("Search completed", "query", query, "results", len(results))

		// Return JSON response
		return c.JSON(results)
	}
}

// sanitizeQuery sanitizes the user's query string (lowercase, remove punctuation)
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

// DistrictSearchHandler handles the district search endpoint
func DistrictSearchHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract and validate the q query parameter
		query := c.Query("q")
		if query == "" {
			slog.Warn("District search query parameter missing", "ip", c.IP())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Query parameter 'q' is required",
			})
		}

		// Log the search query
		slog.Info("Processing district search request", "query", query, "ip", c.IP())

		// Sanitize the user's query string (lowercase, remove punctuation)
		sanitizedQuery := sanitizeQuery(query)

		// Prepare and execute the Levenshtein SQL query with a placeholder
		// Using a threshold of 3 for the Levenshtein distance
		sqlQuery := `
			SELECT id, subdistrict, district, city, province, postal_code, full_text
			FROM regions
			WHERE jaro_winkler_similarity (district, ?) >= 0.8
			ORDER BY jaro_winkler_similarity (district, ?) DESC
			LIMIT 10
		`

		rows, err := db.Query(sqlQuery, sanitizedQuery, sanitizedQuery)
		if err != nil {
			slog.Error("Database query failed", "error", err, "query", query)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database query failed",
			})
		}
		defer rows.Close()

		// Iterate through the results, scanning each row into a Region struct
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
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan row",
				})
			}
			results = append(results, region)
		}

		// Check for errors during iteration
		if err = rows.Err(); err != nil {
			slog.Error("Error iterating rows", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error iterating rows",
			})
		}

		// Log successful search
		slog.Info("District search completed", "query", query, "results", len(results))

		// Return JSON response
		return c.JSON(results)
	}
}

// SubdistrictSearchHandler handles the subdistrict search endpoint
func SubdistrictSearchHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract and validate the q query parameter
		query := c.Query("q")
		if query == "" {
			slog.Warn("Subdistrict search query parameter missing", "ip", c.IP())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Query parameter 'q' is required",
			})
		}

		// Log the search query
		slog.Info("Processing subdistrict search request", "query", query, "ip", c.IP())

		// Sanitize the user's query string (lowercase, remove punctuation)
		sanitizedQuery := sanitizeQuery(query)

		// Prepare and execute the Levenshtein SQL query with a placeholder
		// Using a threshold of 3 for the Levenshtein distance
		sqlQuery := `
			SELECT id, subdistrict, district, city, province, postal_code, full_text
			FROM regions
			WHERE jaro_winkler_similarity (subdistrict, ?) >= 0.8
			ORDER BY jaro_winkler_similarity (subdistrict, ?) DESC
			LIMIT 10
		`

		rows, err := db.Query(sqlQuery, sanitizedQuery, sanitizedQuery)
		if err != nil {
			slog.Error("Database query failed", "error", err, "query", query)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database query failed",
			})
		}
		defer rows.Close()

		// Iterate through the results, scanning each row into a Region struct
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
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan row",
				})
			}
			results = append(results, region)
		}

		// Check for errors during iteration
		if err = rows.Err(); err != nil {
			slog.Error("Error iterating rows", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error iterating rows",
			})
		}

		// Log successful search
		slog.Info("Subdistrict search completed", "query", query, "results", len(results))

		// Return JSON response
		return c.JSON(results)
	}
}

// CitySearchHandler handles the city search endpoint
func CitySearchHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract and validate the q query parameter
		query := c.Query("q")
		if query == "" {
			slog.Warn("City search query parameter missing", "ip", c.IP())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Query parameter 'q' is required",
			})
		}

		// Log the search query
		slog.Info("Processing city search request", "query", query, "ip", c.IP())

		// Sanitize the user's query string (lowercase, remove punctuation)
		sanitizedQuery := sanitizeQuery(query)

		// Prepare and execute the Levenshtein SQL query with a placeholder
		// Using a threshold of 3 for the Levenshtein distance
		sqlQuery := `
			SELECT id, subdistrict, district, city, province, postal_code, full_text
			FROM regions
			WHERE
			    jaro_winkler_similarity (city, 'Kota ' || ?) >= 0.8
				OR jaro_winkler_similarity (city, 'Kabupaten ' || ?) >= 0.8
			ORDER BY jaro_winkler_similarity (city, 'Kota ' || ?) DESC, jaro_winkler_similarity (city, 'Kabupaten ' || ?) DESC
			LIMIT 10
		`

		rows, err := db.Query(sqlQuery, sanitizedQuery, sanitizedQuery, sanitizedQuery, sanitizedQuery)
		if err != nil {
			slog.Error("Database query failed", "error", err, "query", query)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database query failed",
			})
		}
		defer rows.Close()

		// Iterate through the results, scanning each row into a Region struct
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
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan row",
				})
			}
			results = append(results, region)
		}

		// Check for errors during iteration
		if err = rows.Err(); err != nil {
			slog.Error("Error iterating rows", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error iterating rows",
			})
		}

		// Log successful search
		slog.Info("City search completed", "query", query, "results", len(results))

		// Return JSON response
		return c.JSON(results)
	}
}

// ProvinceSearchHandler handles the province search endpoint
func ProvinceSearchHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract and validate the q query parameter
		query := c.Query("q")
		if query == "" {
			slog.Warn("Province search query parameter missing", "ip", c.IP())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Query parameter 'q' is required",
			})
		}

		// Log the search query
		slog.Info("Processing province search request", "query", query, "ip", c.IP())

		// Sanitize the user's query string (lowercase, remove punctuation)
		sanitizedQuery := sanitizeQuery(query)

		// Prepare and execute the Levenshtein SQL query with a placeholder
		// Using a threshold of 3 for the Levenshtein distance
		sqlQuery := `
			SELECT id, subdistrict, district, city, province, postal_code, full_text
			FROM regions
			WHERE jaro_winkler_similarity (province, ?) >= 0.8
			ORDER BY jaro_winkler_similarity (province, ?) DESC
			LIMIT 10
		`

		rows, err := db.Query(sqlQuery, sanitizedQuery, sanitizedQuery)
		if err != nil {
			slog.Error("Database query failed", "error", err, "query", query)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database query failed",
			})
		}
		defer rows.Close()

		// Iterate through the results, scanning each row into a Region struct
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
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan row",
				})
			}
			results = append(results, region)
		}

		// Check for errors during iteration
		if err = rows.Err(); err != nil {
			slog.Error("Error iterating rows", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error iterating rows",
			})
		}

		// Log successful search
		slog.Info("Province search completed", "query", query, "results", len(results))

		// Return JSON response
		return c.JSON(results)
	}
}

// PostalCodeSearchHandler handles the postal code search endpoint
func PostalCodeSearchHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract and validate the postal code from path parameter
		postalCode := c.Params("postalCode")
		if postalCode == "" {
			slog.Warn("Postal code parameter missing", "ip", c.IP())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Postal code parameter is required",
			})
		}

		// Validate that postal code is a 5-digit number
		if len(postalCode) != 5 || !isNumeric(postalCode) {
			slog.Warn("Invalid postal code format", "postalCode", postalCode, "ip", c.IP())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Postal code must be a 5-digit number",
			})
		}

		// Log the search query
		slog.Info("Processing postal code search request", "postalCode", postalCode, "ip", c.IP())

		// Prepare and execute the SQL query to find regions by postal code
		sqlQuery := `
			SELECT id, subdistrict, district, city, province, postal_code, full_text
			FROM regions
			WHERE postal_code = ?
			ORDER BY full_text
			LIMIT 10
		`

		rows, err := db.Query(sqlQuery, postalCode)
		if err != nil {
			slog.Error("Database query failed", "error", err, "postalCode", postalCode)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database query failed",
			})
		}
		defer rows.Close()

		// Iterate through the results, scanning each row into a Region struct
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
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan row",
				})
			}
			results = append(results, region)
		}

		// Check for errors during iteration
		if err = rows.Err(); err != nil {
			slog.Error("Error iterating rows", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error iterating rows",
			})
		}

		// Check if no results found
		if len(results) == 0 {
			slog.Info("No results found for postal code", "postalCode", postalCode)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "No regions found for the provided postal code",
			})
		}

		// Log successful search
		slog.Info("Postal code search completed", "postalCode", postalCode, "results", len(results))

		// Return JSON response
		return c.JSON(results)
	}
}

// isNumeric checks if a string contains only numeric characters
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
