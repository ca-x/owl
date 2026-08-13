package dictionary

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/lib-x/mdx"
	"github.com/redis/go-redis/v9"
)

func TestManagedDictionaryIndexStoreForwardsHealthCheck(t *testing.T) {
	prefix := &healthManagedIndexStore{ManagedIndexStore: mdx.NewMemoryIndexStore(), healthy: true}
	store := newManagedDictionaryIndexStore(prefix, nil)
	ready, err := store.HasDictionaryIndex("My Dict")
	if err != nil {
		t.Fatal(err)
	}
	if !ready || prefix.checkedName != "my-dict" {
		t.Fatalf("unexpected health result: ready=%v name=%q", ready, prefix.checkedName)
	}
}

func TestManagedDictionaryIndexStoreHealthCheckFallback(t *testing.T) {
	store := newManagedDictionaryIndexStore(managedStoreWithoutHealth{ManagedIndexStore: mdx.NewMemoryIndexStore()}, nil)
	ready, err := store.HasDictionaryIndex("My Dict")
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("stores without optional health support should remain reusable")
	}
}

func TestManagedDictionaryIndexStoreForwardsHealthError(t *testing.T) {
	wantErr := errors.New("health failed")
	prefix := &healthManagedIndexStore{ManagedIndexStore: mdx.NewMemoryIndexStore(), err: wantErr}
	store := newManagedDictionaryIndexStore(prefix, nil)
	if _, err := store.HasDictionaryIndex("My Dict"); !errors.Is(err, wantErr) {
		t.Fatalf("expected health error, got %v", err)
	}
}

