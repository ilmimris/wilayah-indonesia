package config

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	"github.com/ilmimris/wilayah-indonesia/internal/ngramcache"
	duckdbrepo "github.com/ilmimris/wilayah-indonesia/internal/repository/duckdb"
	sharederrors "github.com/ilmimris/wilayah-indonesia/internal/shared/errors"
	ingestionusecase "github.com/ilmimris/wilayah-indonesia/internal/usecase/ingestion"
	regionusecase "github.com/ilmimris/wilayah-indonesia/internal/usecase/region"
	regionmatcher "github.com/ilmimris/wilayah-indonesia/internal/usecase/region/matcher"
	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"

	duckdb "github.com/marcboeker/go-duckdb/v2"
)

// Options groups runtime configuration flags consumed by bootstrap routines.
type Options struct {
	DBPath       string
	Port         string
	Features     FeatureFlags
	Ingestion    IngestionPaths
	ReadOnly     bool
	Matcher      MatcherConfig
	MaxOpenConns int
	MaxIdleConns int
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
	App     *fiber.App
	DB      *sql.DB
	Logger  *slog.Logger
	Matcher MatcherConfig
}

// WorkerBootstrap bundles components needed for the ingestion worker.
type WorkerBootstrap struct {
	Logger  *slog.Logger
	DB      *sql.DB
	Runner  *workerdelivery.Runner
	UseCase ingestionusecase.UseCase
	Matcher MatcherConfig
}

// MatcherConfig tunes the behaviour of the region matcher and percolator.
type MatcherConfig struct {
	SnapshotPath     string
	Timeout          time.Duration
	ParallelTopK     int
	LevelThresholds  map[regionhierarchy.Level]float64
	LevelWeights     map[regionhierarchy.Level]float64
	MinCombinedScore float64
	CacheSize        int
}

// applyMatcherDefaults sets sensible defaults on the provided MatcherConfig when fields are zero-valued.
// It populates SnapshotPath, Timeout, ParallelTopK, LevelThresholds, LevelWeights, MinCombinedScore,
// applyMatcherDefaults applies sensible defaults to a MatcherConfig, filling any zero-valued fields.
// It sets SnapshotPath to "data/matcher_snapshot.json", Timeout to 100ms, ParallelTopK to 5,
// per-level thresholds (province: 0.4, city: 0.4, district: 0.45, subdistrict: 0.45) and weights
// (province: 0.2, city: 0.3, district: 0.25, subdistrict: 0.25) where missing, MinCombinedScore to 0.6,
// applyMatcherDefaults populates zero-valued fields on cfg with sane defaults for matcher behaviour.
// 
// It sets a default snapshot path, timeout, parallel top-K, per-level thresholds and weights, minimum
// combined score, and cache size when those fields are not already provided. Existing non-zero values
// on cfg are preserved.
func applyMatcherDefaults(cfg *MatcherConfig) {
	defaultThresholds := map[regionhierarchy.Level]float64{
		regionhierarchy.LevelProvince:    0.4,
		regionhierarchy.LevelCity:        0.4,
		regionhierarchy.LevelDistrict:    0.45,
		regionhierarchy.LevelSubdistrict: 0.45,
	}
	defaultWeights := map[regionhierarchy.Level]float64{
		regionhierarchy.LevelProvince:    0.2,
		regionhierarchy.LevelCity:        0.3,
		regionhierarchy.LevelDistrict:    0.25,
		regionhierarchy.LevelSubdistrict: 0.25,
	}

	if cfg.SnapshotPath == "" {
		cfg.SnapshotPath = filepath.Join("data", "matcher_snapshot.json")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 100 * time.Millisecond
	}
	if cfg.ParallelTopK <= 0 {
		cfg.ParallelTopK = 5
	}

	// Ensure the thresholds map exists, then fill any missing entries
	if len(cfg.LevelThresholds) == 0 {
		cfg.LevelThresholds = make(map[regionhierarchy.Level]float64, len(defaultThresholds))
	}
	for level, threshold := range defaultThresholds {
		if _, ok := cfg.LevelThresholds[level]; !ok {
			cfg.LevelThresholds[level] = threshold
		}
	}

	// Ensure the weights map exists, then fill any missing entries
	if len(cfg.LevelWeights) == 0 {
		cfg.LevelWeights = make(map[regionhierarchy.Level]float64, len(defaultWeights))
	}
	for level, weight := range defaultWeights {
		if _, ok := cfg.LevelWeights[level]; !ok {
			cfg.LevelWeights[level] = weight
		}
	}

	if cfg.MinCombinedScore <= 0 {
		cfg.MinCombinedScore = 0.6
	}
	if cfg.CacheSize <= 0 {
		cfg.CacheSize = 1000
	}
}

