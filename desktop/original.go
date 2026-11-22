package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/lib-x/mdx"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const layoutOrigin = "owl-entry://book"

// The system web view serves an immutable copy of one entry. Resolving assets
// off the UI thread keeps the platform's synchronous scheme callback cheap.
type layoutBuilder struct {
	item  entry
	keys  map[string]string
	paths []string
	files map[string][]byte
}

func (b *layoutBuilder) link(parent, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(strings.ToLower(ref), "data:") {
		return ref
	}
	u, err := url.Parse(strings.ReplaceAll(ref, "\\", "/"))
	if err != nil {
		return ""
	}
	var key string
	switch u.Scheme {
	case "":
		if u.Host != "" {
			return ""
		}
		key = u.Path
		if strings.HasPrefix(key, "/api/") {
			for _, prefix := range []string{"/api/dictionaries/", "/api/public/dictionaries/"} {
				if rem, ok := strings.CutPrefix(key, prefix+strconv.Itoa(b.item.ID)+"/resource/"); ok {
					key = rem
					parent = ""
					break
				}
			}
			if strings.HasPrefix(key, "/api/") {
				return ""
			}
		}
	case "sound", "snd", "css", "img", "file":
		key = u.Host + u.Path
		parent = ""
	default:
		return ""
	}
	// MDX schemes may also be wrapped in the server's resource URL.
	if scheme, rest, ok := strings.Cut(key, "://"); ok {
		switch strings.ToLower(scheme) {
		case "sound", "snd", "css", "img", "file":
			key = rest
		default:
			return ""
		}
	}
	key = strings.ReplaceAll(key, "\\", "/")
	if !strings.HasPrefix(key, "/") {
		key = path.Join(path.Dir(parent), key)
	}
	key = strings.TrimPrefix(path.Clean("/"+key), "/")
	name, exists := b.keys[key]
	if !exists {
		if key == "" || len(b.paths) >= 256 {
			return ""
		}
		name = fmt.Sprintf("/media/%d%s", len(b.paths), path.Ext(key))
		b.paths = append(b.paths, key)
		b.keys[key] = name
	}
	return (&url.URL{Scheme: "owl-entry", Host: "book", Path: name, Fragment: u.Fragment}).String()
}

func (b *layoutBuilder) stylesheet(data []byte, parent string) []byte {
	lexer := css.NewLexer(parse.NewInputBytes(data))
	var out bytes.Buffer
	afterImport := false
	for {
		kind, token := lexer.Next()
		if kind == css.ErrorToken {
			break
		}
		value := string(token)
		switch {
		case kind == css.URLToken:
			_, raw, _ := strings.Cut(value, "(")
			ref := strings.Trim(strings.TrimSpace(strings.TrimSuffix(raw, ")")), "\"'")
			fmt.Fprintf(&out, "url(%q)", b.link(parent, ref))
		case afterImport && kind == css.StringToken:
			fmt.Fprintf(&out, "%q", b.link(parent, strings.Trim(value, "\"'")))
		default:
			out.Write(token)
		}
		if kind != css.WhitespaceToken && kind != css.CommentToken {
			afterImport = kind == css.AtKeywordToken && strings.EqualFold(value, "@import")
		}
	}
	return out.Bytes()
}