func TestManagedDictionaryIndexStoreForwardsBuildLease(t *testing.T) {
	prefix := &leaseManagedIndexStore{ManagedIndexStore: mdx.NewMemoryIndexStore()}
	store := newManagedDictionaryIndexStore(prefix, nil)
	release, acquired, err := store.AcquireIndexBuildLease("My Dict", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired || release == nil {
		t.Fatalf("unexpected lease result: acquired=%v release=%v", acquired, release != nil)
	}
	if prefix.acquiredName != "my-dict" || prefix.acquiredTTL != time.Minute {
		t.Fatalf("unexpected lease forwarding: name=%q ttl=%v", prefix.acquiredName, prefix.acquiredTTL)
	}
	if err := release(); err != nil || !prefix.released {
		t.Fatalf("release was not forwarded: released=%v err=%v", prefix.released, err)
	}
}

func TestManagedDictionaryIndexStoreLeaseFallback(t *testing.T) {
	store := newManagedDictionaryIndexStore(managedStoreWithoutHealth{ManagedIndexStore: mdx.NewMemoryIndexStore()}, nil)
	release, acquired, err := store.AcquireIndexBuildLease("demo", time.Minute)
	if err != nil || !acquired || release == nil {
		t.Fatalf("stores without optional lease support should proceed: acquired=%v release=%v err=%v", acquired, release != nil, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}

func TestManagedDictionaryIndexStoreSanitizesManifestName(t *testing.T) {
	prefix := mdx.NewMemoryIndexStore()
	store := newManagedDictionaryIndexStore(prefix, nil)
	if store == nil {
		t.Fatal("expected managed store")
	}
	if err := store.SaveManifest(mdx.IndexManifest{DictionaryName: "My Dict"}); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}
	manifest, err := prefix.LoadManifest("my-dict")
	if err != nil {
		t.Fatalf("expected sanitized manifest key: %v", err)
	}
	if manifest.DictionaryName != "my-dict" {
		t.Fatalf("expected sanitized dictionary name, got %q", manifest.DictionaryName)
	}
}

func TestManagedDictionaryIndexStoreReusesMatchingManifestWithoutRebuild(t *testing.T) {
	path := writeInvalidDictionaryFile(t, "My Dict *.mdx")
	prefix := &countingManagedIndexStore{ManagedIndexStore: mdx.NewMemoryIndexStore()}
	store := newManagedDictionaryIndexStore(prefix, nil)
	if store == nil {
		t.Fatal("expected managed store")
	}

	manifest, err := mdx.BuildIndexManifest(path, "")
	if err != nil {
		t.Fatalf("BuildIndexManifest failed: %v", err)
	}
	if err := store.SaveManifest(manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	result, err := mdx.EnsureDictionaryIndex(path, store, mdx.WithReuseIfUnchanged(true))
	if err != nil {
		t.Fatalf("EnsureDictionaryIndex should reuse the manifest without opening invalid MDX: %v", err)
	}
	if result == nil || !result.Reused || result.Rebuilt {
		t.Fatalf("expected reused result without rebuild, got %#v", result)
	}
	if prefix.putCalls != 0 {
		t.Fatalf("expected no index Put on reuse, got %d calls", prefix.putCalls)
	}
}

func TestRedisPrefixFuzzyStoreSearchesPrefixIndex(t *testing.T) {
	prefix := mdx.NewMemoryIndexStore()
	store := redisPrefixFuzzyStore{store: prefix}
	info := mdx.DictionaryInfo{Name: "demo"}
	entries := []mdx.IndexEntry{{Keyword: "apple"}, {Keyword: "apricot"}, {Keyword: "banana"}}
	if err := store.Put(info, entries); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	hits, err := store.Search("demo", "ap", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected two prefix hits, got %d: %#v", len(hits), hits)
	}
	for _, hit := range hits {
		if hit.Source != "redis-prefix" {
			t.Fatalf("expected redis-prefix source, got %q", hit.Source)
		}
	}
}

type countingManagedIndexStore struct {
	mdx.ManagedIndexStore
	putCalls int
}

type healthManagedIndexStore struct {
	mdx.ManagedIndexStore
	healthy     bool
	err         error
	checkedName string
}

func (s *healthManagedIndexStore) HasDictionaryIndex(dictionaryName string) (bool, error) {
	s.checkedName = dictionaryName
	return s.healthy, s.err
}

type managedStoreWithoutHealth struct {
	mdx.ManagedIndexStore
}

type leaseManagedIndexStore struct {
	mdx.ManagedIndexStore
	acquiredName string
	acquiredTTL  time.Duration
	released     bool
}

func (s *leaseManagedIndexStore) AcquireIndexBuildLease(dictionaryName string, ttl time.Duration) (func() error, bool, error) {
	s.acquiredName = dictionaryName
	s.acquiredTTL = ttl
	return func() error {
		s.released = true
		return nil
	}, true, nil
}

func (s *countingManagedIndexStore) Put(info mdx.DictionaryInfo, entries []mdx.IndexEntry) error {
	s.putCalls++
	return s.ManagedIndexStore.Put(info, entries)
}

func writeInvalidDictionaryFile(t *testing.T, pattern string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	if _, err := file.WriteString("not a real mdx file"); err != nil {
		_ = file.Close()
		t.Fatalf("WriteString failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	return file.Name()
}

func TestFuzzyBackendNameReportsRedisPrefixFallback(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 0, "", "")
	loaded := &LoadedDictionary{FuzzyStore: redisPrefixFuzzyStore{store: mdx.NewMemoryIndexStore()}}
	if got := svc.fuzzyBackendName(loaded); got != "redis-prefix" {
		t.Fatalf("expected redis-prefix backend, got %q", got)
	}
}

func TestFuzzyBackendNameReportsPrefixAfterRediSearchBecomesUnavailable(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	svc := NewService(nil, "", "", client, "", 0, "", true, 0, "", "")
	loaded := &LoadedDictionary{
		FuzzyStore:        newRedisSearchStore(client, "owl:test", "demo"),
		PrefixStore:       mdx.NewMemoryIndexStore(),
		RediSearchEnabled: true,
	}
	if got := svc.fuzzyBackendName(loaded); got != "redisearch" {
		t.Fatalf("expected redisearch before capability failure, got %q", got)
	}
	svc.markRediSearchUnavailable(errors.New("ERR unknown command 'FT.SEARCH'"))
	if got := svc.fuzzyBackendName(loaded); got != "redis-prefix" {
		t.Fatalf("expected prefix fallback after capability failure, got %q", got)
	}
}

type missingFuzzyStore struct{}

func (missingFuzzyStore) Put(mdx.DictionaryInfo, []mdx.IndexEntry) error { return nil }
func (missingFuzzyStore) Search(string, string, int) ([]mdx.SearchHit, error) {
	return nil, mdx.ErrIndexMiss
}

func TestSearchIndexHitsFallsBackToPrefixWhenFuzzyMisses(t *testing.T) {
	prefix := mdx.NewMemoryIndexStore()
	info := mdx.DictionaryInfo{Name: "demo"}
	entry := mdx.IndexEntry{Keyword: "apple", RecordStartOffset: 10, RecordEndOffset: 20}
	if err := prefix.Put(info, []mdx.IndexEntry{entry}); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	loaded := &LoadedDictionary{
		FuzzyStore:  missingFuzzyStore{},
		PrefixStore: prefix,
		Info:        info,
	}

	hits, err := searchIndexHits(loaded, "demo", "app", 10)
	if err != nil {
		t.Fatalf("searchIndexHits failed: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected one prefix fallback hit, got %d: %#v", len(hits), hits)
	}
	if hits[0].Entry.Keyword != "apple" || hits[0].Source != "redis-prefix" {
		t.Fatalf("unexpected fallback hit: %#v", hits[0])
	}
}