// NewLogger constructs a slog.Logger placeholder for subsequent wiring phases.
func NewLogger() (*slog.Logger, error) {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})), nil
}

// NewDuckDB creates and opens a DuckDB SQL connection using opts.DBPath or the
// default path "data/regions.duckdb". If opts.ReadOnly is true the connection
// string is appended with "access_mode=read_only". It returns the opened
// *sql.DB or a wrapped database failure error if the driver fails to open the
// connection.
func NewDuckDB(ctx context.Context, opts Options) (*sql.DB, error) {
	path := opts.DBPath
	if path == "" {
		path = "data/regions.duckdb"
	}

	var builder strings.Builder
	builder.WriteString(path)
	if opts.ReadOnly {
		builder.WriteString("?access_mode=read_only")
	}
	dsn := builder.String()

	connector, err := duckdb.NewConnector(dsn, func(execer driver.ExecerContext) error {
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mandatory := []string{
			"SET scalar_subquery_error_on_multiple_rows=false",
			"LOAD fts",
		}
		for _, stmt := range mandatory {
			if _, execErr := execer.ExecContext(bootstrapCtx, stmt, nil); execErr != nil {
				return fmt.Errorf("duckdb init statement failed (%s): %w", stmt, execErr)
			}
		}
		optional := []string{"LOAD fuzzystrmatch"}
		for _, stmt := range optional {
			if _, execErr := execer.ExecContext(bootstrapCtx, stmt, nil); execErr != nil {
				continue
			}
		}
		return nil
	})
	if err != nil {
		return nil, sharederrors.Wrap(sharederrors.CodeDatabaseFailure, "failed to create duckdb connector", err)
	}

	db := sql.OpenDB(connector)

	maxOpen := opts.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = runtime.NumCPU() * 4
		if maxOpen < 10 {
			maxOpen = 10
		}
	}
	maxIdle := opts.MaxIdleConns
	if maxIdle <= 0 || maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, sharederrors.Wrap(sharederrors.CodeDatabaseFailure, "failed to ping database", err)
	}

	bootstrapCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := warmDuckDBPool(bootstrapCtx, db, maxIdle); err != nil {
		db.Close()
		return nil, sharederrors.Wrap(sharederrors.CodeDatabaseFailure, "failed to warm database pool", err)
	}

	return db, nil
}

func warmDuckDBPool(ctx context.Context, db *sql.DB, target int) error {
	if target <= 0 {
		return nil
	}
	for i := 0; i < target; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "SELECT 1"); err != nil {
			conn.Close()
			return err
		}
		if err := conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// NewFiber creates a new Fiber application instance without any middleware, ready for further configuration.
func NewFiber() (*fiber.App, error) {
	app := fiber.New()
	return app, nil
}

