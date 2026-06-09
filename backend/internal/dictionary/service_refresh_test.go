package dictionary

import (
	"testing"
	"time"

	"owl/backend/internal/models"
)

func TestLibraryRefreshLockSerializesConcurrentRefreshes(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 0, "", "")

	firstInside := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan struct{})
	secondInside := make(chan struct{})
	secondDone := make(chan struct{})

	go func() {
		defer close(firstDone)
		_, _ = svc.withLibraryRefreshLock(func() (report *models.MaintenanceReport, err error) {
			close(firstInside)
			<-releaseFirst
			return nil, nil
		})
	}()

	select {
	case <-firstInside:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not enter critical section")
	}

	go func() {
		defer close(secondDone)
		_, _ = svc.withLibraryRefreshLock(func() (report *models.MaintenanceReport, err error) {
			close(secondInside)
			return nil, nil
		})
	}()

	select {
	case <-secondInside:
		t.Fatal("second refresh entered while first refresh still held the lock")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not finish")
	}
	select {
	case <-secondInside:
	case <-time.After(time.Second):
		t.Fatal("second refresh did not enter after first refresh finished")
	}
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second refresh did not finish")
	}
}

func TestLoadedDictionaryCacheEvictsLeastRecentlyUsed(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 2, "", "")

	svc.mu.Lock()
	first := &LoadedDictionary{}
	second := &LoadedDictionary{}
	third := &LoadedDictionary{}
	svc.cacheLoadedLocked(1, first)
	svc.cacheLoadedLocked(2, second)
	svc.touchLoadedLocked(1)
	svc.cacheLoadedLocked(3, third)
	svc.mu.Unlock()

	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if _, ok := svc.loaded[2]; ok {
		t.Fatal("expected dictionary 2 to be evicted")
	}
	if svc.loaded[1] != first {
		t.Fatal("expected dictionary 1 to remain cached")
	}
	if svc.loaded[3] != third {
		t.Fatal("expected dictionary 3 to remain cached")
	}
}

func TestLoadedDictionaryCacheCanBeUnlimited(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 0, "", "")

	svc.mu.Lock()
	svc.cacheLoadedLocked(1, &LoadedDictionary{})
	svc.cacheLoadedLocked(2, &LoadedDictionary{})
	svc.cacheLoadedLocked(3, &LoadedDictionary{})
	svc.mu.Unlock()

	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if len(svc.loaded) != 3 {
		t.Fatalf("expected unlimited cache to retain all dictionaries, got %d", len(svc.loaded))
	}
}
