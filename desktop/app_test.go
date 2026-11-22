package main

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	app := nsgotest.Start(t, Root, nsgotest.Size(1010, 800), nsgotest.Storage(initial))
	if nsgotest.Target() == "" {
		app.PickFiles = func(core.FilesRequest) []core.File { return []core.File{{Path: path}} }
		app.Press("id:open-mdx")
	}
	app.WaitFor("id:query", 5*time.Second)
	app.Type("id:query", "Fo")
	app.WaitFor("id:suggestion-0", 5*time.Second)
	app.Press("id:suggestion-0")
	if got := app.WaitFor("id:headword", 5*time.Second).Text; got != "folio" {
		t.Fatalf("headword: %s", got)
	}
	app.WaitFor("text:A leaf of a book, with writing on either side.", 5*time.Second)
	app.WaitFor("id:entry-image", 5*time.Second)
	app.Screenshot("local-entry")
	if os.Getenv("OWL_TEST_WEBVIEW") == "1" && nsgotest.Target() != "" {
		app.Press("id:layout")
		app.WaitFor("id:original-page", 10*time.Second)
		app.WaitForGone("id:layout-status", 20*time.Second)
		// WindowServer may still be swapping in the browser layer when the
		// load completes. Save a painted frame from the platform compositor.
		deadline := time.Now().Add(3 * time.Second)
		for {
			frame := app.Capture()
			_, _, _, alpha := frame.At(frame.Bounds().Dx()/2, 32).RGBA()
			if alpha > 0 {
				app.Screenshot("original-entry")
				break
			}
			if time.Now().After(deadline) {
				t.Log("compositor returned transparent frames for the original layout")
				break
			}
			time.Sleep(80 * time.Millisecond)
		}
		app.Press("id:layout")
		app.WaitFor("id:definition", 5*time.Second)
	}
	app.Press("id:copy")
	app.Press("id:match-1")
	app.WaitFor("text:Numbered by leaves.", 5*time.Second)
	app.Press("id:match-0")
	app.Press("id:related-0")
	app.WaitFor("text:A single sheet in a manuscript.", 5*time.Second)
	app.Press("id:back")
	app.WaitFor("text:A leaf of a book, with writing on either side.", 5*time.Second)
	app.Press("id:forward")
	app.WaitFor("text:A single sheet in a manuscript.", 5*time.Second)
	app.Press("id:history")
	app.WaitFor("id:recent-1", 5*time.Second)
	app.Press("id:recent-1")
	app.WaitFor("text:A leaf of a book, with writing on either side.", 5*time.Second)
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
		items, err := s.lookup(t.Context(), "folio", 0)
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
	entries, err := book.lookup(t.Context(), "folio", 0)
	if err != nil || len(entries) == 0 {
		t.Fatalf("lookup: %v", err)
	}
	for _, ref := range readDefinition(entries[0].HTML).audio {
		content, err := book.asset(t.Context(), entries[0], ref)
		if err != nil || !strings.HasPrefix(string(content), "RIFF") {
			t.Fatalf("pronunciation: %v", err)
		}
		return
	}
	t.Fatal("missing pronunciation")
}

