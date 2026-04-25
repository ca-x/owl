package user

import (
	"context"
	"testing"

	_ "github.com/lib-x/entsqlite"
	"owl/backend/ent"
	"owl/backend/internal/models"
)

func TestUpdatePreferencesNormalizesRecentSearchLimit(t *testing.T) {
	ctx := context.Background()
	client, err := ent.Open("sqlite3", "file:user_preferences?mode=memory&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}

	svc := NewService(client, "secret", t.TempDir())
	u, err := client.User.Create().SetUsername("alice").SetDisplayName("Alice").SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	prefs, err := svc.GetPreferences(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prefs.RecentSearchLimit != 8 {
		t.Fatalf("expected default recent search limit 8, got %d", prefs.RecentSearchLimit)
	}

	prefs, err = svc.UpdatePreferences(ctx, u.ID, models.UserPreferences{Language: "zh-CN", Theme: "system", FontMode: "sans", RecentSearchLimit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if prefs.RecentSearchLimit != 20 {
		t.Fatalf("expected upper clamp to 20, got %d", prefs.RecentSearchLimit)
	}

	prefs, err = svc.UpdatePreferences(ctx, u.ID, models.UserPreferences{Language: "zh-CN", Theme: "system", FontMode: "sans", RecentSearchLimit: -1})
	if err != nil {
		t.Fatal(err)
	}
	if prefs.RecentSearchLimit != 0 {
		t.Fatalf("expected lower clamp to 0, got %d", prefs.RecentSearchLimit)
	}
}
