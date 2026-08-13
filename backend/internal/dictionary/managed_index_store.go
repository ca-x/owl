package dictionary

import (
	"strings"
	"time"

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

func (s *managedDictionaryIndexStore) HasDictionaryIndex(dictionaryName string) (bool, error) {
	// Use the method set instead of naming mdx.IndexHealthStore directly so
	// this adapter remains source-compatible with mdx releases predating the
	// optional health interface.
	healthStore, ok := s.prefixStore.(interface {
		HasDictionaryIndex(string) (bool, error)
	})
	if !ok {
		// Health checks are optional. Preserve the lifecycle behavior of a
		// ManagedIndexStore that cannot verify its underlying data.
		return true, nil
	}
	return healthStore.HasDictionaryIndex(sanitizeManagedDictionaryName(dictionaryName))
}

func (s *managedDictionaryIndexStore) AcquireIndexBuildLease(dictionaryName string, ttl time.Duration) (func() error, bool, error) {
	leaseStore, ok := s.prefixStore.(interface {
		AcquireIndexBuildLease(string, time.Duration) (func() error, bool, error)
	})
	if !ok {
		// A missing optional lease capability means this process may proceed;
		// in-process namespace locking still serializes local rebuilds.
		return func() error { return nil }, true, nil
	}
	return leaseStore.AcquireIndexBuildLease(sanitizeManagedDictionaryName(dictionaryName), ttl)
}

func sanitizeManagedDictionaryName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	return sanitizeSlug(name)
}