// BootstrapHTTP wires and configures the HTTP server, database connection, logger, optional region matcher, and routing.
//
// It enforces read-only mode on the provided options and applies matcher defaults. If a matcher snapshot is available it
// will attempt to initialise a matcher and inject it into the region use case; if the snapshot is missing or initialisation
// fails the bootstrap proceeds with a nil matcher and logs a warning. The function opens a DuckDB connection, constructs the
// region use case, creates a Fiber app with request logging and recovery middleware, registers region routes, and exposes a
// /healthz endpoint that verifies the database is reachable within a 2-second timeout.
//
// On success it returns an HTTPBootstrap containing the configured App, DB, Logger and the Matcher configuration. An error
// BootstrapHTTP creates and configures the HTTP server, database, logger, and optional region matcher,
// and returns an HTTPBootstrap ready for serving.
// It applies matcher defaults, sets ReadOnly mode on the provided options, and attempts to load and
// initialize a matcher snapshot; matcher initialization failures are logged and do not abort bootstrapping.
// The function opens a DuckDB connection, constructs the region repository and use case (injecting the
// matcher if available), creates a Fiber app with request logging and recovery middleware, registers
// region routes under /v1, and adds a /healthz endpoint that pings the database with a 2-second timeout.
// It returns the configured HTTPBootstrap on success, or an error if logger creation, database
// BootstrapHTTP creates and initializes the HTTP server stack: it enforces read-only mode,
// applies matcher defaults, attempts to load and initialize the region matcher, opens a DuckDB
// connection, constructs the region use case and HTTP routes (including a /healthz endpoint),
// and returns the assembled Fiber app, DB handle, logger, and matcher configuration.
// 
// If matcher snapshot loading or matcher initialization fails the function continues with a nil
// matcher while logging a warning. If database opening, use-case construction, or Fiber creation
// fails the database handle is closed and an error is returned.
func BootstrapHTTP(ctx context.Context, opts Options) (HTTPBootstrap, error) {
	opts.ReadOnly = true
	applyMatcherDefaults(&opts.Matcher)
	logger, err := NewLogger()
	if err != nil {
		return HTTPBootstrap{}, err
	}

	var matcherInstance *regionmatcher.Matcher
	if snapshot, err := ngramcache.LoadSnapshot(opts.Matcher.SnapshotPath); err != nil {
		logger.Warn("matcher snapshot unavailable", "path", opts.Matcher.SnapshotPath, "error", err)
	} else {
		weights := make(map[regionmatcher.Level]float64, len(opts.Matcher.LevelWeights))
		for level, weight := range opts.Matcher.LevelWeights {
			weights[regionmatcher.Level(level)] = weight
		}
		matcherOpts := []regionmatcher.Option{
			regionmatcher.WithParallelTopK(opts.Matcher.ParallelTopK),
			regionmatcher.WithSuggestionTimeout(opts.Matcher.Timeout),
			regionmatcher.WithMinCombinedScore(opts.Matcher.MinCombinedScore),
			regionmatcher.WithCacheSize(opts.Matcher.CacheSize),
			regionmatcher.WithWordComboSize(2),
			regionmatcher.WithNGramSize(3),
		}
		if len(weights) > 0 {
			matcherOpts = append(matcherOpts, regionmatcher.WithPercolatorWeights(weights))
		}
		for level, threshold := range opts.Matcher.LevelThresholds {
			matcherOpts = append(matcherOpts, regionmatcher.WithLevelThreshold(regionmatcher.Level(level), threshold))
		}
		if m, err := regionmatcher.NewMatcher(snapshot.Facets, matcherOpts...); err != nil {
			logger.Warn("matcher initialisation failed", "error", err)
		} else {
			matcherInstance = m
		}
	}

	db, err := NewDuckDB(ctx, opts)
	if err != nil {
		return HTTPBootstrap{}, err
	}

	repo := duckdbrepo.NewRegionRepository(db)
	uc, err := regionusecase.New(ctx, repo, regionusecase.RegionUseCaseOptions{Logger: logger, Matcher: matcherInstance, MatcherMinScore: opts.Matcher.MinCombinedScore})
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

	return HTTPBootstrap{App: app, DB: db, Logger: logger, Matcher: opts.Matcher}, nil
}

// BootstrapWorker creates and wires the dependencies required by the ingestion worker.
// It returns a WorkerBootstrap containing the logger, database connection, ingestion runner,
// use case, and resolved matcher configuration. An error is returned if required initialization
// BootstrapWorker creates and wires the components required to run the ingestion worker and returns a WorkerBootstrap.
// It sets up the logger, opens the DuckDB database, constructs the admin repository, file loader, SQL normalizer,
// BootstrapWorker initializes components required for the ingestion worker and returns a WorkerBootstrap.
// It forces read-only mode to false, applies matcher defaults, creates a logger and DuckDB connection,
// constructs the admin repository, file loader, SQL normalizer, ingestion use case, and the worker runner.
// Returns a non-nil error if any initialization step fails.
func BootstrapWorker(ctx context.Context, opts Options) (WorkerBootstrap, error) {
	opts.ReadOnly = false
	applyMatcherDefaults(&opts.Matcher)
	logger, err := NewLogger()
	if err != nil {
		return WorkerBootstrap{}, err
	}

	db, err := NewDuckDB(ctx, opts)
	if err != nil {
		return WorkerBootstrap{}, err
	}

	adminRepo := duckdbrepo.NewAdminRepository(db)
	loader := filesystem.FileLoader{}
	normalizer := sqlnormalize.MySQLStripper{}
	uc := ingestionusecase.New(loader, normalizer, adminRepo, ingestionusecase.Options{Logger: logger})
	runner := workerdelivery.NewRunner(uc)

	return WorkerBootstrap{
		Logger:  logger,
		DB:      db,
		Runner:  runner,
		UseCase: uc,
		Matcher: opts.Matcher,
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
