package dictionary

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"owl/backend/ent"
	entindexentry "owl/backend/ent/dictionaryindexentry"

	_ "github.com/lib-x/entsqlite"
	"github.com/lib-x/mdx"
)

var benchmarkSQLIndexEntries []mdx.IndexEntry

func BenchmarkSQLDictionaryIndexStoreRead(b *testing.B) {
	client, err := ent.Open("sqlite3", "file:"+b.TempDir()+"/index.db?_pragma=foreign_keys(1)")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(b.Context()); err != nil {
		b.Fatal(err)
	}

	store := newSQLDictionaryIndexStore(b.Context(), client)
	entries := make([]mdx.IndexEntry, 256)
	for i := range entries {
		entries[i] = mdx.IndexEntry{
			Keyword:           fmt.Sprintf("entry-%04d", i),
			NormalizedKeyword: fmt.Sprintf(`\\entry-%04d`, i),
			RecordStartOffset: int64(i * 100),
			RecordEndOffset:   int64(i*100 + 99),
			KeyBlockIdx:       int64(i / 16),
			IsResource:        i%2 == 0,
		}
	}
	if err := store.Put(mdx.DictionaryInfo{Name: "benchmark"}, entries); err != nil {
		b.Fatal(err)
	}

	b.Run("PrefixSearch10", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			got, err := store.PrefixSearch("benchmark", "entry-00", 10)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkSQLIndexEntries = got
		}
	})
	b.Run("Search10", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			got, err := store.Search("benchmark", "entry-0001", 10)
			if err != nil {
				b.Fatal(err)
			}
			if len(got) == 0 {
				b.Fatal("expected search results")
			}
			benchmarkSQLIndexEntries = []mdx.IndexEntry{got[0].Entry}
		}
	})
	b.Run("GetComparable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			got, err := store.GetComparable("benchmark", "Entry 0001")
			if err != nil {
				b.Fatal(err)
			}
			benchmarkSQLIndexEntries = []mdx.IndexEntry{got}
		}
	})
}

func TestSQLDictionaryIndexStore(t *testing.T) {
	ctx := t.Context()
	client, err := ent.Open("sqlite3", "file:sql_dictionary_index?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}

	store := newSQLDictionaryIndexStore(ctx, client)
	info := mdx.DictionaryInfo{Name: "Oxford English"}
	entries := []mdx.IndexEntry{
		{Keyword: "apple", RecordStartOffset: 1, RecordEndOffset: 2, KeyBlockIdx: 3},
		{Keyword: "application", RecordStartOffset: 4, RecordEndOffset: 5, KeyBlockIdx: 6},
		{Keyword: "pineapple", RecordStartOffset: 7, RecordEndOffset: 8, KeyBlockIdx: 9},
		{Keyword: `audio/file.mp3`, NormalizedKeyword: `\\audio\\file.mp3`, RecordStartOffset: 10, RecordEndOffset: 11, KeyBlockIdx: 12, IsResource: true},
	}
	if err := store.Put(info, entries); err != nil {
		t.Fatal(err)
	}

	exact, err := store.GetExact("oxford-english", "apple")
	if err != nil {
		t.Fatal(err)
	}
	if exact.Keyword != "apple" || exact.KeyBlockIdx != 3 {
		t.Fatalf("unexpected exact entry: %+v", exact)
	}
	resource, err := store.GetExact("oxford-english", `\\audio\\file.mp3`)
	if err != nil {
		t.Fatal(err)
	}
	if resource.Keyword != `audio/file.mp3` || resource.NormalizedKeyword != `\\audio\\file.mp3` || !resource.IsResource {
		t.Fatalf("resource columns did not round trip: %+v", resource)
	}

	prefix, err := store.PrefixSearch("oxford-english", "app", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefix) != 2 {
		t.Fatalf("expected 2 prefix entries, got %d", len(prefix))
	}

	hits, err := store.Search("oxford-english", "apple", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected exact and contains hits, got %d", len(hits))
	}
	if hits[0].Entry.Keyword != "apple" || hits[0].Source != "sql-exact" {
		t.Fatalf("unexpected first hit: %+v", hits[0])
	}

	manifest := mdx.IndexManifest{
		DictionaryName: "Oxford English",
		SourcePath:     "/dicts/oxford.mdx",
		Fingerprint:    "stat:1",
		SchemaVersion:  "v1",
		BuiltAt:        time.Now().UTC(),
	}
	if err := store.SaveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadManifest("oxford-english")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Fingerprint != "stat:1" {
		t.Fatalf("unexpected manifest: %+v", loaded)
	}
	ready, err := store.HasDictionaryIndex("Oxford English")
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("expected complete SQL index to be healthy")
	}
}

