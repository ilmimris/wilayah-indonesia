package ngramcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"
)

func TestBuildSnapshotFromWilayah(t *testing.T) {
	sql := "INSERT INTO wilayah (kode, nama)\nVALUES\n('11','Aceh'),\n('11.01','Kota Test'),\n('11.01.01','Kecamatan Test'),\n('11.01.01.2001','Desa Test');\n"

	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "wilayah.sql")
	if err := os.WriteFile(sqlPath, []byte(sql), 0o644); err != nil {
		t.Fatalf("write sql: %v", err)
	}

	snapshot, err := BuildSnapshotFromWilayah(sqlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Metadata.SourcePath != sqlPath {
		t.Fatalf("expected source path %q, got %q", sqlPath, snapshot.Metadata.SourcePath)
	}
	if snapshot.Metadata.DatasetHash == "" {
		t.Fatalf("expected dataset hash")
	}
	if len(snapshot.Facets) != 1 {
		t.Fatalf("expected one facet, got %d", len(snapshot.Facets))
	}
	facet := snapshot.Facets[0]
	if facet.RegionID != "11.01.01.2001" {
		t.Fatalf("unexpected region id %s", facet.RegionID)
	}
	if facet.Province != "Aceh" || facet.City != "Kota Test" || facet.District != "Kecamatan Test" || facet.Subdistrict != "Desa Test" {
		t.Fatalf("unexpected facet data: %+v", facet)
	}
	if snapshot.Metadata.RecordCounts[regionhierarchy.LevelProvince] != 1 {
		t.Fatalf("expected province count 1")
	}
}

func TestBuildSnapshotMissingParent(t *testing.T) {
	sql := "INSERT INTO wilayah (kode, nama)\nVALUES\n('11','Aceh'),\n('11.01','Kota Test'),\n('11.01.01.2001','Desa Test');\n"
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "wilayah.sql")
	if err := os.WriteFile(sqlPath, []byte(sql), 0o644); err != nil {
		t.Fatalf("write sql: %v", err)
	}

	if _, err := BuildSnapshotFromWilayah(sqlPath); err == nil {
		t.Fatalf("expected error for missing district")
	}
}

func TestWriteSnapshot(t *testing.T) {
	snap := Snapshot{Metadata: Metadata{SourcePath: "wilayah.sql"}}

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "snapshot.json")
	if err := WriteSnapshot(snap, dest); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected snapshot file: %v", err)
	}
}

func TestLoadSnapshot(t *testing.T) {
	snap := Snapshot{Metadata: Metadata{SourcePath: "wilayah.sql", DatasetHash: "abc"}}
	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "snapshot.json")
	if err := WriteSnapshot(snap, dest); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	loaded, err := LoadSnapshot(dest)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if loaded.Metadata.DatasetHash != snap.Metadata.DatasetHash {
		t.Fatalf("expected dataset hash %s, got %s", snap.Metadata.DatasetHash, loaded.Metadata.DatasetHash)
	}
}
