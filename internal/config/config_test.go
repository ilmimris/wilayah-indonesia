package config

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewDuckDBConfiguresConnectionPool(t *testing.T) {
	dbPath := filepath.Join("..", "..", "data", "regions.duckdb")
	db, err := NewDuckDB(context.Background(), Options{ReadOnly: true, DBPath: dbPath, MaxOpenConns: 12, MaxIdleConns: 6})
	if err != nil {
		t.Skipf("skipping DuckDB pool test: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	})

	stats := db.Stats()
	if stats.MaxOpenConnections != 12 {
		t.Fatalf("expected MaxOpenConnections=12, got %d", stats.MaxOpenConnections)
	}
	if stats.OpenConnections == 0 {
		t.Fatalf("expected warm-up to open connections, got %d", stats.OpenConnections)
	}
}
