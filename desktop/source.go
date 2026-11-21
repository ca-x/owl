package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lib-x/mdx"
)

type entry struct {
	Word       string `json:"word"`
	Dictionary string `json:"dictionary_name"`
	ID         int    `json:"dictionary_id"`
	HTML       string `json:"html"`
}

// Both sources feed the same native reader. Resource references stay scoped to
// the dictionary that returned the entry, including when a server requires login.
type source interface {
	lookup(context.Context, string) ([]entry, error)
	asset(context.Context, entry, string) ([]byte, error)
}

type headword struct{ folded, word string }

type localBook struct {
	dict   *mdx.Mdict
	words  []headword
	assets func() (*mdx.AssetResolver, error)
}

func openBook(ctx context.Context, path string) (*localBook, error) {
	if !strings.EqualFold(filepath.Ext(path), ".mdx") {
		return nil, errors.New("expected an .mdx file")
	}
	dict, err := mdx.New(path)
	if err != nil {
		return nil, err
	}
	if err := dict.BuildIndex(); err != nil {
		return nil, err
	}
	keys, err := dict.GetKeyWordEntries()
	if err != nil {
		return nil, err
	}
	words := make([]headword, 0, len(keys))
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		words = append(words, headword{strings.ToLower(key.KeyWord), key.KeyWord})
	}
	slices.SortStableFunc(words, func(a, b headword) int { return strings.Compare(a.folded, b.folded) })
	specs, err := mdx.ScanDirectory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	var companions []string
	for _, spec := range specs {
		if filepath.Clean(spec.MDXPath) == filepath.Clean(path) {
			companions = spec.MDDPaths
			break
		}
	}
	return &localBook{dict: dict, words: words, assets: sync.OnceValues(func() (*mdx.AssetResolver, error) {
		var volumes []*mdx.Mdict
		for _, name := range companions {
			vol, err := mdx.New(name)
			if err != nil {
				return nil, err
			}
			if err := vol.BuildIndex(); err != nil {
				return nil, err
			}
			volumes = append(volumes, vol)
		}
		return mdx.NewAssetResolver(nil, mdx.WithAssetMdicts(volumes...)), nil
	})}, nil
}

func (b *localBook) lookup(ctx context.Context, query string) ([]entry, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	var matches []string
	exact, ok := b.dict.FindExactEntry(query)
	if !ok {
		exact, ok = b.dict.FindComparableEntry(query)
	}
	if ok {
		matches = append(matches, exact.KeyWord)
	}
	prefix := strings.ToLower(query)
	start := sort.Search(len(b.words), func(i int) bool { return b.words[i].folded >= prefix })
	for _, k := range b.words[start:] {
		if len(matches) == 10 || !strings.HasPrefix(k.folded, prefix) {
			break
		}
		if !slices.Contains(matches, k.word) {
			matches = append(matches, k.word)
		}
	}
	results := make([]entry, 0, len(matches))
	for _, word := range matches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		html, err := b.dict.Lookup(word)
		if err != nil {
			return nil, err
		}
		results = append(results, entry{Word: word, Dictionary: cmp.Or(b.dict.Title(), b.dict.Name()), HTML: string(html)})
	}
	return results, nil
}

func (b *localBook) asset(ctx context.Context, _ entry, ref string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	assets, err := b.assets()
	if err != nil {
		return nil, err
	}
	return assets.Read(ref)
}

type remoteServer struct {
	base   *url.URL
	client *http.Client
	token  string
}

func connect(ctx context.Context, address, username, password string) (*remoteServer, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(address), "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("expected an http or https server URL")
	}
	s := &remoteServer{base: base, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if (username == "") != (password == "") {
		return nil, errors.New("both username and password are required")
	}
	if username == "" {
		var info struct{ Version string }
		if err := s.json(ctx, "GET", "api/health", nil, nil, &info); err != nil {
			return nil, err
		}
		if info.Version == "" {
			return nil, errors.New("Owl version missing from server response")
		}
	} else {
		var auth struct{ Token string }
		body, err := json.Marshal(map[string]string{"username": username, "password": password})
		if err != nil {
			return nil, err
		}
		if err := s.json(ctx, "POST", "api/auth/login", nil, body, &auth); err != nil {
			return nil, err
		}
		if auth.Token == "" {
			return nil, errors.New("missing session token")
		}
		s.token = auth.Token
	}
	return s, nil
}

func (s *remoteServer) get(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, error) {
	u := s.base.JoinPath(path)
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Owl HTTP %d · %s", res.StatusCode, http.StatusText(res.StatusCode))
	}
	const limit = 64 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if len(data) > limit {
		return nil, errors.New("entry or media exceeds 64 MiB")
	}
	return data, err
}

func (s *remoteServer) json(ctx context.Context, method, path string, query url.Values, body []byte, dst any) error {
	data, err := s.get(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func (s *remoteServer) lookup(ctx context.Context, word string) ([]entry, error) {
	path := "api/public/search"
	if s.token != "" {
		path = "api/search"
	}
	var entries []entry
	err := s.json(ctx, "GET", path, url.Values{"q": {word}}, nil, &entries)
	return entries, err
}

func (s *remoteServer) asset(ctx context.Context, item entry, ref string) ([]byte, error) {
	u, err := url.Parse(ref)
	if err != nil || u.IsAbs() || u.Host != "" {
		return nil, os.ErrPermission
	}
	path := strings.TrimPrefix(u.Path, "/")
	allowed := fmt.Sprintf("dictionaries/%d/resource/", item.ID)
	if !strings.HasPrefix(path, "api/"+allowed) && !strings.HasPrefix(path, "api/public/"+allowed) {
		return nil, os.ErrPermission
	}
	if slices.Contains(strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }), "..") {
		return nil, os.ErrPermission
	}
	return s.get(ctx, "GET", strings.TrimPrefix(u.EscapedPath(), "/"), u.Query(), nil)
}
