package dictionary

import (
	"path/filepath"
	"sync/atomic"
	"testing"
)

// TestEnsureMDDsLoadedNoPairsIsIdempotent verifies that lazily loading MDD
// resources is a safe no-op when a dictionary has no paired MDD files, and that
// the work only runs once even across repeated calls.
func TestEnsureMDDsLoadedNoPairsIsIdempotent(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 0, "", "")

	loaded := &LoadedDictionary{
		slug:    "sample",
		mdxPath: filepath.Join(t.TempDir(), "sample.mdx"),
	}

	if err := svc.ensureMDDsLoaded(loaded); err != nil {
		t.Fatalf("unexpected error on first load: %v", err)
	}
	if loaded.MDDs == nil {
		t.Fatal("expected MDDs to be initialized to an empty slice")
	}
	if len(loaded.MDDs) != 0 {
		t.Fatalf("expected no MDDs for an unpaired dictionary, got %d", len(loaded.MDDs))
	}

	// A second call must be a no-op thanks to sync.Once.
	if err := svc.ensureMDDsLoaded(loaded); err != nil {
		t.Fatalf("unexpected error on second load: %v", err)
	}
}

// TestEnsureMDDsLoadedRunsOnce confirms the underlying work executes a single
// time even under concurrent access.
func TestEnsureMDDsLoadedRunsOnce(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 0, "", "")
	loaded := &LoadedDictionary{
		slug:    "sample",
		mdxPath: filepath.Join(t.TempDir(), "sample.mdx"),
	}

	var calls int32
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			loaded.mddOnce.Do(func() {
				atomic.AddInt32(&calls, 1)
			})
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected once-guarded work to run exactly once, ran %d times", got)
	}

	// The service helper shares the same Once, so it should not re-run the body.
	if err := svc.ensureMDDsLoaded(loaded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.MDDs != nil {
		t.Fatal("expected helper to skip work after the Once already fired")
	}
}
