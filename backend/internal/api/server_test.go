package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"owl/backend/ent"
	_ "github.com/lib-x/entsqlite"
)

func TestHealthEndpoint(t *testing.T) {
	client, err := ent.Open("sqlite3", "file:health?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := New(client, nil, nil, "*", true)
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
