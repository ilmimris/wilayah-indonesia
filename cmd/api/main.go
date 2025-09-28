package main

import (
	"context"
	"log/slog"
	"os"
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
// error and exits with status 1. The bootstrapped database is closed on exit.
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
			MinCombinedScore: 0.5,
			Timeout:          250 * time.Millisecond,
			LevelThresholds: map[regionhierarchy.Level]float64{
				regionhierarchy.LevelProvince:    0.3,  // 0.5
				regionhierarchy.LevelCity:        0.35, // 0.5
				regionhierarchy.LevelDistrict:    0.35, // 0.5
				regionhierarchy.LevelSubdistrict: 0.3,  // 0.4
			},
			ParallelTopK: 50,
		},
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
