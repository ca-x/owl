package dictionary

import (
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
