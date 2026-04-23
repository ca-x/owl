package dictionary

import (
	"strings"

	"github.com/lib-x/mdx"
)

type managedDictionaryIndexStore struct {
	prefixStore mdx.ManagedIndexStore
	searchStore *redisSearchStore
}

func newManagedDictionaryIndexStore(prefixStore mdx.ManagedIndexStore, searchStore *redisSearchStore) *managedDictionaryIndexStore {
	if prefixStore == nil {
		return nil
	}
	return &managedDictionaryIndexStore{prefixStore: prefixStore, searchStore: searchStore}
}

func (s *managedDictionaryIndexStore) Put(info mdx.DictionaryInfo, entries []mdx.IndexEntry) error {
	if s == nil || s.prefixStore == nil {
		return mdx.ErrIndexMiss
	}
	info.Name = sanitizeManagedDictionaryName(info.Name)
	if err := s.prefixStore.Put(info, entries); err != nil {
		return err
	}
	if s.searchStore != nil {
		if err := s.searchStore.Put(info, entries); err != nil {
			if isRediSearchUnavailable(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

func (s *managedDictionaryIndexStore) GetExact(dictionaryName, keyword string) (mdx.IndexEntry, error) {
	return s.prefixStore.GetExact(sanitizeManagedDictionaryName(dictionaryName), keyword)
}

func (s *managedDictionaryIndexStore) PrefixSearch(dictionaryName, prefix string, limit int) ([]mdx.IndexEntry, error) {
	return s.prefixStore.PrefixSearch(sanitizeManagedDictionaryName(dictionaryName), prefix, limit)
}

func (s *managedDictionaryIndexStore) LoadManifest(dictionaryName string) (mdx.IndexManifest, error) {
	return s.prefixStore.LoadManifest(sanitizeManagedDictionaryName(dictionaryName))
}

func (s *managedDictionaryIndexStore) SaveManifest(manifest mdx.IndexManifest) error {
	manifest.DictionaryName = sanitizeManagedDictionaryName(manifest.DictionaryName)
	return s.prefixStore.SaveManifest(manifest)
}

func (s *managedDictionaryIndexStore) DeleteDictionary(dictionaryName string) error {
	dictionaryName = sanitizeManagedDictionaryName(dictionaryName)
	if s.searchStore != nil {
		if err := s.searchStore.DeleteDictionary(dictionaryName); err != nil {
			return err
		}
	}
	return s.prefixStore.DeleteDictionary(dictionaryName)
}

func sanitizeManagedDictionaryName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	return sanitizeSlug(name)
}
