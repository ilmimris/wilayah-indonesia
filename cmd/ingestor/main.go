package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

func main() {
	// Connect to a new or existing DuckDB file: data/regions.duckdb
	dbPath := filepath.Join("data", "regions.duckdb")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Read the entire data/wilayah.sql file into a string
	sqlPath := filepath.Join("data", "wilayah.sql")
	sqlData, err := os.ReadFile(sqlPath)
	if err != nil {
		log.Fatal("Failed to read SQL file:", err)
	}

	// Preprocess the SQL to make it compatible with DuckDB
	sqlString := string(sqlData)

	// Remove MySQL-specific syntax
	sqlString = removeMySQLSyntax(sqlString)

	// Execute the string as a single command to create and populate the raw wilayah table
	_, err = db.Exec(sqlString)
	if err != nil {
		log.Fatal("Failed to execute SQL:", err)
	}

	// Execute the transformation query to denormalize the data and create the final regions table
	transformationQuery := `
CREATE OR REPLACE TABLE regions AS
SELECT
    sub.kode AS id,
    sub.nama AS subdistrict,
    dist.nama AS district,
    city.nama AS city,
    prov.nama AS province,
    LOWER(prov.nama || ' ' || city.nama || ' ' || dist.nama || ' ' || sub.nama) AS full_text
FROM
    wilayah AS sub
JOIN wilayah AS dist ON dist.kode = SUBSTRING(sub.kode FROM 1 FOR 8)
JOIN wilayah AS city ON city.kode = SUBSTRING(sub.kode FROM 1 FOR 5)
JOIN wilayah AS prov ON prov.kode = SUBSTRING(sub.kode FROM 1 FOR 2)
WHERE
    LENGTH(sub.kode) = 13;
`

	_, err = db.Exec(transformationQuery)
	if err != nil {
		log.Fatal("Failed to execute transformation query:", err)
	}

	// Clean up by dropping the raw wilayah table
	_, err = db.Exec("DROP TABLE IF EXISTS wilayah;")
	if err != nil {
		log.Fatal("Failed to drop wilayah table:", err)
	}

	fmt.Println("Data ingestion and preparation completed successfully!")
}

// removeMySQLSyntax removes MySQL-specific syntax to make the SQL compatible with DuckDB
func removeMySQLSyntax(sql string) string {
	// Remove ENGINE specification
	re := regexp.MustCompile(`\) ENGINE=[^;]+;`)
	sql = re.ReplaceAllString(sql, ");")

	// Remove CREATE INDEX statements (DuckDB handles indexing differently)
	re = regexp.MustCompile(`CREATE INDEX [^;]+;`)
	sql = re.ReplaceAllString(sql, "")

	// Remove lines that only contain whitespace after processing
	lines := strings.Split(sql, "\n")
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
