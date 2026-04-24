package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib-x/entsqlite"
	"owl/backend/ent"
	"owl/backend/internal/user"
)

func TestHealthEndpoint(t *testing.T) {
	client, err := ent.Open("sqlite3", "file:health?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := New(client, nil, nil, nil, "*")
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	res := httptest.NewRecorder()
	server.echo.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(res.Body.String())).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if _, ok := body["version"]; !ok {
		t.Fatalf("expected version field in response: %#v", body)
	}
	_ = server.Shutdown(context.Background())
}

func TestPublicFontsExposeSharedFontMetadataAndFile(t *testing.T) {
	ctx := context.Background()
	client, err := ent.Open("sqlite3", "file:public_fonts?mode=memory&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	fontDir := t.TempDir()
	fontPath := filepath.Join(fontDir, "Demo Font.woff2")
	fontBytes := []byte("demo-font")
	if err := os.WriteFile(fontPath, fontBytes, 0o644); err != nil {
		t.Fatalf("write font: %v", err)
	}
	if _, err := client.Font.Create().
		SetName("Demo Font.woff2").
		SetFamily("Demo Font").
		SetPath(fontPath).
		SetMime("font/woff2").
		Save(ctx); err != nil {
		t.Fatalf("create font: %v", err)
	}

	server := New(client, user.NewService(client, "test-secret", t.TempDir()), nil, nil, "*")
	defer func() { _ = server.Shutdown(ctx) }()

	listReq := httptest.NewRequest(http.MethodGet, "/api/public/fonts", nil)
	listRes := httptest.NewRecorder()
	server.echo.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRes.Code, listRes.Body.String())
	}
	var fonts []map[string]string
	if err := json.NewDecoder(strings.NewReader(listRes.Body.String())).Decode(&fonts); err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 1 {
		t.Fatalf("expected one font, got %#v", fonts)
	}
	if fonts[0]["name"] != "Demo Font.woff2" || fonts[0]["family"] != "Demo Font" {
		t.Fatalf("unexpected font metadata: %#v", fonts[0])
	}
	if fonts[0]["url"] != "/api/public/fonts/Demo%20Font.woff2" {
		t.Fatalf("expected escaped public font URL, got %q", fonts[0]["url"])
	}

	fileReq := httptest.NewRequest(http.MethodGet, fonts[0]["url"], nil)
	fileRes := httptest.NewRecorder()
	server.echo.ServeHTTP(fileRes, fileReq)
	if fileRes.Code != http.StatusOK {
		t.Fatalf("expected file 200, got %d: %s", fileRes.Code, fileRes.Body.String())
	}
	if got := fileRes.Body.String(); got != string(fontBytes) {
		t.Fatalf("unexpected font body %q", got)
	}
	if got := fileRes.Header().Get("Content-Type"); got != "font/woff2" {
		t.Fatalf("unexpected content type %q", got)
	}
}
