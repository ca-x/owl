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
