package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/ilmimris/wilayah-indonesia/internal/config"
	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"
)

// main initializes logging, constructs runtime options from environment variables, bootstraps the HTTP application, and starts the HTTP server.
// main is the program entry point that configures logging, constructs runtime options from environment
// variables (including DB_PATH, PORT, and MATCHER_SNAPSHOT_PATH), bootstraps the HTTP application, and
// starts the HTTP server.
//
// It sets a temporary bootstrap logger, populates config.Options with a MatcherConfig and other defaults,
// invokes config.BootstrapHTTP, and—if the bootstrap provides a logger—sets it as the default. The server
// listens on PORT (defaults to "8080" when empty). If bootstrapping or server startup fails, main logs an
// main is the program entry point. It sets up a temporary bootstrap logger, requires
// MATCHER_SNAPSHOT_PATH, constructs runtime options (including matcher settings such
// as snapshot path, thresholds, timeout, and parallel top-K), bootstraps the HTTP
// application, replaces the default logger if provided, defers closing the bootstrapped
// database, and starts the HTTP server on PORT (default "8080"). On fatal errors it
// logs the problem and exits with status 1.
func main() {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(bootstrapLogger)

	matcherSnapshotPath, ok := os.LookupEnv("MATCHER_SNAPSHOT_PATH")
	if !ok || matcherSnapshotPath == "" {
		slog.Error("MATCHER_SNAPSHOT_PATH environment variable not set or empty")
		os.Exit(1)
	}

	ctx := context.Background()
	opts := config.Options{
		DBPath:   os.Getenv("DB_PATH"),
		Port:     os.Getenv("PORT"),
		ReadOnly: true,
		Matcher: config.MatcherConfig{
			SnapshotPath:     matcherSnapshotPath,
			MinCombinedScore: 0.8,
			Timeout:          250 * time.Millisecond,
			LevelThresholds: map[regionhierarchy.Level]float64{
				regionhierarchy.LevelProvince:    0.4,
				regionhierarchy.LevelCity:        0.5,
				regionhierarchy.LevelDistrict:    0.58,
				regionhierarchy.LevelSubdistrict: 0.45,
			},
			WordComboSize: 3,
			ParallelTopK:  50,
		},
	}

	if value := os.Getenv("MATCHER_MIN_SCORE"); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err != nil || parsed <= 0 || parsed > 1 {
			slog.Warn("invalid MATCHER_MIN_SCORE override, using default", "value", value, "error", err)
		} else {
			opts.Matcher.MinCombinedScore = parsed
		}
	}

	thresholdEnv := map[regionhierarchy.Level]string{
		regionhierarchy.LevelProvince:    "MATCHER_THRESHOLD_PROVINCE",
		regionhierarchy.LevelCity:        "MATCHER_THRESHOLD_CITY",
		regionhierarchy.LevelDistrict:    "MATCHER_THRESHOLD_DISTRICT",
		regionhierarchy.LevelSubdistrict: "MATCHER_THRESHOLD_SUBDISTRICT",
	}
	for level, envName := range thresholdEnv {
		if value := os.Getenv(envName); value != "" {
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil || parsed < 0 || parsed > 1 {
				slog.Warn("invalid matcher threshold override", "env", envName, "value", value, "error", err)
				continue
			}
			opts.Matcher.LevelThresholds[level] = parsed
		}
	}

	if value := os.Getenv("MATCHER_WORD_COMBO_SIZE"); value != "" {
		if parsed, err := strconv.Atoi(value); err != nil || parsed <= 0 {
			slog.Warn("invalid MATCHER_WORD_COMBO_SIZE override", "value", value, "error", err)
		} else {
			opts.Matcher.WordComboSize = parsed
		}
	}

	if value := os.Getenv("MATCHER_PARALLEL_TOP_K"); value != "" {
		if parsed, err := strconv.Atoi(value); err != nil || parsed <= 0 {
			slog.Warn("invalid MATCHER_PARALLEL_TOP_K override", "value", value, "error", err)
		} else {
			opts.Matcher.ParallelTopK = parsed
		}
	}

	bootstrap, err := config.BootstrapHTTP(ctx, opts)
	if err != nil {
		slog.Error("Failed to bootstrap HTTP application", "error", err)
		os.Exit(1)
	}
	defer bootstrap.DB.Close()

	if bootstrap.Logger != nil {
		slog.SetDefault(bootstrap.Logger)
	}

	port := opts.Port
	if port == "" {
		port = "8080"
	}

	slog.Info("Server starting", "port", port)
	if err := bootstrap.App.Listen(":" + port); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