func TestSQLDictionaryIndexStoreStoresMinimalCompatibilityPayload(t *testing.T) {
	store, client := newTestSQLDictionaryIndexStore(t, "sql_dictionary_index_minimal_payload")
	entry := mdx.IndexEntry{
		Keyword:           "ability",
		NormalizedKeyword: "ability",
		RecordStartOffset: 123,
		RecordEndOffset:   456,
		KeyBlockIdx:       7,
	}
	if err := store.Put(mdx.DictionaryInfo{Name: "minimal"}, []mdx.IndexEntry{entry}); err != nil {
		t.Fatal(err)
	}
	rows, err := client.DictionaryIndexEntry.Query().
		Where(entindexentry.DictionaryNameEQ("minimal")).
		All(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want entry, comparable entry, and sentinel", len(rows))
	}
	payloads := make(map[string]int)
	for _, row := range rows {
		payloads[row.Payload]++
	}
	if payloads[sqlIndexEntryPayload] != 1 || payloads[sqlIndexComparablePayloadPrefix+"ability"] != 1 || payloads[sqlIndexSentinelPayload] != 1 {
		t.Fatalf("unexpected SQL index payload layout: %#v", payloads)
	}
	got, err := store.GetExact("minimal", "ability")
	if err != nil {
		t.Fatal(err)
	}
	if got != entry {
		t.Fatalf("typed columns did not round trip: got %+v, want %+v", got, entry)
	}
}

func TestSQLDictionaryIndexStoreComparableLookupMatchesMDXFirstWins(t *testing.T) {
	store, _ := newTestSQLDictionaryIndexStore(t, "sql_dictionary_index_comparable")
	first := mdx.IndexEntry{Keyword: "Co-Operate", RecordStartOffset: 10, RecordEndOffset: 19, KeyBlockIdx: 1}
	second := mdx.IndexEntry{Keyword: "cooperate", RecordStartOffset: 20, RecordEndOffset: 29, KeyBlockIdx: 2}
	punctuated := mdx.IndexEntry{Keyword: `A: B.C_D-'E"(F)#G<H>!`, RecordStartOffset: 30, RecordEndOffset: 39, KeyBlockIdx: 3}
	entries := []mdx.IndexEntry{first, second, punctuated}
	if err := store.Put(mdx.DictionaryInfo{Name: "comparable"}, entries); err != nil {
		t.Fatal(err)
	}

	memory := mdx.NewMemoryIndexStore()
	if err := memory.Put(mdx.DictionaryInfo{Name: "comparable"}, entries); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{" cooperate ", "CO.OPERATE", "a b.c_d-'e\"(f)#g<h>!"} {
		want, err := memory.GetComparable("comparable", query)
		if err != nil {
			t.Fatalf("memory comparable lookup %q: %v", query, err)
		}
		got, err := store.GetComparable("comparable", query)
		if err != nil {
			t.Fatalf("SQL comparable lookup %q: %v", query, err)
		}
		if got != want {
			t.Fatalf("SQL comparable lookup %q got %+v, want mdx semantics %+v", query, got, want)
		}
	}

	got, err := store.GetComparable("comparable", "cooperate")
	if err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Fatalf("comparable collision did not preserve first entry: got %+v, want %+v", got, first)
	}
	if _, err := store.GetComparable("comparable", "---"); !errors.Is(err, mdx.ErrIndexMiss) {
		t.Fatalf("empty comparable key returned %v, want ErrIndexMiss", err)
	}
}

