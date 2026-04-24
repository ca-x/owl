package settings

import (
	"context"
	"path/filepath"
	"testing"

	"owl/backend/internal/models"
)

func TestSystemSettingsFooterContentDefaultsHiddenAndPersists(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	svc, err := NewService(dataDir, models.SystemSettings{AllowRegister: true})
	if err != nil {
		t.Fatal(err)
	}
	initial := svc.Get(ctx)
	if !initial.AllowRegister || initial.FooterExtra != "" || initial.Copyright != "" {
		t.Fatalf("unexpected default settings: %#v", initial)
	}

	updated, err := svc.Update(ctx, models.SystemSettings{AllowRegister: false, FooterExtra: "  Extra footer  ", Copyright: " © Owl "})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AllowRegister || updated.FooterExtra != "Extra footer" || updated.Copyright != "© Owl" {
		t.Fatalf("unexpected updated settings: %#v", updated)
	}

	reloaded, err := NewService(filepath.Dir(filepath.Join(dataDir, fileName)), models.SystemSettings{AllowRegister: true})
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.Get(ctx)
	if got.AllowRegister || got.FooterExtra != "Extra footer" || got.Copyright != "© Owl" {
		t.Fatalf("unexpected reloaded settings: %#v", got)
	}
}
