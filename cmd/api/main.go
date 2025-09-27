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
// It reads DB_PATH, PORT, and MATCHER_SNAPSHOT_PATH (among other configured defaults), sets the process logger, and exits the process if bootstrapping or server startup fails.
func main() {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(bootstrapLogger)

	ctx := context.Background()
	opts := config.Options{
		DBPath:   os.Getenv("DB_PATH"),
		Port:     os.Getenv("PORT"),
		ReadOnly: true,
		Matcher: config.MatcherConfig{
			SnapshotPath:     os.Getenv("MATCHER_SNAPSHOT_PATH"),
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
