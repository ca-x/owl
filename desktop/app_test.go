package main

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

//go:embed testdata/*
var dictionaries embed.FS

func testBook(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"Field Notes.mdx", "Field Notes.mdd"} {
		data, err := dictionaries.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "Field Notes.mdx")
}

func TestLocalReader(t *testing.T) {
	path := testBook(t)
	// Native runs restore a previously opened dictionary. The simulated host
	// also exercises the file chooser callback, which is an OS dialog on desktop.
	openPath, err := json.Marshal(map[string]any{"version": 0, "value": path})
	if err != nil {
		t.Fatal(err)
	}
	initial := map[string]string{}
	if nsgotest.Target() != "" {
		initial["mdx-file"] = string(openPath)
	}
	app := nsgotest.Start(t, Root, nsgotest.Size(780, 620), nsgotest.Storage(initial))
	if nsgotest.Target() == "" {
		app.PickFiles = func(core.FilesRequest) []core.File { return []core.File{{Path: path}} }
		app.Press("id:open-mdx")
	}
	app.WaitFor("id:query", 5*time.Second)
	app.Type("id:query", "Folio")
	app.Submit("id:query")
	if got := app.WaitFor("id:headword", 5*time.Second).Text; got != "folio" {
		t.Fatalf("headword: %s", got)
	}
	if got := app.Find("id:definition").Text; !strings.Contains(got, "A leaf of a book") {
		t.Fatalf("definition: %s", got)
	}
	app.WaitFor("id:entry-image", 5*time.Second)
	app.Screenshot("local-entry")
	app.Press("id:copy")
	app.Press("role:button → leaf")
	app.WaitFor("text:A single sheet in a manuscript.", 5*time.Second)
	app.Press("id:server")
	app.Find("id:connect")
}

func TestRemoteReader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /owl/api/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"version":"v0.2.0"}`)) })
	mux.HandleFunc("POST /owl/api/auth/login", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"token":"test-session"}`)) })
	query := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "folio" {
			t.Errorf("query: %s", r.URL.RawQuery)
		}
		if r.URL.Path == "/owl/api/search" && r.Header.Get("Authorization") != "Bearer test-session" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`[{"word":"folio","dictionary_id":1,"dictionary_name":"Notes","html":"<p>A leaf.</p>"}]`))
	}
	mux.HandleFunc("GET /owl/api/public/search", query)
	mux.HandleFunc("GET /owl/api/search", query)
	mux.HandleFunc("GET /owl/api/dictionaries/1/resource/leaf.png", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-session" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte("image"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	for _, user := range []string{"", "reader"} {
		s, err := connect(t.Context(), ts.URL+"/owl", user, user)
		if err != nil {
			t.Fatal(err)
		}
		items, err := s.lookup(t.Context(), "folio")
		if err != nil || len(items) != 1 {
			t.Fatalf("lookup: %v %v", items, err)
		}
		if user != "" {
			b, err := s.asset(t.Context(), items[0], "/api/dictionaries/1/resource/leaf.png")
			if err != nil || string(b) != "image" {
				t.Fatalf("asset: %q %v", b, err)
			}
		}
		for _, ref := range []string{"https://other.example/image", "/api/me", "/api/dictionaries/2/resource/x", "/api/dictionaries/1/resource/%2e%2e/%2e%2e/me"} {
			if _, err := s.asset(t.Context(), items[0], ref); err == nil {
				t.Errorf("unexpected resource: %s", ref)
			}
		}
	}
}

func TestLocalPronunciation(t *testing.T) {
	book, err := openBook(t.Context(), testBook(t))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := book.lookup(t.Context(), "folio")
	if err != nil || len(entries) == 0 {
		t.Fatalf("lookup: %v", err)
	}
	for _, a := range readDefinition(entries[0].HTML).attachments {
		if a.kind == "audio" {
			content, err := book.asset(t.Context(), entries[0], a.ref)
			if err != nil || !strings.HasPrefix(string(content), "RIFF") {
				t.Fatalf("pronunciation: %v", err)
			}
			return
		}
	}
	t.Fatal("missing pronunciation")
}

func TestServerConnectionForm(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"v0.2.0"}`))
	})
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var credentials struct{ Username, Password string }
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil || credentials.Username != "reader" || credentials.Password != "test-pass" {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"token":"session"}`))
	})
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer session" || r.URL.Query().Get("q") != "private" {
			http.Error(w, "not signed in", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`[{"word":"private","dictionary_name":"My notes","html":"Personal entry"}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	app := nsgotest.Start(t, Root, nsgotest.Size(780, 620))
	app.Press("id:server")
	app.Clear("id:address")
	app.Type("id:address", server.URL)
	app.Submit("id:address")
	app.WaitFor("id:query", 5*time.Second)
	app.Press("id:disconnect")
	app.Press("id:use-account")
	app.Type("id:username", "reader")
	app.Type("id:password", "test-pass")
	app.Submit("id:password")
	app.WaitFor("id:query", 5*time.Second)
	app.Type("id:query", "private")
	app.Submit("id:query")
	app.WaitFor("text:Personal entry", 5*time.Second)
	app.Press("id:disconnect")
	app.Find("id:connect")
}