func TestPrivateOriginalResources(t *testing.T) {
	assets := map[string]string{
		"styles/book.css":  `@import "type.css"; .leaf{background:url("/api/dictionaries/7/resource/image.png")}`,
		"styles/type.css":  `@font-face{font-family:notes;src:url("../fonts/book.woff2")}`,
		"fonts/book.woff2": "sample font",
		"image.png":        "sample image",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"token":"private-test-session"}`))
	})
	mux.HandleFunc("GET /api/dictionaries/7/resource/{name...}", func(w http.ResponseWriter, r *http.Request) {
		content, ok := assets[r.PathValue("name")]
		if !ok || r.Header.Get("Authorization") != "Bearer private-test-session" {
			t.Errorf("unexpected resource request for %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(content))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	s, err := connect(t.Context(), ts.URL, "reader", "test-pass")
	if err != nil {
		t.Fatal(err)
	}
	item := entry{ID: 7, HTML: `<link rel="stylesheet" href="/api/dictionaries/7/resource/styles%2Fbook.css"><img src="/api/dictionaries/8/resource/other.png"><p class="leaf">Folio</p>`}
	files, err := buildOriginal(t.Context(), s, item)
	if err != nil || len(files) != len(assets)+2 {
		t.Fatalf("expected entry, navigation and %d dictionary resources, got %d files (%v)", len(assets), len(files), err)
	}
	for _, data := range files {
		if strings.Contains(string(data), ts.URL) || strings.Contains(string(data), "/api/dictionaries") || strings.Contains(string(data), "private-test-session") {
			t.Fatal("original layout retained a server URL or credential")
		}
	}
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
		if r.Header.Get("Authorization") != "Bearer session" || r.URL.Query().Get("q") != "private" || r.URL.Query().Get("dict") != "7" {
			http.Error(w, "not signed in", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`[{"word":"private","dictionary_name":"My notes","html":"Personal entry"}]`))
	})
	mux.HandleFunc("GET /api/suggest", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"word":"private","sources":[{"dictionary_id":7,"dictionary_name":"My notes"}]}]`))
	})
	mux.HandleFunc("GET /api/dictionaries", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"id":7,"name":"My notes","enabled":true}]`))
	})
	mux.HandleFunc("GET /api/public/dictionaries", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"id":9,"name":"Shared notes","enabled":true}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	app := nsgotest.Start(t, Root, nsgotest.Size(780, 620))
	app.Screenshot("open-local")
	app.Press("id:server")
	app.Clear("id:address")
	app.Type("id:address", "invalid")
	app.Submit("id:address")
	app.WaitUntil(5*time.Second, func() bool {
		return strings.Contains(app.Find("id:connection-status").Text, "https://owl.example.org")
	})
	app.Screenshot("connection-error")
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
	app.Type("id:query", "pri")
	app.WaitFor("id:suggestion-0-source-0", 5*time.Second)
	app.Press("id:suggestion-0-source-0")
	app.WaitFor("text:Personal entry", 5*time.Second)
	app.Press("id:disconnect")
	app.Find("id:connect")
}

func TestServerWorkspace(t *testing.T) {
	if nsgotest.Target() == "" || os.Getenv("OWL_TEST_WEBVIEW") != "1" {
		t.Skip("requires a desktop web view")
	}
	loaded := make(chan bool, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"token":"workspace-test-session"}`))
	})
	mux.HandleFunc("GET /workspace", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer workspace-test-session" {
			http.Error(w, "workspace session missing", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "owl_token", Value: "workspace-test-session", Path: "/", HttpOnly: true})
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><title>Owl library</title><img src="/library-cover.png" alt="Library cover">`))
	})
	mux.HandleFunc("GET /library-cover.png", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("owl_token")
		select {
		case loaded <- err == nil && cookie.Value == "workspace-test-session":
		default:
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(owlIcon)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	reader := nsgotest.Start(t, Root, nsgotest.Size(880, 720))
	reader.Press("id:server")
	reader.Clear("id:address")
	reader.Type("id:address", server.URL)
	reader.Press("id:use-account")
	reader.Type("id:username", "reader")
	reader.Type("id:password", "test-pass")
	reader.Submit("id:password")
	reader.WaitFor("id:workspace", 5*time.Second)
	reader.Press("id:workspace")
	select {
	case ok := <-loaded:
		if !ok {
			t.Fatal("workspace request lost its session cookie")
		}
	case <-time.After(16 * time.Second):
		t.Fatal("server workspace did not load")
	}
	reader.Press("id:close-workspace")
	reader.Find("id:query")
}

func TestCompactDarkReader(t *testing.T) {
	path, err := json.Marshal(testBook(t))
	if err != nil {
		t.Fatal(err)
	}
	app := nsgotest.Start(t, Root, nsgotest.Size(490, 650), nsgotest.Storage(map[string]string{
		"mdx-file":   `{"version":0,"value":` + string(path) + "}",
		"appearance": `{"version":0,"value":"dark"}`,
	}))
	app.WaitFor("id:query", 5*time.Second)
	app.Type("id:query", "folio")
	app.Submit("id:query")
	app.WaitFor("id:matches", 5*time.Second)
	app.WaitFor("id:entry-image", 5*time.Second)
	app.Screenshot("compact-dark")
}

func TestOriginalDictionaryLayout(t *testing.T) {
	book, err := openBook(t.Context(), testBook(t))
	if err != nil {
		t.Fatal(err)
	}
	items, err := book.lookup(t.Context(), "folio", 0)
	if err != nil || len(items) == 0 {
		t.Fatalf("lookup: %v", err)
	}
	files, err := buildOriginal(t.Context(), book, items[0])
	if err != nil {
		t.Fatal(err)
	}
	if doc := string(files["/index.html"]); !strings.Contains(doc, layoutOrigin+"/lookup?q=leaf") || !strings.Contains(doc, "Content-Security-Policy") || !strings.Contains(doc, "<audio") {
		t.Fatal("missing original layout navigation or embedded controls")
	}
	var style, imported, image, audio bool
	for name, data := range files {
		switch filepath.Ext(name) {
		case ".css":
			style = style || strings.Contains(string(data), `@import "owl-entry://book/media/`)
			imported = imported || strings.Contains(string(data), "font-family:Georgia")
		case ".png":
			image = len(data) > 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n"
		case ".wav":
			audio = strings.HasPrefix(string(data), "RIFF")
		}
	}
	if !style || !imported || !image || !audio {
		t.Fatalf("original assets: CSS %v, import %v, image %v, audio %v", style, imported, image, audio)
	}
	if nsgotest.Target() == "" || os.Getenv("OWL_TEST_WEBVIEW") != "1" {
		return
	}
	// Inspect the fixture in the platform engine. WebView2 requires scripting
	// enabled for the test probe. The actual reader exercises its restricted
	// web view in TestLocalReader.
	const inspect = `(async () => {
		if (document.readyState !== "complete") {
			await new Promise(resolve => window.addEventListener("load", resolve, {once: true}));
		}
		const picture = document.querySelector("img");
		await picture.decode();
		await document.fonts.ready;
		await new Promise(requestAnimationFrame);
		const ink = getComputedStyle(document.querySelector(".pronunciation")).color;
		const face = getComputedStyle(document.querySelector("article")).fontFamily;
		window.nsgo.postMessage([ink, face, picture.naturalWidth].join("|"));
	})()`
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		result := core.NewSignal(cx, "")
		if err := cx.ServeScheme("owl-entry", func(raw string) []byte {
			u, err := url.Parse(raw)
			if err != nil || u.Host != "book" {
				return nil
			}
			return files[u.Path]
		}); err != nil {
			t.Error(err)
		}
		return core.View{Style: paneStyle, Children: []core.Node{
			core.Text{ContentFunc: result.Get, TestID: "style-result"},
			core.WebView{Source: core.WebSrc(layoutOrigin + "/index.html"), Style: paneStyle, Incognito: true,
				InjectedJavaScript: inspect, OnMessage: result.Set},
		}}
	}, nsgotest.Size(600, 440))
	app.WaitFor("text:rgb(129, 63, 52)|Georgia, serif|320", 12*time.Second)
}
