package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRuntimeDirsCreatesUploadsAndLibraryDirs(t *testing.T) {
	base := t.TempDir()
	cfg := Config{
		DataDir:    filepath.Join(base, "data"),
		UploadsDir: filepath.Join(base, "runtime", "uploads"),
		LibraryDir: filepath.Join(base, "mounted", "library"),
	}

	if err := EnsureRuntimeDirs(cfg); err != nil {
		t.Fatalf("EnsureRuntimeDirs returned error: %v", err)
	}

	for _, dir := range []string{cfg.DataDir, cfg.UploadsDir, cfg.LibraryDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}
}

func TestLoadDatabaseDefaultsToSQLiteDSN(t *testing.T) {
	t.Setenv("OWL_DB_TYPE", "")
	t.Setenv("OWL_DB_DSN", "")
	base := t.TempDir()
	dbPath := filepath.Join(base, "owl.db")
	t.Setenv("OWL_DB_PATH", dbPath)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseType != "sqlite" {
		t.Fatalf("expected sqlite database type, got %q", cfg.DatabaseType)
	}
	if cfg.DatabaseDriver != "sqlite3" {
		t.Fatalf("expected sqlite3 driver, got %q", cfg.DatabaseDriver)
	}
	expectedDSN := "file:" + dbPath + "?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)"
	if cfg.DatabaseDSN != expectedDSN {
		t.Fatalf("expected generated sqlite DSN %q, got %q", expectedDSN, cfg.DatabaseDSN)
	}
}

func TestLoadDatabaseTypeAndDSN(t *testing.T) {
	t.Setenv("OWL_DB_TYPE", "postgresql")
	t.Setenv("OWL_DB_DSN", "postgres://owl:secret@db:5432/owl?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseType != "postgres" {
		t.Fatalf("expected normalized postgres type, got %q", cfg.DatabaseType)
	}
	if cfg.DatabaseDriver != "postgres" {
		t.Fatalf("expected postgres driver, got %q", cfg.DatabaseDriver)
	}
	if cfg.DatabaseDSN != "postgres://owl:secret@db:5432/owl?sslmode=disable" {
		t.Fatalf("unexpected database DSN %q", cfg.DatabaseDSN)
	}
}
