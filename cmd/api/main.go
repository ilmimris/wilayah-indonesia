package main

import (
	"database/sql"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	_ "github.com/marcboeker/go-duckdb"

	"github.com/ilmimris/wilayah-indonesia/internal/api"
	"github.com/ilmimris/wilayah-indonesia/pkg/service"
)

func main() {
	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Get database path from environment variable or default to data/regions.duckdb
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/regions.duckdb"
	}

	// Open a read-only connection to the database file
	db, err := sql.Open("duckdb", dbPath+"?access_mode=read_only")
	if err != nil {
		slog.Error("Failed to open database connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create service and handler instances
	svc := service.New(db)
	handler := api.New(svc)

	// Set up a new Fiber application
	app := fiber.New()

	// Define the search endpoint
	app.Get("/v1/search", handler.SearchHandler())

	// Define the district search endpoint
	app.Get("/v1/search/district", handler.DistrictSearchHandler())

	// Define the subdistrict search endpoint
	app.Get("/v1/search/subdistrict", handler.SubdistrictSearchHandler())

	// Define the city search endpoint
	app.Get("/v1/search/city", handler.CitySearchHandler())

	// Define the province search endpoint
	app.Get("/v1/search/province", handler.ProvinceSearchHandler())

	// Define the postal code search endpoint
	app.Get("/v1/search/postal/:postalCode", handler.PostalCodeSearchHandler())

	// Add health check endpoint
	app.Get("/healthz", func(c *fiber.Ctx) error {
		// Check database connection
		err := db.Ping()
		if err != nil {
			slog.Error("Database connection failed in health check", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Database connection failed",
			})
		}
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Service is healthy",
		})
	})

	// Get port from environment variable or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start the Fiber server on a configurable port
	slog.Info("Server starting", "port", port)
	err = app.Listen(":" + port)
	if err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
