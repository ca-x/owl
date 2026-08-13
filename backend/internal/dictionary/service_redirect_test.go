package dictionary

import (
	"fmt"
	"testing"

	"github.com/lib-x/mdx"
)

func TestResolveEntryRedirectsUsesStoredEntryAtEveryHop(t *testing.T) {
	entries := map[string]mdx.IndexEntry{
		"second": {Keyword: "second", RecordStartOffset: 20, RecordEndOffset: 29},
		"third":  {Keyword: "third", RecordStartOffset: 30, RecordEndOffset: 39},
	}
	contents := map[int64]string{
		10: "@@@LINK=second",
		20: "@@@LINK=third",
		30: "final definition",
	}
	var resolvedOffsets []int64

	got, err := resolveEntryRedirects(
		mdx.IndexEntry{Keyword: "first", RecordStartOffset: 10, RecordEndOffset: 19},
		0,
		nil,
		func(entry mdx.IndexEntry) ([]byte, error) {
			resolvedOffsets = append(resolvedOffsets, entry.RecordStartOffset)
			content, ok := contents[entry.RecordStartOffset]
			if !ok {
				return nil, fmt.Errorf("unexpected offset %d", entry.RecordStartOffset)
			}
			return []byte(content), nil
		},
		func(word string) (mdx.IndexEntry, error) {
			entry, ok := entries[word]
			if !ok {
				return mdx.IndexEntry{}, mdx.ErrIndexMiss
			}
			return entry, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "final definition" {
		t.Fatalf("got %q, want final definition", got)
	}
	wantOffsets := []int64{10, 20, 30}
	if fmt.Sprint(resolvedOffsets) != fmt.Sprint(wantOffsets) {
		t.Fatalf("resolved offsets %v, want %v", resolvedOffsets, wantOffsets)
	}
}

func TestResolveEntryRedirectsStopsCycle(t *testing.T) {
	entries := map[string]mdx.IndexEntry{
		"first":  {Keyword: "first", RecordStartOffset: 10},
		"second": {Keyword: "second", RecordStartOffset: 20},
	}
	contents := map[int64]string{
		10: "@@@LINK=second",
		20: "@@@LINK=first",
	}
	resolveCalls := 0

	got, err := resolveEntryRedirects(entries["first"], 0, nil,
		func(entry mdx.IndexEntry) ([]byte, error) {
			resolveCalls++
			return []byte(contents[entry.RecordStartOffset]), nil
		},
		func(word string) (mdx.IndexEntry, error) {
			entry, ok := entries[word]
			if !ok {
				return mdx.IndexEntry{}, mdx.ErrIndexMiss
			}
			return entry, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "<p>second</p>" {
		t.Fatalf("got %q, want escaped cycle target", got)
	}
	if resolveCalls != 3 {
		t.Fatalf("got %d resolve calls, want 3", resolveCalls)
	}
}

func TestLookupRedirectEntryFallsBackToComparableKeyword(t *testing.T) {
	store := mdx.NewMemoryIndexStore()
	entry := mdx.IndexEntry{Keyword: "apple", RecordStartOffset: 42}
	if err := store.Put(mdx.DictionaryInfo{Name: "demo"}, []mdx.IndexEntry{entry}); err != nil {
		t.Fatal(err)
	}
	got, err := lookupRedirectEntry(store, "demo", "Apple")
	if err != nil {
		t.Fatal(err)
	}
	if got != entry {
		t.Fatalf("got %+v, want %+v", got, entry)
	}

	punctuated := mdx.IndexEntry{Keyword: "cooperate", RecordStartOffset: 84}
	if err := store.Put(mdx.DictionaryInfo{Name: "punctuation"}, []mdx.IndexEntry{punctuated}); err != nil {
		t.Fatal(err)
	}
	got, err = lookupRedirectEntry(store, "punctuation", "Co-Operate")
	if err != nil {
		t.Fatal(err)
	}
	if got != punctuated {
		t.Fatalf("got %+v, want %+v", got, punctuated)
	}
}
