package main

import (
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
)

type attachment struct{ kind, ref, label string }
type definition struct {
	text        string
	attachments []attachment
	related     []string
}

// Read dictionary markup as text and media using native controls. Inline
// scripts, stylesheet text and remote navigation are outside the reading flow.
func readDefinition(markup string) definition {
	doc, _ := html.Parse(strings.NewReader(markup))
	var result definition
	var text strings.Builder
	space, line := false, false
	addText := func(s string) {
		for _, r := range s {
			switch r {
			case ' ', '\n', '\t', '\r':
				space = true
			default:
				if text.Len() > 0 {
					if line {
						text.WriteByte('\n')
					} else if space {
						text.WriteByte(' ')
					}
				}
				line, space = false, false
				text.WriteRune(r)
			}
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			addText(n.Data)
			return
		}
		tag := n.Data
		switch tag {
		case "head", "script", "style", "template", "noscript":
			return
		}
		block := false
		switch tag {
		case "p", "div", "li", "br", "tr", "dt", "dd", "h1", "h2", "h3", "def", "blockquote":
			block = true
			line = true
		case "td", "th":
			space = true
		}
		var src, href, alt string
		for _, a := range n.Attr {
			switch a.Key {
			case "src":
				src = a.Val
			case "href":
				href = a.Val
			case "alt":
				alt = a.Val
			}
		}
		switch {
		case tag == "img" && src != "":
			result.attachments = append(result.attachments, attachment{"image", src, alt})
		case (tag == "audio" || tag == "source" && n.Parent != nil && n.Parent.Data == "audio") && src != "":
			result.attachments = append(result.attachments, attachment{"audio", src, ""})
		case tag == "a" && href != "":
			lower := strings.ToLower(href)
			if strings.HasPrefix(lower, "sound://") || strings.HasPrefix(lower, "snd://") {
				result.attachments = append(result.attachments, attachment{"audio", href, ""})
			} else if u, err := url.Parse(href); err == nil {
				if isAudioResource(u.Path) {
					result.attachments = append(result.attachments, attachment{"audio", href, ""})
				} else if u.Scheme == "entry" || u.Scheme == "mdxentry" || u.Scheme == "dict" {
					word, _ := url.PathUnescape(u.Host + u.EscapedPath())
					if word != "" {
						result.related = append(result.related, word)
					}
				} else if u.Path == "/search" && u.Query().Get("q") != "" {
					result.related = append(result.related, u.Query().Get("q"))
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if block {
			line = true
		}
	}
	if doc != nil {
		walk(doc)
	}
	result.text = text.String()
	return result
}

func isAudioResource(resource string) bool {
	_, ref, ok := strings.Cut(strings.ToLower(resource), "/resource/")
	if !ok {
		return false
	}
	if strings.HasPrefix(ref, "snd:") || strings.HasPrefix(ref, "sound:") {
		return true
	}
	switch path.Ext(ref) {
	case ".mp3", ".wav", ".ogg", ".spx", ".m4a", ".aac", ".flac":
		return true
	}
	return false
}
