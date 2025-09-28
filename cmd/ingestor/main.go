package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/ilmimris/wilayah-indonesia/internal/config"
	"github.com/ilmimris/wilayah-indonesia/internal/ngramcache"
	ingestionusecase "github.com/ilmimris/wilayah-indonesia/internal/usecase/ingestion"
)

// main is the program entry point for the ingestion worker.
// It configures a bootstrap logger, resolves ingestion paths and options from environment,
// initializes the worker, runs the ingestion workflow, and then builds and persists a matcher
// snapshot from the Wilayah SQL source. The function logs progress and exits with a non-zero
// main initializes the ingestion worker, executes the ingestion workflow, builds a matcher snapshot from the Wilayah SQL source, and persists that snapshot.
//
// It reads DATA_DIR and DB_PATH from the environment, bootstraps the worker, runs the ingestion steps, and writes the generated matcher snapshot to the configured snapshot path while logging progress. On any initialization, ingestion, or snapshot failure the program logs the error and exits with a non-zero status.
func main() {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(bootstrapLogger)

	ctx := context.Background()

	dataDir := os.Getenv("DATA_DIR")
	paths := config.ResolveIngestionPaths(dataDir, config.IngestionPaths{})

	opts := config.Options{
		DBPath:    os.Getenv("DB_PATH"),
		Ingestion: paths,
		ReadOnly:  false,
	}

	bootstrap, err := config.BootstrapWorker(ctx, opts)
	if err != nil {
		slog.Error("Failed to bootstrap ingestion worker", "error", err)
		os.Exit(1)
	}
	defer bootstrap.DB.Close()

	if strings.TrimSpace(bootstrap.Matcher.SnapshotPath) == "" {
		slog.Error("MATCHER_SNAPSHOT_PATH (or the configured snapshot path) is missing")
		os.Exit(1)
	}

	refreshOpts := ingestionusecase.RefreshOptions{
		WilayahSQLPath:    paths.WilayahSQL,
		PostalSQLPath:     paths.PostalSQL,
		BPSMappingSQLPath: paths.BPSMappingSQL,
	}

	slog.Info("Running ingestion workflow")
	if err := bootstrap.Runner.Run(ctx, refreshOpts); err != nil {
		slog.Error("Ingestion failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Ingestion completed successfully")

	slog.Info("Building matcher snapshot", "source", paths.WilayahSQL, "destination", bootstrap.Matcher.SnapshotPath)
	snapshot, err := ngramcache.BuildSnapshotFromWilayah(paths.WilayahSQL)
	if err != nil {
		slog.Error("Failed to build matcher snapshot", "error", err)
		os.Exit(1)
	}
	if err := ngramcache.WriteSnapshot(snapshot, bootstrap.Matcher.SnapshotPath); err != nil {
		slog.Error("Failed to persist matcher snapshot", "error", err)
		os.Exit(1)
	}
	slog.Info("Matcher snapshot generated", "path", bootstrap.Matcher.SnapshotPath, "facets", len(snapshot.Facets), "dataset_hash", snapshot.Metadata.DatasetHash)
}
