package main

import (
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/base/textview"
	"golang.org/x/net/html"
)

type attachment struct{ kind, ref, label string }

type definition struct {
	document textview.Document
	related  []string
	audio    []string
}

// Give the native rich-text reader a pronunciation control at the original
// position of each sound link. All other blocks and spans keep their order.
func readDefinition(markup string) definition {
	root, _ := html.Parse(strings.NewReader(markup))
	var d definition
	if root == nil {
		return d
	}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var sound string
			switch n.Data {
			case "script", "style", "head":
				return
			case "audio":
				sound = attribute(n, "src")
				for c := n.FirstChild; sound == "" && c != nil; c = c.NextSibling {
					if c.Data == "source" {
						sound = attribute(c, "src")
					}
				}
			case "a":
				href := attribute(n, "href")
				u, err := url.Parse(href)
				if err == nil && (u.Scheme == "sound" || u.Scheme == "snd" || isAudioResource(u.Path)) {
					sound = href
				} else if word := linkedWord(href); word != "" && !slices.Contains(d.related, word) {
					d.related = append(d.related, word)
				}
			}
			if sound != "" {
				d.audio = append(d.audio, sound)
				n.Data, n.DataAtom = "owl-pronunciation", 0
				n.Attr = []html.Attribute{{Key: "src", Val: sound}}
				n.FirstChild, n.LastChild = nil, nil
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(root)
	var body strings.Builder
	_ = html.Render(&body, root)
	d.document = textview.ParseHTML(body.String(), textview.Extension{Name: "pronunciation", Tag: "owl-pronunciation"})
	return d
}

func attribute(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func linkedWord(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	switch u.Scheme {
	case "entry", "mdxentry", "dict":
		word, _ := url.PathUnescape(u.Host + u.EscapedPath())
		return strings.TrimSpace(word)
	case "":
		if u.Path == "/search" {
			return strings.TrimSpace(u.Query().Get("q"))
		}
	}
	return ""
}

func isAudioResource(resource string) bool {
	ref := strings.ToLower(resource)
	if _, suffix, ok := strings.Cut(ref, "/resource/"); ok {
		ref = suffix
	}
	if strings.HasPrefix(ref, "snd:") || strings.HasPrefix(ref, "sound:") {
		return true
	}
	switch path.Ext(ref) {
	case ".mp3", ".wav", ".ogg", ".spx", ".snd", ".m4a", ".aac", ".flac":
		return true
	}
	return false
}
