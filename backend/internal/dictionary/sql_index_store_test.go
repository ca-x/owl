package dictionary

import (
	"context"
	"testing"
	"time"

	"owl/backend/ent"

	_ "github.com/lib-x/entsqlite"
	"github.com/lib-x/mdx"
)

func TestSQLDictionaryIndexStore(t *testing.T) {
	ctx := context.Background()
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
}
