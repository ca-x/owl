package dictionary

import "github.com/lib-x/mdx"

type redisPrefixFuzzyStore struct {
	store  mdx.IndexStore
	source string
}

func (s redisPrefixFuzzyStore) Put(info mdx.DictionaryInfo, entries []mdx.IndexEntry) error {
	if s.store == nil {
		return mdx.ErrIndexMiss
	}
	return s.store.Put(info, entries)
}

func (s redisPrefixFuzzyStore) Search(dictionaryName, query string, limit int) ([]mdx.SearchHit, error) {
	if s.store == nil {
		return nil, mdx.ErrIndexMiss
	}
	entries, err := s.store.PrefixSearch(dictionaryName, query, limit)
	if err != nil {
		return nil, err
	}
	hits := make([]mdx.SearchHit, 0, len(entries))
	for _, entry := range entries {
		hits = append(hits, mdx.SearchHit{Entry: entry, Score: prefixScore(query, entry.Keyword), Source: s.sourceName()})
	}
	if len(hits) == 0 {
		return nil, mdx.ErrIndexMiss
	}
	return hits, nil
}

func (s redisPrefixFuzzyStore) sourceName() string {
	if s.source != "" {
		return s.source
	}
	return "redis-prefix"
}