func buildOriginal(ctx context.Context, s source, item entry) (map[string][]byte, error) {
	b := layoutBuilder{item: item, keys: make(map[string]string), files: make(map[string][]byte)}
	root, err := html.Parse(strings.NewReader(item.HTML))
	if err != nil {
		return nil, err
	}
	var head *html.Node
	var audioSources []*html.Attribute
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "head":
				head = n
			case "a":
				href := attribute(n, "href")
				if u, err := url.Parse(href); err == nil && (u.Scheme == "sound" || u.Scheme == "snd" || isAudioResource(u.Path)) {
					n.Data, n.DataAtom = "audio", atom.Audio
					n.Attr = []html.Attribute{{Key: "controls"}, {Key: "src", Val: href}, {Key: "preload", Val: "none"}}
				}
			case "script", "base", "iframe", "frame", "object", "embed", "meta":
				n.Parent.RemoveChild(n)
				return
			case "style":
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						c.Data = string(b.stylesheet([]byte(c.Data), ""))
					}
				}
			}
			attrs := n.Attr[:0]
			for _, a := range n.Attr {
				switch {
				case strings.HasPrefix(a.Key, "on"), a.Key == "srcset", a.Key == "action", a.Key == "target":
					continue
				case a.Key == "style":
					a.Val = string(b.stylesheet([]byte(a.Val), ""))
				case a.Key == "src" || a.Key == "poster" || (n.Data == "link" && a.Key == "href"):
					a.Val = b.link("", a.Val)
				case n.Data == "a" && a.Key == "href":
					if word := linkedWord(a.Val); word != "" {
						a.Val = layoutOrigin + "/lookup?q=" + url.QueryEscape(word)
					} else if !strings.HasPrefix(a.Val, "#") {
						continue
					}
				}
				attrs = append(attrs, a)
			}
			n.Attr = attrs
			if n.Data == "audio" || n.Data == "source" && n.Parent.Data == "audio" {
				for i := range n.Attr {
					if n.Attr[i].Key == "src" {
						audioSources = append(audioSources, &n.Attr[i])
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			walk(c)
			c = next
		}
	}
	walk(root)
	if head != nil {
		policy := "default-src 'none'; style-src owl-entry: 'unsafe-inline'; img-src owl-entry: data:; font-src owl-entry: data:; media-src owl-entry: data:; form-action 'none'; base-uri 'none'"
		meta := &html.Node{Type: html.ElementNode, Data: "meta", DataAtom: atom.Meta, Attr: []html.Attribute{{Key: "http-equiv", Val: "Content-Security-Policy"}, {Key: "content", Val: policy}}}
		style := &html.Node{Type: html.ElementNode, Data: "style", DataAtom: atom.Style}
		style.AppendChild(&html.Node{Type: html.TextNode, Data: "html{color-scheme:light}body{margin:16px;color:#302e28;background:#fffefa;font:17px/1.55 Georgia,serif;overflow-wrap:break-word}img,audio,video{max-width:100%}a{color:#763b32}"})
		head.InsertBefore(style, head.FirstChild)
		head.InsertBefore(meta, style)
	}
	b.files["/lookup"] = []byte("<!doctype html><title>Owl</title>")
	size := len(item.HTML)
	for i := 0; i < len(b.paths); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := b.paths[i]
		ref := key
		if remote, ok := s.(*remoteServer); ok {
			ref = mdx.AssetURL("/"+remote.endpoint(fmt.Sprintf("dictionaries/%d/resource", item.ID)), key)
		}
		data, err := s.asset(ctx, item, ref)
		if err != nil {
			continue
		}
		size += len(data)
		if size > 64<<20 {
			return nil, fmt.Errorf("dictionary layout exceeds 64 MiB")
		}
		if strings.EqualFold(path.Ext(key), ".css") {
			data = b.stylesheet(data, key)
		}
		b.files[b.keys[key]] = data
	}
	// WebKit's media process cannot read an app-owned URL scheme on every
	// platform. A data URL gives its player the same pronunciation bytes.
	for _, attr := range audioSources {
		name, ok := strings.CutPrefix(attr.Val, layoutOrigin)
		data := b.files[name]
		if !ok || len(data) == 0 {
			continue
		}
		size += base64.StdEncoding.EncodedLen(len(data))
		if size > 64<<20 {
			return nil, fmt.Errorf("dictionary layout exceeds 64 MiB")
		}
		kind := mime.TypeByExtension(path.Ext(name))
		if kind == "" {
			kind = http.DetectContentType(data)
		}
		attr.Val = "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	var doc bytes.Buffer
	if err := html.Render(&doc, root); err != nil {
		return nil, err
	}
	b.files["/index.html"] = doc.Bytes()
	return b.files, ctx.Err()
}

func originalView(cx *core.Scope, d *readingDesk, item entry) core.Node {
	files := core.NewResource(cx, nil, func(ctx context.Context, _ int, _ core.ResourceInfo) (map[string][]byte, error) {
		return buildOriginal(ctx, d.source, item)
	})
	return core.Dynamic(func(cx *core.Scope) core.Node {
		if err := files.Err(); err != nil {
			return mutedText(cx, err.Error())
		}
		pages, ok := files.Get()
		if !ok {
			return mutedText(cx, d.tr("正在读取原版排版…", "Loading dictionary layout…"))
		}
		err := cx.ServeScheme("owl-entry", func(raw string) []byte {
			u, err := url.Parse(raw)
			if err != nil || u.Host != "book" {
				return nil
			}
			return pages[u.Path]
		})
		if err != nil {
			return mutedText(cx, err.Error())
		}
		documentURL := layoutOrigin + "/index.html"
		loaded := core.NewSignal(cx, false)
		message := core.NewSignal(cx, d.tr("正在打开原版排版…", "Opening original layout…"))
		cx.AfterFunc(6*time.Second, func() {
			if !loaded.Get() {
				message.Set(d.tr("如果页面未显示，请切回阅读排版。Linux 的原版排版需要 WebKitGTK 6。", "If the page stays blank, choose Reader layout. Original layout on Linux requires WebKitGTK 6."))
			}
		})
		followed := false
		view := core.WebView{Source: core.WebSrc(documentURL), Style: paneStyle, NoJavaScript: true, Incognito: true, PinchZoom: true, Label: d.tr("词典原版排版", "Original dictionary layout"), TestID: "original-page",
			OnLoad: func(nav core.WebNavigation) {
				if nav.URL == documentURL {
					loaded.Set(true)
				}
			},
			OnError: func(err error) {
				loaded.Set(false)
				message.Set(d.tr("原版排版未能打开，可切回阅读排版。", "Could not open the original layout. Choose Reader layout to continue.") + " " + err.Error())
			},
			OnNavigationStateChange: func(nav core.WebNavigation) {
				u, err := url.Parse(nav.URL)
				if err == nil && !followed && u.Scheme == "owl-entry" && u.Host == "book" && u.Path == "/lookup" {
					if word := strings.TrimSpace(u.Query().Get("q")); word != "" {
						followed = true
						d.search(word)
					}
				}
			},
		}
		return core.View{Style: paneStyle.Merge(core.Style{Gap: 0}), Children: []core.Node{
			core.Show{When: func() bool { return !loaded.Get() }, Then: func(*core.Scope) core.Node {
				return core.Text{ContentFunc: message.Get, Style: core.Style{Padding: core.Symmetric(8, 16)}, TestID: "layout-status", Role: core.RoleStatus}
			}}, view,
		}}
	})
}
