package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/ilmimris/wilayah-indonesia/internal/config"
	"github.com/ilmimris/wilayah-indonesia/internal/ngramcache"
	ingestionusecase "github.com/ilmimris/wilayah-indonesia/internal/usecase/ingestion"
)

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