func TestLookupRedirectEntryUsesSQLComparableIndex(t *testing.T) {
	store, _ := newTestSQLDictionaryIndexStore(t, "sql_dictionary_index_redirect")
	want := mdx.IndexEntry{Keyword: "cooperate", RecordStartOffset: 84, RecordEndOffset: 99, KeyBlockIdx: 4}
	if err := store.Put(mdx.DictionaryInfo{Name: "redirect"}, []mdx.IndexEntry{want}); err != nil {
		t.Fatal(err)
	}

	got, err := lookupRedirectEntry(store, "redirect", "Co-Operate")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("SQL redirect lookup got %+v, want %+v", got, want)
	}
}

func TestSQLDictionaryIndexStoreHealthDetectsMissingData(t *testing.T) {
	store, client := newTestSQLDictionaryIndexStore(t, "sql_dictionary_index_health")
	info := mdx.DictionaryInfo{Name: "health"}
	entries := []mdx.IndexEntry{{Keyword: "one"}, {Keyword: "two"}}
	if err := store.Put(info, entries); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DictionaryIndexEntry.Delete().
		Where(
			entindexentry.DictionaryNameEQ("health"),
			entindexentry.LookupKeyEQ("one"),
		).
		Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	ready, err := store.HasDictionaryIndex("health")
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("expected a partially deleted SQL index to be unhealthy")
	}
}

func TestSQLDictionaryIndexStoreHealthAcceptsEmptyIndex(t *testing.T) {
	store, _ := newTestSQLDictionaryIndexStore(t, "sql_dictionary_index_empty_health")
	if err := store.Put(mdx.DictionaryInfo{Name: "empty"}, nil); err != nil {
		t.Fatal(err)
	}
	ready, err := store.HasDictionaryIndex("empty")
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("expected a valid empty SQL index to be healthy")
	}
	if _, err := store.PrefixSearch("empty", "", 10); !errors.Is(err, mdx.ErrIndexMiss) {
		t.Fatalf("expected sentinel to remain invisible to prefix search, got %v", err)
	}
	if _, err := store.GetExact("empty", sqlIndexSentinelLookupKey); !errors.Is(err, mdx.ErrIndexMiss) {
		t.Fatalf("expected sentinel to remain invisible to exact search, got %v", err)
	}
	if _, err := store.Search("empty", sqlIndexSentinelLookupKey, 10); !errors.Is(err, mdx.ErrIndexMiss) {
		t.Fatalf("expected sentinel to remain invisible to fuzzy search, got %v", err)
	}
}

func TestSQLDictionaryIndexStoreHealthRejectsPreComparableLayout(t *testing.T) {
	store, client := newTestSQLDictionaryIndexStore(t, "sql_dictionary_index_old_layout")
	if err := store.Put(mdx.DictionaryInfo{Name: "old-layout"}, []mdx.IndexEntry{{Keyword: "one"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DictionaryIndexEntry.Update().
		Where(
			entindexentry.DictionaryNameEQ("old-layout"),
			entindexentry.LookupKeyEQ(sqlIndexSentinelLookupKey),
		).
		SetPayload(sqlIndexEntryPayload).
		Save(t.Context()); err != nil {
		t.Fatal(err)
	}
	ready, err := store.HasDictionaryIndex("old-layout")
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("pre-comparable SQL index layout must be rebuilt")
	}
}

func newTestSQLDictionaryIndexStore(t testing.TB, databaseName string) (*sqlDictionaryIndexStore, *ent.Client) {
	t.Helper()
	client, err := ent.Open("sqlite3", "file:"+databaseName+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}
	return newSQLDictionaryIndexStore(ctx, client), client
}
