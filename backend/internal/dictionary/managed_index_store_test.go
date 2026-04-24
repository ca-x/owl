package dictionary

import (
	"os"
	"testing"

	"github.com/lib-x/mdx"
)

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
	loaded := &LoadedDictionary{FuzzyStore: redisPrefixFuzzyStore{store: mdx.NewMemoryIndexStore()}}
	if got := fuzzyBackendName(loaded); got != "redis-prefix" {
		t.Fatalf("expected redis-prefix backend, got %q", got)
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
