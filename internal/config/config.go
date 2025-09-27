package config

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/ilmimris/wilayah-indonesia/internal/delivery/http/middleware"
	regiondelivery "github.com/ilmimris/wilayah-indonesia/internal/delivery/http/region"
	"github.com/ilmimris/wilayah-indonesia/internal/delivery/http/router"
	workerdelivery "github.com/ilmimris/wilayah-indonesia/internal/delivery/worker/ingestor"
	"github.com/ilmimris/wilayah-indonesia/internal/gateway/filesystem"
	"github.com/ilmimris/wilayah-indonesia/internal/gateway/sqlnormalize"
	"github.com/ilmimris/wilayah-indonesia/internal/model"
	"github.com/ilmimris/wilayah-indonesia/internal/repository/duckdb"
	sharederrors "github.com/ilmimris/wilayah-indonesia/internal/shared/errors"
	ingestionusecase "github.com/ilmimris/wilayah-indonesia/internal/usecase/ingestion"
	regionusecase "github.com/ilmimris/wilayah-indonesia/internal/usecase/region"

	_ "github.com/marcboeker/go-duckdb/v2"
)

// Options groups runtime configuration flags consumed by bootstrap routines.
type Options struct {
	DBPath    string
	Port      string
	Features  FeatureFlags
	Ingestion IngestionPaths
	ReadOnly  bool
}

// FeatureFlags exposes optional toggles used across the application.
type FeatureFlags struct {
	EnableBPSSearch bool
	IncludeScores   bool
}

// IngestionPaths enumerates filesystem paths required for dataset refresh.
type IngestionPaths struct {
	WilayahSQL    string
	PostalSQL     string
	BPSMappingSQL string
}

// HTTPBootstrap bundles HTTP-specific components produced by BootstrapHTTP.
type HTTPBootstrap struct {
	App    *fiber.App
	DB     *sql.DB
	Logger *slog.Logger
}

// WorkerBootstrap bundles components needed for the ingestion worker.
type WorkerBootstrap struct {
	Logger  *slog.Logger
	DB      *sql.DB
	Runner  *workerdelivery.Runner
	UseCase ingestionusecase.UseCase
}

// NewLogger constructs a slog.Logger placeholder for subsequent wiring phases.
func NewLogger() (*slog.Logger, error) {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})), nil
}

// NewDuckDB creates a DuckDB connection; currently returns ErrNotImplemented.
func NewDuckDB(ctx context.Context, opts Options) (*sql.DB, error) {
	path := opts.DBPath
	if path == "" {
		path = "data/regions.duckdb"
	}
	connStr := path
	if opts.ReadOnly {
		connStr = path + "?access_mode=read_only"
	}
	conn, err := sql.Open("duckdb", connStr)
	if err != nil {
		return nil, sharederrors.Wrap(sharederrors.CodeDatabaseFailure, "failed to open database", err)
	}
	return conn, nil
}

// NewFiber returns a basic Fiber app without middleware, ready for configuration.
func NewFiber() (*fiber.App, error) {
	app := fiber.New()
	return app, nil
}

// BootstrapHTTP wires HTTP components; future phases will connect repositories and use cases.
func BootstrapHTTP(ctx context.Context, opts Options) (HTTPBootstrap, error) {
	opts.ReadOnly = true
	logger, err := NewLogger()
	if err != nil {
		return HTTPBootstrap{}, err
	}

	db, err := NewDuckDB(ctx, opts)
	if err != nil {
		return HTTPBootstrap{}, err
	}

	repo := duckdb.NewRegionRepository(db)
	uc, err := regionusecase.New(ctx, repo, regionusecase.RegionUseCaseOptions{Logger: logger})
	if err != nil {
		db.Close()
		return HTTPBootstrap{}, err
	}

	app, err := NewFiber()
	if err != nil {
		db.Close()
		return HTTPBootstrap{}, err
	}

	app.Use(middleware.RequestLogger())
	app.Use(recover.New())

	controller := regiondelivery.NewController(uc)
	apiGroup := app.Group("/v1")
	router.RegisterRegionRoutes(apiGroup, controller)

	app.Get("/healthz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(model.ErrorResponse{Error: "Database connection failed"})
		}
		return c.JSON(fiber.Map{"status": "ok", "message": "Service is healthy"})
	})

	return HTTPBootstrap{App: app, DB: db, Logger: logger}, nil
}

// BootstrapWorker wires dependencies for the ingestion worker binary.
func BootstrapWorker(ctx context.Context, opts Options) (WorkerBootstrap, error) {
	opts.ReadOnly = false
	logger, err := NewLogger()
	if err != nil {
		return WorkerBootstrap{}, err
	}

	db, err := NewDuckDB(ctx, opts)
	if err != nil {
		return WorkerBootstrap{}, err
	}

	adminRepo := duckdb.NewAdminRepository(db)
	loader := filesystem.FileLoader{}
	normalizer := sqlnormalize.MySQLStripper{}
	uc := ingestionusecase.New(loader, normalizer, adminRepo, ingestionusecase.Options{Logger: logger})
	runner := workerdelivery.NewRunner(uc)

	return WorkerBootstrap{
		Logger:  logger,
		DB:      db,
		Runner:  runner,
		UseCase: uc,
	}, nil
}

// ResolveIngestionPaths populates default paths when not explicitly provided.
func ResolveIngestionPaths(base string, paths IngestionPaths) IngestionPaths {
	if base == "" {
		base = "data"
	}
	withDefault := func(value, name string) string {
		if value != "" {
			return value
		}
		return filepath.Join(base, name)
	}
	return IngestionPaths{
		WilayahSQL:    withDefault(paths.WilayahSQL, "wilayah.sql"),
		PostalSQL:     withDefault(paths.PostalSQL, "wilayah_kodepos.sql"),
		BPSMappingSQL: withDefault(paths.BPSMappingSQL, "bps_wilayah.sql"),
	}
}
