package service_test

import (
	"database/sql"
	"log"

	"github.com/ilmimris/wilayah-indonesia/pkg/service"
	_ "github.com/marcboeker/go-duckdb"
)

// Example of how to use the service package
func ExampleService() {
	// Open a connection to the database
	db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create a new service instance
	svc := service.New(db)

	// Perform a general search
	regions, err := svc.Search("Jakarta")
	if err != nil {
		log.Fatal(err)
	}

	// Process the results
	for _, region := range regions {
		log.Printf("Found region: %s, %s, %s", region.Subdistrict, region.District, region.City)
	}
}
