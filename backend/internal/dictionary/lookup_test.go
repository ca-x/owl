package dictionary

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"owl/backend/ent"

	_ "github.com/lib-x/entsqlite"
)

func TestResolveAccessibleDictionaryIDHonorsVisibilityAndEnabledState(t *testing.T) {
	client, ownerID, otherUserID := newLookupTestClient(t, 0)
	ctx := t.Context()

	owned, err := client.Dictionary.Create().
		SetName("owned-dictionary").
		SetTitle("Owned Dictionary").
		SetSlug("owned-dictionary").
		SetMdxPath(filepath.Join(t.TempDir(), "owned.mdx")).
		SetOwnerID(ownerID).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	public, err := client.Dictionary.Create().
		SetName("public-dictionary").
		SetTitle("Public Dictionary").
		SetSlug("public-dictionary").
		SetMdxPath(filepath.Join(t.TempDir(), "public.mdx")).
		SetOwnerID(otherUserID).
		SetPublic(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unicode, err := client.Dictionary.Create().
		SetName("Kelvin").
		SetTitle("A Unicode Fold").
		SetSlug("unicode-fold").
		SetMdxPath(filepath.Join(t.TempDir(), "unicode.mdx")).
		SetOwnerID(otherUserID).
		SetPublic(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Dictionary.Create().
		SetName("kelvin").
		SetTitle("Z ASCII Fold").
		SetSlug("ascii-fold").
		SetMdxPath(filepath.Join(t.TempDir(), "ascii.mdx")).
		SetOwnerID(otherUserID).
		SetPublic(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Dictionary.Create().
		SetName("disabled-dictionary").
		SetTitle("Disabled Dictionary").
		SetSlug("disabled-dictionary").
		SetMdxPath(filepath.Join(t.TempDir(), "disabled.mdx")).
		SetOwnerID(ownerID).
		SetEnabled(false).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Dictionary.Create().
		SetName("private-dictionary").
		SetTitle("Private Dictionary").
		SetSlug("private-dictionary").
		SetMdxPath(filepath.Join(t.TempDir(), "private.mdx")).
		SetOwnerID(otherUserID).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewService(client, "", "", nil, "", 0, "", false, 0, "", "")
	tests := []struct {
		name  string
		query string
		want  int
		found bool
	}{
		{name: "owned by internal name", query: " OWNED-DICTIONARY ", want: owned.ID, found: true},
		{name: "public by title", query: "public dictionary", want: public.ID, found: true},
		{name: "unicode simple case fold", query: "kelvin", want: unicode.ID, found: true},
		{name: "disabled", query: "disabled-dictionary"},
		{name: "other private", query: "private-dictionary"},
		{name: "empty", query: "  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := svc.ResolveAccessibleDictionaryID(t.Context(), ownerID, tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want || found != tt.found {
				t.Fatalf("got (%d, %t), want (%d, %t)", got, found, tt.want, tt.found)
			}
		})
	}
}

func BenchmarkResolveAccessibleDictionaryID256(b *testing.B) {
	client, ownerID, _ := newLookupTestClient(b, 256)
	svc := NewService(client, "", "", nil, "", 0, "", false, 0, "", "")
	const target = "Dictionary 255"

	b.Run("full_list_and_match", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			items, err := svc.ListAccessible(b.Context(), ownerID)
			if err != nil {
				b.Fatal(err)
			}
			found := 0
			for _, item := range items {
				if strings.EqualFold(item.Name, target) || strings.EqualFold(item.Title, target) {
					found = item.ID
					break
				}
			}
			if found == 0 {
				b.Fatal("target dictionary not found")
			}
		}
	})

	b.Run("select_target_id", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			id, found, err := svc.ResolveAccessibleDictionaryID(b.Context(), ownerID, target)
			if err != nil {
				b.Fatal(err)
			}
			if !found || id == 0 {
				b.Fatal("target dictionary not found")
			}
		}
	})
}

func newLookupTestClient(tb testing.TB, dictionaryCount int) (*ent.Client, int, int) {
	tb.Helper()
	databasePath := filepath.Join(tb.TempDir(), "lookup.db")
	client, err := ent.Open("sqlite3", "file:"+databasePath+"?_pragma=foreign_keys(1)")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = client.Close() })
	ctx := tb.Context()
	if err := client.Schema.Create(ctx); err != nil {
		tb.Fatal(err)
	}
	owner, err := client.User.Create().
		SetUsername("owner").
		SetDisplayName("Owner").
		SetPasswordHash("test").
		Save(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	other, err := client.User.Create().
		SetUsername("other").
		SetDisplayName("Other").
		SetPasswordHash("test").
		Save(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	mdxPath := filepath.Join(tb.TempDir(), "dictionary.mdx")
	for i := range dictionaryCount {
		name := fmt.Sprintf("Dictionary %03d", i)
		_, err := client.Dictionary.Create().
			SetName(name).
			SetTitle(name).
			SetSlug(fmt.Sprintf("dictionary-%03d", i)).
			SetMdxPath(mdxPath).
			SetOwnerID(owner.ID).
			Save(ctx)
		if err != nil {
			tb.Fatal(err)
		}
	}
	return client, owner.ID, other.ID
}
