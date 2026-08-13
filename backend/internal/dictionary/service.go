package dictionary

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"owl/backend/ent"
	entdict "owl/backend/ent/dictionary"
	entuser "owl/backend/ent/user"
	"owl/backend/internal/models"

	"github.com/lib-x/mdx"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	client                    *ent.Client
	uploadsDir                string
	libraryDir                string
	redisClient               *redis.Client
	redisKeyPrefix            string
	redisPrefixMaxLen         int
	redisSearchKeyPrefix      string
	redisSearchEnabled        bool
	redisSearchUnavailable    atomic.Bool
	maxLoadedDictionaries     int
	audioTranscoder           *audioTranscoder
	libraryRefreshMu          sync.Mutex
	mu                        sync.RWMutex
	loaded                    map[int]*LoadedDictionary
	loadedOrder               []int
	loading                   map[int]*dictionaryLoadCall
	externalIndexReady        map[string]externalIndexCacheEntry
	externalIndexLoading      map[string]*externalIndexLoadCall
	legacyIndexCleaned        map[string]struct{}
	externalIndexCacheTTL     time.Duration
	now                       func() time.Time
	querySemaphore            chan struct{}
	managedPrefixStoreForTest func(context.Context) mdx.ManagedIndexStore
	deleteRedisSearchForTest  func(context.Context, string) error
	buildExternalIndexForTest func(context.Context, *ent.Dictionary) (*mdx.EnsureIndexResult, error)
}

type LoadedDictionary struct {
	MDX               *mdx.Mdict
	MDDs              []*mdx.Mdict
	FuzzyStore        mdx.FuzzyIndexStore
	PrefixStore       mdx.IndexStore
	ManagedIndexStore mdx.ManagedIndexStore
	Entries           []mdx.IndexEntry
	Info              mdx.DictionaryInfo
	RediSearchEnabled bool

	// MDD resource files are loaded lazily on first resource access so that
	// startup/warm-up and the first search stay fast. Building a full MDD index
	// (often the largest files, holding audio/images) is only needed when a
	// resource is actually served.
	slug     string
	mdxPath  string
	indexKey string
	mddPaths []string
	mddOnce  sync.Once
	mddErr   error
}

type dictionaryLoadCall struct {
	done   chan struct{}
	loaded *LoadedDictionary
	err    error
}

type externalIndexLoadCall struct {
	done chan struct{}
	err  error
}

type SearchParams struct {
	UserID       int
	IsAdmin      bool
	Query        string
	DictionaryID int
	Guest        bool
}

const (
	dictionaryQueryConcurrency = 8
	externalIndexCacheTTL      = 30 * time.Second
)

type externalIndexCacheEntry struct {
	fingerprint string
	validatedAt time.Time
}

type dictionarySearchWork struct {
	item    *ent.Dictionary
	results []models.SearchResult
}

type dictionarySuggestionWork struct {
	item *ent.Dictionary
	hits []mdx.SearchHit
}

func (s *Service) rediSearchAvailable() bool {
	return s != nil && s.redisClient != nil && s.redisSearchEnabled && !s.redisSearchUnavailable.Load()
}

func (s *Service) markRediSearchUnavailable(err error) {
	if s != nil && isRediSearchUnavailable(err) {
		s.redisSearchUnavailable.Store(true)
	}
}

func (s *Service) newRedisSearchStore(ctx context.Context, dictionaryName string) *redisSearchStore {
	if !s.rediSearchAvailable() {
		return nil
	}
	store := newRedisSearchStore(s.redisClient, firstNonEmpty(s.redisSearchKeyPrefix, "owl:mdx:search"), dictionaryName)
	store.ctx = ctx
	store.onUnavailable = s.markRediSearchUnavailable
	return store
}

func (s *Service) newRedisSearchCleanupStore(ctx context.Context, dictionaryName string) *redisSearchStore {
	if s == nil || s.redisClient == nil || !s.redisSearchEnabled {
		return nil
	}
	store := newRedisSearchStore(s.redisClient, firstNonEmpty(s.redisSearchKeyPrefix, "owl:mdx:search"), dictionaryName)
	store.ctx = ctx
	store.onUnavailable = s.markRediSearchUnavailable
	return store
}

func NewService(client *ent.Client, uploadsDir string, libraryDir string, redisClient *redis.Client, redisKeyPrefix string, redisPrefixMaxLen int, redisSearchKeyPrefix string, redisSearchEnabled bool, maxLoadedDictionaries int, audioCacheDir string, ffmpegBin string) *Service {
	return &Service{
		client:                client,
		uploadsDir:            uploadsDir,
		libraryDir:            libraryDir,
		redisClient:           redisClient,
		redisKeyPrefix:        strings.TrimSpace(redisKeyPrefix),
		redisPrefixMaxLen:     redisPrefixMaxLen,
		redisSearchKeyPrefix:  strings.TrimSpace(redisSearchKeyPrefix),
		redisSearchEnabled:    redisSearchEnabled,
		maxLoadedDictionaries: normalizeLoadedDictionaryLimit(maxLoadedDictionaries),
		audioTranscoder:       newAudioTranscoder(audioCacheDir, firstNonEmpty(strings.TrimSpace(ffmpegBin), resolveFFmpegBinary())),
		loaded:                make(map[int]*LoadedDictionary),
		loading:               make(map[int]*dictionaryLoadCall),
		externalIndexReady:    make(map[string]externalIndexCacheEntry),
		externalIndexLoading:  make(map[string]*externalIndexLoadCall),
		legacyIndexCleaned:    make(map[string]struct{}),
		externalIndexCacheTTL: externalIndexCacheTTL,
		now:                   time.Now,
		querySemaphore:        make(chan struct{}, dictionaryQueryConcurrency),
	}
}

func (s *Service) List(ctx context.Context, userID int, isAdmin bool) ([]models.DictionarySummary, error) {
	query := s.client.Dictionary.Query().WithOwner().Order(entdict.ByCreatedAt())
	if !isAdmin {
		query = query.Where(entdict.HasOwnerWith(entuser.IDEQ(userID)))
	}
	items, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.DictionarySummary, 0, len(items))
	for _, item := range items {
		out = append(out, toSummary(item))
	}
	return out, nil
}

func (s *Service) Toggle(ctx context.Context, id int, enabled bool, userID int, isAdmin bool) (*models.DictionarySummary, error) {
	item, err := s.getOwnedDictionary(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	updated, err := s.client.Dictionary.UpdateOneID(item.ID).SetEnabled(enabled).Save(ctx)
	if err != nil {
		return nil, err
	}
	return ptrSummary(updated), nil
}

func (s *Service) SetPublic(ctx context.Context, id int, public bool, userID int, isAdmin bool) (*models.DictionarySummary, error) {
	item, err := s.getOwnedDictionary(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	updated, err := s.client.Dictionary.UpdateOneID(item.ID).SetPublic(public).Save(ctx)
	if err != nil {
		return nil, err
	}
	return ptrSummary(updated), nil
}

func (s *Service) WarmEnabledDictionaries(ctx context.Context) error {
	dicts, err := s.client.Dictionary.Query().Where(entdict.Enabled(true)).Order(entdict.ByTitle()).All(ctx)
	if err != nil {
		return err
	}
	concurrency := warmConcurrency()
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, item := range dicts {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(item *ent.Dictionary) {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := s.ensureLoaded(ctx, item); err != nil {
				// Keep warming the remaining dictionaries even if one fails.
				log.Printf("warm dictionary id=%d name=%s: %v", item.ID, item.Slug, err)
			}
		}(item)
	}
	wg.Wait()
	return nil
}

func warmConcurrency() int {
	n := runtime.GOMAXPROCS(0)
	if n < 2 {
		return 2
	}
	if n > 8 {
		return 8
	}
	return n
}

func (s *Service) ListPublic(ctx context.Context) ([]models.DictionarySummary, error) {
	items, err := s.client.Dictionary.Query().
		WithOwner().
		Where(entdict.Enabled(true), entdict.Public(true)).
		Order(entdict.ByTitle()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.DictionarySummary, 0, len(items))
	for _, item := range items {
		out = append(out, toSummary(item))
	}
	return out, nil
}

func (s *Service) ListAccessible(ctx context.Context, userID int) ([]models.DictionarySummary, error) {
	items, err := s.client.Dictionary.Query().
		WithOwner().
		Where(
			entdict.Enabled(true),
			entdict.Or(
				entdict.Public(true),
				entdict.HasOwnerWith(entuser.IDEQ(userID)),
			),
		).
		Order(entdict.ByTitle()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.DictionarySummary, 0, len(items))
	for _, item := range items {
		out = append(out, toSummary(item))
	}
	return out, nil
}

func (s *Service) SearchBackendDebug(ctx context.Context, userID int, isAdmin bool, guest bool) (*models.SearchBackendDebug, error) {
	query := s.client.Dictionary.Query().Where(entdict.Enabled(true)).Order(entdict.ByTitle())
	if !isAdmin {
		if guest || userID == 0 {
			query = query.Where(entdict.Public(true))
		} else {
			query = query.Where(
				entdict.Or(
					entdict.Public(true),
					entdict.HasOwnerWith(entuser.IDEQ(userID)),
				),
			)
		}
	}
	dicts, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	result := &models.SearchBackendDebug{
		RedisConfigured:    s.redisClient != nil,
		RedisSearchEnabled: s.rediSearchAvailable(),
		Scope:              debugScopeLabel(userID, guest),
		Dictionaries:       make([]models.SearchBackendDictionary, 0, len(dicts)),
	}
	for _, item := range dicts {
		loaded, err := s.ensureLoaded(ctx, item)
		if err != nil {
			result.Dictionaries = append(result.Dictionaries, models.SearchBackendDictionary{
				DictionaryID:   item.ID,
				DictionaryName: displayName(item),
				Visibility:     visibilityLabel(item.Public),
				FuzzyBackend:   "unavailable",
				PrefixBackend:  "unavailable",
				Loaded:         false,
			})
			continue
		}
		result.Dictionaries = append(result.Dictionaries, models.SearchBackendDictionary{
			DictionaryID:   item.ID,
			DictionaryName: displayName(item),
			Visibility:     visibilityLabel(item.Public),
			FuzzyBackend:   s.fuzzyBackendName(loaded),
			PrefixBackend:  prefixBackendName(loaded),
			Loaded:         true,
		})
	}
	return result, nil
}

func (s *Service) Delete(ctx context.Context, id int, userID int, isAdmin bool) error {
	item, err := s.getOwnedDictionary(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if err := s.client.Dictionary.DeleteOneID(item.ID).Exec(ctx); err != nil {
		return err
	}
	s.unload(item.ID)
	if err := s.deleteExternalIndex(ctx, item); err != nil {
		log.Printf("delete dictionary index id=%d name=%s: %v", item.ID, item.Slug, err)
	}
	_ = os.Remove(item.MdxPath)
	for _, path := range decodePaths(item.MddPathsJSON) {
		_ = os.Remove(path)
	}
	return nil
}

func (s *Service) Upload(ctx context.Context, userID int, mdxFile *multipart.FileHeader, mddFiles []*multipart.FileHeader) (*models.DictionarySummary, error) {
	if mdxFile == nil {
		return nil, fmt.Errorf("mdx file is required")
	}
	userDir := filepath.Join(s.uploadsDir, fmt.Sprintf("user-%d", userID), time.Now().Format("20060102150405"))
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, err
	}
	mdxPath, err := saveUploadedFile(mdxFile, userDir)
	if err != nil {
		return nil, err
	}
	mddPaths := make([]string, 0, len(mddFiles))
	for _, file := range mddFiles {
		path, err := saveUploadedFile(file, userDir)
		if err != nil {
			return nil, err
		}
		mddPaths = append(mddPaths, path)
	}
	loaded, meta, err := s.buildLoadedDictionary(ctx, mdxPath, mddPaths)
	if err != nil {
		return nil, err
	}
	pathsJSON, err := json.Marshal(mddPaths)
	if err != nil {
		return nil, err
	}
	created, err := s.client.Dictionary.Create().
		SetName(meta.Name).
		SetTitle(meta.Title).
		SetDescription(meta.Description).
		SetSlug(meta.Slug).
		SetMdxPath(mdxPath).
		SetMddPathsJSON(string(pathsJSON)).
		SetEntryCount(meta.EntryCount).
		SetPublic(false).
		SetOwnerID(userID).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cacheLoadedLocked(created.ID, loaded)
	s.mu.Unlock()
	return ptrSummary(created), nil
}

func (s *Service) Search(ctx context.Context, params SearchParams) ([]models.SearchResult, error) {
	query := s.client.Dictionary.Query().Where(entdict.Enabled(true)).Order(entdict.ByTitle())
	if params.DictionaryID > 0 {
		query = query.Where(entdict.IDEQ(params.DictionaryID))
	}
	if !params.IsAdmin {
		if params.Guest || params.UserID == 0 {
			query = query.Where(entdict.Public(true))
		} else {
			query = query.Where(
				entdict.Or(
					entdict.Public(true),
					entdict.HasOwnerWith(entuser.IDEQ(params.UserID)),
				),
			)
		}
	}
	dicts, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	work := mapDictionariesPrepared(ctx, s.querySemaphore, dicts, func(item *ent.Dictionary) error {
		return s.ensureExternalIndex(ctx, item)
	}, func(item *ent.Dictionary, indexErr error) dictionarySearchWork {
		result := dictionarySearchWork{item: item}
		if indexErr != nil {
			return result
		}
		hits, indexed, err := s.searchPreparedExternalIndexHits(ctx, item, params.Query, 10)
		if err != nil {
			return result
		}
		if !indexed {
			loaded, err := s.ensureLoaded(ctx, item)
			if err != nil {
				return result
			}
			hits, err = searchIndexHits(loaded, loadedIndexKey(loaded), params.Query, 10)
			if err != nil {
				return result
			}
		}
		if len(hits) == 0 {
			return result
		}
		loaded, err := s.ensureLoaded(ctx, item)
		if err != nil {
			return result
		}
		result.results = make([]models.SearchResult, 0, len(hits))
		for _, hit := range hits {
			searchResult, buildErr := buildSearchResult(item, loaded, hit.Entry, hit.Score, hit.Source)
			if buildErr == nil {
				result.results = append(result.results, searchResult)
			}
		}
		return result
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results := make([]models.SearchResult, 0)
	seen := make(map[string]struct{})
	for _, itemWork := range work {
		for _, result := range itemWork.results {
			key := fmt.Sprintf("%d:%s", itemWork.item.ID, strings.ToLower(result.Word))
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			results = append(results, result)
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		left := resultRank(results[i], params)
		right := resultRank(results[j], params)
		if left != right {
			return left < right
		}
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Visibility != results[j].Visibility {
			if params.Guest || params.UserID == 0 {
				return results[i].Visibility == "public"
			}
			return results[i].Visibility == "private"
		}
		if results[i].DictionaryName == results[j].DictionaryName {
			return len(results[i].Word) < len(results[j].Word)
		}
		return results[i].DictionaryName < results[j].DictionaryName
	})
	return results, nil
}

func searchIndexHits(loaded *LoadedDictionary, dictionaryName string, query string, limit int) ([]mdx.SearchHit, error) {
	if loaded == nil {
		return nil, fmt.Errorf("dictionary not loaded")
	}
	if limit <= 0 {
		limit = 10
	}

	hits := make([]mdx.SearchHit, 0, limit)
	if loaded.MDX != nil {
		if exactEntry, ok := loaded.MDX.FindExactEntry(query); ok && exactEntry != nil {
			hits = append(hits, mdx.SearchHit{Entry: keywordEntryToIndexEntry(exactEntry), Score: 1.0, Source: "exact"})
		} else if comparableEntry, ok := loaded.MDX.FindComparableEntry(query); ok && comparableEntry != nil {
			hits = append(hits, mdx.SearchHit{Entry: keywordEntryToIndexEntry(comparableEntry), Score: 0.99, Source: "comparable"})
		}
	}

	if loaded.FuzzyStore != nil {
		searchHits, searchErr := loaded.FuzzyStore.Search(dictionaryName, query, limit)
		if searchErr == nil {
			hits = append(hits, searchHits...)
		} else if !errors.Is(searchErr, mdx.ErrIndexMiss) && !isRediSearchUnavailable(searchErr) && loaded.PrefixStore == nil {
			return nil, searchErr
		}
	}

	if len(hits) == 0 && loaded.PrefixStore != nil {
		entries, prefixErr := loaded.PrefixStore.PrefixSearch(dictionaryName, query, limit)
		if prefixErr != nil {
			return nil, prefixErr
		}
		for _, entry := range entries {
			hits = append(hits, mdx.SearchHit{Entry: entry, Score: prefixScore(query, entry.Keyword), Source: "redis-prefix"})
		}
	}
	if len(hits) == 0 {
		return nil, mdx.ErrIndexMiss
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (s *Service) searchPreparedExternalIndexHits(ctx context.Context, item *ent.Dictionary, query string, limit int) ([]mdx.SearchHit, bool, error) {
	if item == nil || !s.hasExternalIndexStore() {
		return nil, false, nil
	}
	hits, err := s.searchExternalStoreHits(ctx, externalIndexKey(item.MdxPath), query, limit)
	if err != nil {
		if errors.Is(err, mdx.ErrIndexMiss) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return hits, true, nil
}

func (s *Service) ensureExternalIndex(ctx context.Context, item *ent.Dictionary) error {
	if item == nil || !s.hasExternalIndexStore() {
		return nil
	}
	for {
		fingerprint, ready := s.externalIndexFingerprint(item.MdxPath)
		if ready {
			return nil
		}

		s.mu.Lock()
		now := s.now()
		entry, ok := s.externalIndexReady[item.MdxPath]
		if ok && !now.Before(entry.validatedAt) && now.Sub(entry.validatedAt) < s.externalIndexCacheTTL {
			s.mu.Unlock()
			return nil
		}
		if call, ok := s.externalIndexLoading[item.MdxPath]; ok {
			done := call.done
			s.mu.Unlock()
			select {
			case <-done:
				if err := ctx.Err(); err != nil {
					return err
				}
				if isContextCancellation(call.err) {
					continue
				}
				return call.err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		call := &externalIndexLoadCall{done: make(chan struct{})}
		s.externalIndexLoading[item.MdxPath] = call
		s.mu.Unlock()

		result, err := s.buildExternalIndex(ctx, item)
		if err == nil {
			s.markExternalIndexReady(item.MdxPath, fingerprint, result.Manifest.Fingerprint)
		}
		s.mu.Lock()
		call.err = err
		delete(s.externalIndexLoading, item.MdxPath)
		close(call.done)
		s.mu.Unlock()
		return err
	}
}

func (s *Service) buildExternalIndex(ctx context.Context, item *ent.Dictionary) (*mdx.EnsureIndexResult, error) {
	if s.buildExternalIndexForTest != nil {
		select {
		case s.querySemaphore <- struct{}{}:
			defer func() { <-s.querySemaphore }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return s.buildExternalIndexForTest(ctx, item)
	}
	select {
	case s.querySemaphore <- struct{}{}:
		defer func() { <-s.querySemaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	prefixStore := s.newManagedPrefixStore(ctx)
	indexKey := externalIndexKey(item.MdxPath)
	searchStore := s.newRedisSearchStore(ctx, indexKey)
	managed := newManagedDictionaryIndexStore(prefixStore, searchStore)
	result, err := mdx.EnsureDictionaryIndex(item.MdxPath, managed,
		mdx.WithReuseIfUnchanged(true),
		mdx.WithIndexDictionaryName(indexKey),
	)
	if err != nil {
		return nil, err
	}
	if err := s.deleteLegacyExternalIndexes(ctx, item, indexKey); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) deleteExternalIndex(ctx context.Context, item *ent.Dictionary) error {
	if item == nil || !s.hasExternalIndexStore() {
		return nil
	}
	prefixStore := s.newManagedPrefixStore(ctx)
	indexKey := externalIndexKey(item.MdxPath)
	searchStore := s.newRedisSearchCleanupStore(ctx, indexKey)
	if err := s.deleteRedisSearchIndex(ctx, searchStore, indexKey); err != nil {
		return err
	}
	err := prefixStore.DeleteDictionary(indexKey)
	if err == nil {
		s.mu.Lock()
		delete(s.externalIndexReady, item.MdxPath)
		s.mu.Unlock()
	}
	return err
}

func (s *Service) searchExternalStoreHits(ctx context.Context, dictionaryName string, query string, limit int) ([]mdx.SearchHit, error) {
	if !s.hasExternalIndexStore() {
		return nil, mdx.ErrIndexMiss
	}
	if limit <= 0 {
		limit = 10
	}

	hits := make([]mdx.SearchHit, 0, limit)
	if searchStore := s.newRedisSearchStore(ctx, dictionaryName); searchStore != nil {
		searchHits, searchErr := searchStore.Search(dictionaryName, query, limit)
		if searchErr == nil {
			hits = append(hits, searchHits...)
		} else if !errors.Is(searchErr, mdx.ErrIndexMiss) && !isRediSearchUnavailable(searchErr) {
			return nil, searchErr
		}
	}

	prefixStore := s.newManagedPrefixStore(ctx)
	if len(hits) == 0 {
		if searchStore, ok := prefixStore.(mdx.FuzzyIndexStore); ok {
			searchHits, searchErr := searchStore.Search(dictionaryName, query, limit)
			if searchErr == nil {
				hits = append(hits, searchHits...)
			} else if !errors.Is(searchErr, mdx.ErrIndexMiss) {
				return nil, searchErr
			}
		}
	}
	if len(hits) == 0 {
		entries, prefixErr := prefixStore.PrefixSearch(dictionaryName, query, limit)
		if prefixErr != nil {
			return nil, prefixErr
		}
		for _, entry := range entries {
			hits = append(hits, mdx.SearchHit{Entry: entry, Score: prefixScore(query, entry.Keyword), Source: s.prefixSearchSource()})
		}
	}
	if len(hits) == 0 {
		return nil, mdx.ErrIndexMiss
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (s *Service) newManagedPrefixStore(ctx context.Context) mdx.ManagedIndexStore {
	if s.managedPrefixStoreForTest != nil {
		return s.managedPrefixStoreForTest(ctx)
	}
	if s.redisClient == nil {
		return newSQLDictionaryIndexStore(ctx, s.client)
	}
	return s.newRedisPrefixStore(ctx)
}

func (s *Service) newRedisPrefixStore(ctx context.Context) mdx.ManagedIndexStore {
	if ctx == nil {
		ctx = context.Background()
	}
	return mdx.NewRedisIndexStore(s.redisClient,
		mdx.WithRedisIndexContext(ctx),
		mdx.WithRedisKeyPrefix(firstNonEmpty(s.redisKeyPrefix, "owl:mdx:index")),
		mdx.WithRedisPrefixIndexMaxLen(max(s.redisPrefixMaxLen, 1)),
	)
}

func (s *Service) hasExternalIndexStore() bool {
	return s.redisClient != nil || s.client != nil
}

func (s *Service) prefixSearchSource() string {
	if s.redisClient != nil {
		return "redis-prefix"
	}
	return "sql-prefix"
}

func keywordEntryToIndexEntry(entry *mdx.MDictKeywordEntry) mdx.IndexEntry {
	return mdx.IndexEntry{
		Keyword:           entry.KeyWord,
		RecordStartOffset: entry.RecordStartOffset,
		RecordEndOffset:   entry.RecordEndOffset,
		KeyBlockIdx:       entry.KeyBlockIdx,
	}
}

func buildSearchResult(item *ent.Dictionary, loaded *LoadedDictionary, entry mdx.IndexEntry, score float64, source string) (models.SearchResult, error) {
	htmlContent, err := resolveEntryHTML(loaded, entry, 0, nil)
	if err != nil {
		return models.SearchResult{}, err
	}
	assetBase := fmt.Sprintf("/api/dictionaries/%d/resource", item.ID)
	if item.Public {
		assetBase = fmt.Sprintf("/api/public/dictionaries/%d/resource", item.ID)
	}
	rewritten := mdx.RewriteEntryHTML([]byte(htmlContent), assetBase, "/search?q=")
	html := string(rewritten)
	return models.SearchResult{
		DictionaryID:   item.ID,
		DictionaryName: displayName(item),
		Visibility:     visibilityLabel(item.Public),
		Word:           entry.Keyword,
		HTML:           html,
		Score:          score,
		Source:         source,
	}, nil
}

func resolveEntryHTML(loaded *LoadedDictionary, entry mdx.IndexEntry, depth int, seen map[string]struct{}) (string, error) {
	if loaded == nil || loaded.MDX == nil {
		return "", fmt.Errorf("dictionary not loaded")
	}
	return resolveEntryRedirects(entry, depth, seen, loaded.MDX.Resolve, func(word string) (mdx.IndexEntry, error) {
		if loaded.PrefixStore == nil {
			return mdx.IndexEntry{}, mdx.ErrIndexMiss
		}
		return lookupRedirectEntry(loaded.PrefixStore, loadedIndexKey(loaded), word)
	})
}

func lookupRedirectEntry(store mdx.IndexStore, dictionaryName, word string) (mdx.IndexEntry, error) {
	word = strings.TrimSpace(word)
	entry, err := store.GetExact(dictionaryName, word)
	if err == nil || !errors.Is(err, mdx.ErrIndexMiss) {
		return entry, err
	}
	if comparableStore, ok := store.(mdx.ComparableIndexStore); ok {
		return comparableStore.GetComparable(dictionaryName, word)
	}
	return mdx.IndexEntry{}, mdx.ErrIndexMiss
}

func resolveEntryRedirects(entry mdx.IndexEntry, depth int, seen map[string]struct{}, resolve func(mdx.IndexEntry) ([]byte, error), lookup func(string) (mdx.IndexEntry, error)) (string, error) {
	if depth > 6 {
		return "", fmt.Errorf("link depth exceeded")
	}
	content, err := resolve(entry)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(content))
	if !strings.HasPrefix(text, "@@@LINK=") {
		return text, nil
	}

	target := strings.TrimSpace(strings.TrimPrefix(text, "@@@LINK="))
	if target == "" {
		return "", fmt.Errorf("empty link target")
	}
	if seen == nil {
		seen = make(map[string]struct{})
	}
	key := strings.ToLower(target)
	if _, ok := seen[key]; ok {
		return fmt.Sprintf("<p>%s</p>", html.EscapeString(target)), nil
	}
	seen[key] = struct{}{}

	targetEntry, err := lookup(target)
	if err != nil {
		return fmt.Sprintf("<p>%s</p>", html.EscapeString(target)), nil
	}
	return resolveEntryRedirects(targetEntry, depth+1, seen, resolve, lookup)
}

func (s *Service) Suggest(ctx context.Context, params SearchParams, limit int) ([]models.SearchSuggestion, error) {
	if limit <= 0 {
		limit = 8
	}
	query := s.client.Dictionary.Query().Where(entdict.Enabled(true)).Order(entdict.ByTitle())
	if params.DictionaryID > 0 {
		query = query.Where(entdict.IDEQ(params.DictionaryID))
	}
	if !params.IsAdmin {
		if params.Guest || params.UserID == 0 {
			query = query.Where(entdict.Public(true))
		} else {
			query = query.Where(
				entdict.Or(
					entdict.Public(true),
					entdict.HasOwnerWith(entuser.IDEQ(params.UserID)),
				),
			)
		}
	}
	dicts, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	work := mapDictionariesPrepared(ctx, s.querySemaphore, dicts, func(item *ent.Dictionary) error {
		return s.ensureExternalIndex(ctx, item)
	}, func(item *ent.Dictionary, indexErr error) dictionarySuggestionWork {
		result := dictionarySuggestionWork{item: item}
		if indexErr != nil {
			return result
		}
		indexHits, indexed, indexErr := s.searchPreparedExternalIndexHits(ctx, item, params.Query, max(limit*4, limit))
		if indexErr != nil {
			return result
		}
		if indexed {
			result.hits = indexHits
			return result
		}

		loaded, loadErr := s.ensureLoaded(ctx, item)
		if loadErr != nil {
			return result
		}
		if loaded.FuzzyStore != nil {
			searchHits, searchErr := loaded.FuzzyStore.Search(loadedIndexKey(loaded), params.Query, max(limit*3, limit))
			if searchErr == nil {
				result.hits = searchHits
			} else if loaded.RediSearchEnabled && !errors.Is(searchErr, mdx.ErrIndexMiss) && !isRediSearchUnavailable(searchErr) {
				return result
			}
		}
		if len(result.hits) == 0 && loaded.PrefixStore != nil {
			entries, prefixErr := loaded.PrefixStore.PrefixSearch(loadedIndexKey(loaded), params.Query, max(limit*4, limit))
			if prefixErr == nil {
				for _, entry := range entries {
					result.hits = append(result.hits, mdx.SearchHit{Entry: entry, Score: prefixScore(params.Query, entry.Keyword), Source: "redis-prefix"})
				}
			}
		}
		return result
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	type aggregatedSuggestion struct {
		word       string
		sources    []models.SearchSuggestionSource
		bestScore  float64
		bestRank   int
		firstIndex int
	}

	aggregated := make(map[string]*aggregatedSuggestion)
	orderedKeys := make([]string, 0, limit)
	seenSources := make(map[string]struct{})
	globalIndex := 0

	for _, itemWork := range work {
		for _, hit := range itemWork.hits {
			word := strings.TrimSpace(hit.Entry.Keyword)
			if word == "" {
				continue
			}

			normalizedWord := strings.ToLower(word)
			sourceKey := fmt.Sprintf("%s:%d", normalizedWord, itemWork.item.ID)
			if _, ok := seenSources[sourceKey]; ok {
				continue
			}
			seenSources[sourceKey] = struct{}{}

			agg, ok := aggregated[normalizedWord]
			if !ok {
				agg = &aggregatedSuggestion{
					word:       word,
					bestScore:  hit.Score,
					bestRank:   suggestionRank(word, params.Query),
					firstIndex: globalIndex,
				}
				aggregated[normalizedWord] = agg
				orderedKeys = append(orderedKeys, normalizedWord)
			}

			rank := suggestionRank(word, params.Query)
			if rank < agg.bestRank || (rank == agg.bestRank && hit.Score > agg.bestScore) {
				agg.bestRank = rank
				agg.bestScore = hit.Score
				agg.word = word
			}

			agg.sources = append(agg.sources, models.SearchSuggestionSource{
				DictionaryID:   itemWork.item.ID,
				DictionaryName: displayName(itemWork.item),
				Visibility:     visibilityLabel(itemWork.item.Public),
				Source:         hit.Source,
			})
			globalIndex++
		}
	}

	sort.SliceStable(orderedKeys, func(i, j int) bool {
		left := aggregated[orderedKeys[i]]
		right := aggregated[orderedKeys[j]]
		if left.bestRank != right.bestRank {
			return left.bestRank < right.bestRank
		}
		if left.bestScore != right.bestScore {
			return left.bestScore > right.bestScore
		}
		if len(left.sources) != len(right.sources) {
			return len(left.sources) > len(right.sources)
		}
		return left.firstIndex < right.firstIndex
	})

	if len(orderedKeys) > limit {
		orderedKeys = orderedKeys[:limit]
	}

	suggestions := make([]models.SearchSuggestion, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		agg := aggregated[key]
		sort.SliceStable(agg.sources, func(i, j int) bool {
			left := agg.sources[i]
			right := agg.sources[j]
			if left.Visibility != right.Visibility {
				return left.Visibility == "public"
			}
			return left.DictionaryName < right.DictionaryName
		})
		suggestions = append(suggestions, models.SearchSuggestion{
			Word:    agg.word,
			Sources: agg.sources,
		})
	}
	return suggestions, nil
}

func mapDictionaries[T any](ctx context.Context, semaphore chan struct{}, dicts []*ent.Dictionary, fn func(*ent.Dictionary) T) []T {
	return mapDictionariesPrepared(ctx, semaphore, dicts, func(*ent.Dictionary) struct{} { return struct{}{} }, func(item *ent.Dictionary, _ struct{}) T {
		return fn(item)
	})
}

func mapDictionariesPrepared[P, T any](ctx context.Context, semaphore chan struct{}, dicts []*ent.Dictionary, prepare func(*ent.Dictionary) P, fn func(*ent.Dictionary, P) T) []T {
	results := make([]T, len(dicts))
	if len(dicts) == 0 {
		return results
	}
	if ctx == nil {
		ctx = context.Background()
	}

	type dictionaryJob struct {
		index int
		item  *ent.Dictionary
	}
	jobs := make(chan dictionaryJob, len(dicts))
	for index, item := range dicts {
		jobs <- dictionaryJob{index: index, item: item}
	}
	close(jobs)

	var wg sync.WaitGroup
	for range min(len(dicts), dictionaryQueryConcurrency) {
		wg.Go(func() {
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				prepared := prepare(job.item)
				if ctx.Err() != nil {
					return
				}
				select {
				case semaphore <- struct{}{}:
				case <-ctx.Done():
					return
				}
				if ctx.Err() != nil {
					<-semaphore
					return
				}
				func() {
					defer func() { <-semaphore }()
					results[job.index] = fn(job.item, prepared)
				}()
			}
		})
	}
	wg.Wait()
	return results
}

// ensureMDDsLoaded lazily opens and indexes the paired MDD resource files for a
// loaded dictionary. This is deferred out of the startup/search path because MDD
// files are typically the largest (audio, images) and building their index is
// only required to serve a resource. The work runs at most once per dictionary.
func (s *Service) ensureMDDsLoaded(loaded *LoadedDictionary) error {
	if loaded == nil {
		return fmt.Errorf("dictionary not loaded")
	}
	loaded.mddOnce.Do(func() {
		mddPaths := discoverPairedMDDs(loaded.mdxPath, loaded.mddPaths)
		mdds := make([]*mdx.Mdict, 0, len(mddPaths))
		for _, mddPath := range mddPaths {
			mddDict, err := mdx.New(mddPath)
			if err != nil {
				loaded.mddErr = err
				return
			}
			if err := mddDict.BuildIndex(); err != nil {
				loaded.mddErr = err
				return
			}
			mdds = append(mdds, mddDict)
		}
		loaded.MDDs = mdds
		mdx.ConfigureDictionaryPairAssets(mdx.DictionarySpec{ID: loaded.slug, Name: loaded.slug, MDXPath: loaded.mdxPath, MDDPaths: mddPaths}, loaded.MDX, mdds...)
	})
	return loaded.mddErr
}

func (s *Service) OpenResource(ctx context.Context, id int, userID int, isAdmin bool, guest bool, resourcePath string) ([]byte, string, error) {
	item, err := s.getAccessibleDictionary(ctx, id, userID, isAdmin, guest)
	if err != nil {
		return nil, "", err
	}
	loaded, err := s.ensureLoaded(ctx, item)
	if err != nil {
		return nil, "", err
	}
	if err := s.ensureMDDsLoaded(loaded); err != nil {
		return nil, "", err
	}
	resourcePath = strings.TrimSpace(strings.TrimPrefix(resourcePath, "/"))
	if decoded, err := url.PathUnescape(resourcePath); err == nil {
		resourcePath = decoded
	}

	if loaded.MDX != nil && loaded.MDX.AssetResolver() != nil {
		data, resolverErr := loaded.MDX.AssetResolver().Read(resourcePath)
		if resolverErr == nil {
			return s.prepareResource(resourcePath, data)
		}
	}

	candidates := mdx.AssetLookupCandidates(resourcePath)
	if len(candidates) == 0 {
		candidates = []string{resourcePath}
	}

	for _, dict := range loaded.MDDs {
		fs := mdx.NewMdictFS(dict)
		for _, candidate := range candidates {
			file, err := fs.Open(candidate)
			if err != nil {
				continue
			}
			data, readErr := io.ReadAll(file)
			_ = file.Close()
			if readErr != nil {
				continue
			}
			return s.prepareResource(candidate, data)
		}
	}
	return nil, "", fmt.Errorf("resource not found")
}

func (s *Service) prepareResource(path string, data []byte) ([]byte, string, error) {
	if s.audioTranscoder != nil && s.audioTranscoder.enabled() && looksLikeSpeex(data) {
		transcoded, err := s.audioTranscoder.transcodeToMP3(path, data)
		if err == nil {
			return transcoded, "audio/mpeg", nil
		}
	}
	return data, detectResourceContentType(path, data), nil
}

func (s *Service) ensureLoaded(ctx context.Context, item *ent.Dictionary) (*LoadedDictionary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		s.mu.RLock()
		loaded, ok := s.loaded[item.ID]
		s.mu.RUnlock()
		if ok {
			s.mu.Lock()
			s.touchLoadedLocked(item.ID)
			s.mu.Unlock()
			return loaded, nil
		}

		s.mu.Lock()
		if loaded, ok := s.loaded[item.ID]; ok {
			s.touchLoadedLocked(item.ID)
			s.mu.Unlock()
			return loaded, nil
		}
		if call, ok := s.loading[item.ID]; ok {
			done := call.done
			s.mu.Unlock()
			select {
			case <-done:
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				// A leader's request context is independent of this waiter's. If
				// the leader was canceled while this request remains active, let
				// this request retry and become the next leader instead of
				// inheriting a cancellation it did not request.
				if isContextCancellation(call.err) {
					continue
				}
				return call.loaded, call.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		call := &dictionaryLoadCall{done: make(chan struct{})}
		s.loading[item.ID] = call
		s.mu.Unlock()

		fresh, _, err := s.buildLoadedDictionary(ctx, item.MdxPath, decodePaths(item.MddPathsJSON))
		s.mu.Lock()
		if err == nil {
			if loaded, ok := s.loaded[item.ID]; ok {
				fresh = loaded
				s.touchLoadedLocked(item.ID)
			} else {
				s.cacheLoadedLocked(item.ID, fresh)
			}
		}
		call.loaded = fresh
		call.err = err
		delete(s.loading, item.ID)
		close(call.done)
		s.mu.Unlock()
		return fresh, err
	}
}

func isContextCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *Service) unload(id int) {
	s.mu.Lock()
	delete(s.loaded, id)
	s.removeLoadedOrderLocked(id)
	s.mu.Unlock()
}

func (s *Service) cacheLoadedLocked(id int, loaded *LoadedDictionary) {
	if loaded == nil {
		return
	}
	s.loaded[id] = loaded
	s.touchLoadedLocked(id)
	if s.maxLoadedDictionaries == 0 {
		return
	}
	for len(s.loadedOrder) > s.maxLoadedDictionaries {
		evictID := s.loadedOrder[0]
		s.loadedOrder = s.loadedOrder[1:]
		if evictID == id {
			continue
		}
		delete(s.loaded, evictID)
	}
}

func (s *Service) touchLoadedLocked(id int) {
	s.removeLoadedOrderLocked(id)
	s.loadedOrder = append(s.loadedOrder, id)
}

func (s *Service) removeLoadedOrderLocked(id int) {
	for idx, loadedID := range s.loadedOrder {
		if loadedID == id {
			copy(s.loadedOrder[idx:], s.loadedOrder[idx+1:])
			s.loadedOrder = s.loadedOrder[:len(s.loadedOrder)-1]
			return
		}
	}
}

func (s *Service) getOwnedDictionary(ctx context.Context, id int, userID int, isAdmin bool) (*ent.Dictionary, error) {
	query := s.client.Dictionary.Query().Where(entdict.IDEQ(id))
	if !isAdmin {
		query = query.Where(entdict.HasOwnerWith(entuser.IDEQ(userID)))
	}
	return query.Only(ctx)
}

func (s *Service) getAccessibleDictionary(ctx context.Context, id int, userID int, isAdmin bool, guest bool) (*ent.Dictionary, error) {
	query := s.client.Dictionary.Query().Where(entdict.IDEQ(id), entdict.Enabled(true))
	if isAdmin {
		return query.Only(ctx)
	}
	if guest || userID == 0 {
		query = query.Where(entdict.Public(true))
		return query.Only(ctx)
	}
	query = query.Where(
		entdict.Or(
			entdict.Public(true),
			entdict.HasOwnerWith(entuser.IDEQ(userID)),
		),
	)
	return query.Only(ctx)
}

func (s *Service) Refresh(ctx context.Context, id int, userID int, isAdmin bool) (*models.MaintenanceReport, error) {
	item, err := s.getOwnedDictionary(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	mddPaths := discoverPairedMDDs(item.MdxPath, decodePaths(item.MddPathsJSON))
	loaded, meta, err := s.buildLoadedDictionary(ctx, item.MdxPath, mddPaths)
	if err != nil {
		return &models.MaintenanceReport{
			Summary: "refresh failed",
			Failed:  1,
			Items: []models.MaintenanceItemReport{{
				DictionaryID: item.ID,
				Name:         item.Title,
				Action:       "refresh",
				Status:       "failed",
				Message:      err.Error(),
			}},
		}, nil
	}
	rawPaths, err := json.Marshal(mddPaths)
	if err != nil {
		return nil, err
	}
	updated, err := s.client.Dictionary.UpdateOneID(item.ID).
		SetName(meta.Name).
		SetTitle(meta.Title).
		SetDescription(meta.Description).
		SetSlug(meta.Slug).
		SetMddPathsJSON(string(rawPaths)).
		SetEntryCount(meta.EntryCount).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cacheLoadedLocked(item.ID, loaded)
	s.mu.Unlock()
	return &models.MaintenanceReport{
		Summary: "dictionary refreshed",
		Updated: 1,
		Items: []models.MaintenanceItemReport{{
			DictionaryID: updated.ID,
			Name:         displayName(updated),
			Action:       "refresh",
			Status:       "updated",
			Message:      "Dictionary metadata and paired resources reloaded",
			Dictionary:   ptrSummary(updated),
		}},
	}, nil
}

func (s *Service) RefreshLibrary(ctx context.Context, userID int, isAdmin bool) (*models.MaintenanceReport, error) {
	return s.withLibraryRefreshLock(func() (*models.MaintenanceReport, error) {
		root := s.libraryDir
		if strings.TrimSpace(root) == "" {
			root = s.uploadsDir
		}
		pairs, err := scanDictionaryPairs(root)
		if err != nil {
			return nil, err
		}
		report := &models.MaintenanceReport{Items: make([]models.MaintenanceItemReport, 0, len(pairs))}
		for _, pair := range pairs {
			item, action, err := s.upsertDictionaryFromPair(ctx, pair, userID, isAdmin)
			if err != nil {
				report.Failed++
				report.Items = append(report.Items, models.MaintenanceItemReport{
					Name:    filepath.Base(pair.MDXPath),
					Action:  "scan",
					Status:  "failed",
					Message: err.Error(),
				})
				continue
			}
			switch action {
			case "discovered":
				report.Discovered++
			case "updated":
				report.Updated++
			default:
				report.Skipped++
			}
			report.Items = append(report.Items, models.MaintenanceItemReport{
				DictionaryID: item.ID,
				Name:         item.Title,
				Action:       "scan",
				Status:       action,
				Message:      maintenanceMessage(action),
				Dictionary:   item,
			})
		}
		report.Summary = fmt.Sprintf("discovered %d, updated %d, skipped %d, failed %d", report.Discovered, report.Updated, report.Skipped, report.Failed)
		return report, nil
	})
}

func (s *Service) withLibraryRefreshLock(fn func() (*models.MaintenanceReport, error)) (*models.MaintenanceReport, error) {
	s.libraryRefreshMu.Lock()
	defer s.libraryRefreshMu.Unlock()
	return fn()
}

type dictionaryMeta struct {
	Name        string
	Title       string
	Description string
	Slug        string
	EntryCount  int
}

func (s *Service) buildLoadedDictionary(ctx context.Context, mdxPath string, mddPaths []string) (*LoadedDictionary, dictionaryMeta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, dictionaryMeta{}, err
	}
	mdxDict, err := mdx.New(mdxPath)
	if err != nil {
		return nil, dictionaryMeta{}, err
	}
	info := mdxDict.DictionaryInfo()
	slug := sanitizeSlug(firstNonEmpty(info.Name, mdxDict.Name(), strings.TrimSuffix(filepath.Base(mdxPath), filepath.Ext(mdxPath))))
	info.Name = slug

	var entries []mdx.IndexEntry
	var prefixStore mdx.IndexStore
	var managedStore mdx.ManagedIndexStore
	var fuzzyStore mdx.FuzzyIndexStore
	rediSearchEnabled := false
	hasExternalIndex := s.redisClient != nil || s.client != nil
	indexKey := slug
	if hasExternalIndex {
		indexKey = externalIndexKey(mdxPath)
	}
	if hasExternalIndex {
		if err := mdxDict.PrepareForResolve(); err != nil {
			return nil, dictionaryMeta{}, err
		}
	} else if err := mdxDict.PrepareForExternalIndex(); err != nil {
		return nil, dictionaryMeta{}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, dictionaryMeta{}, err
	}
	if s.redisClient != nil {
		redisOptions := []mdx.RedisIndexStoreOption{
			mdx.WithRedisKeyPrefix(firstNonEmpty(s.redisKeyPrefix, "owl:mdx:index")),
			mdx.WithRedisPrefixIndexMaxLen(max(s.redisPrefixMaxLen, 1)),
		}
		ensurePrefixStore := mdx.NewRedisIndexStore(s.redisClient,
			mdx.WithRedisIndexContext(ctx),
			redisOptions[0],
			redisOptions[1],
		)
		ensureSearchStore := s.newRedisSearchStore(ctx, indexKey)
		managed := newManagedDictionaryIndexStore(ensurePrefixStore, ensureSearchStore)
		fingerprint, ready := s.externalIndexFingerprint(mdxPath)
		if !ready {
			ensureResult, err := mdx.EnsureDictionaryIndex(mdxPath, managed,
				mdx.WithReuseIfUnchanged(true),
				mdx.WithIndexDictionaryName(indexKey),
			)
			if err != nil {
				return nil, dictionaryMeta{}, err
			}
			if err := s.deleteLegacyExternalIndexes(ctx, &ent.Dictionary{Slug: slug, MdxPath: mdxPath}, indexKey); err != nil {
				return nil, dictionaryMeta{}, err
			}
			log.Printf("dictionary index sync name=%s reused=%t rebuilt=%t schema=%s source=%s", ensureResult.DictionaryName, ensureResult.Reused, ensureResult.Rebuilt, ensureResult.Manifest.SchemaVersion, ensureResult.Manifest.SourcePath)
			s.markExternalIndexReady(mdxPath, fingerprint, ensureResult.Manifest.Fingerprint)
		}
		redisPrefixStore := mdx.NewRedisIndexStore(s.redisClient,
			mdx.WithRedisIndexContext(context.Background()),
			redisOptions[0],
			redisOptions[1],
		)
		searchStore := s.newRedisSearchStore(context.Background(), indexKey)
		prefixStore = redisPrefixStore
		managedStore = newManagedDictionaryIndexStore(redisPrefixStore, searchStore)
		if searchStore != nil && s.rediSearchAvailable() {
			fuzzyStore = searchStore
			rediSearchEnabled = true
		} else {
			fuzzyStore = redisPrefixFuzzyStore{store: redisPrefixStore, source: "redis-prefix"}
		}
	} else if s.client != nil {
		ensureSQLStore := newSQLDictionaryIndexStore(ctx, s.client)
		managed := newManagedDictionaryIndexStore(ensureSQLStore, nil)
		fingerprint, ready := s.externalIndexFingerprint(mdxPath)
		if !ready {
			ensureResult, err := mdx.EnsureDictionaryIndex(mdxPath, managed,
				mdx.WithReuseIfUnchanged(true),
				mdx.WithIndexDictionaryName(indexKey),
			)
			if err != nil {
				return nil, dictionaryMeta{}, err
			}
			if err := s.deleteLegacyExternalIndexes(ctx, &ent.Dictionary{Slug: slug, MdxPath: mdxPath}, indexKey); err != nil {
				return nil, dictionaryMeta{}, err
			}
			log.Printf("dictionary index sync name=%s reused=%t rebuilt=%t schema=%s source=%s", ensureResult.DictionaryName, ensureResult.Reused, ensureResult.Rebuilt, ensureResult.Manifest.SchemaVersion, ensureResult.Manifest.SourcePath)
			s.markExternalIndexReady(mdxPath, fingerprint, ensureResult.Manifest.Fingerprint)
		}
		sqlStore := newSQLDictionaryIndexStore(context.Background(), s.client)
		prefixStore = sqlStore
		managedStore = newManagedDictionaryIndexStore(sqlStore, nil)
		fuzzyStore = sqlStore
	} else {
		entries, err = mdxDict.ExportIndex()
		if err != nil {
			return nil, dictionaryMeta{}, err
		}
		prefixStore = mdx.NewMemoryIndexStore()
		if err := prefixStore.Put(info, entries); err != nil {
			return nil, dictionaryMeta{}, err
		}
		fallbackFuzzyStore := mdx.NewMemoryFuzzyIndexStore()
		if err := fallbackFuzzyStore.Put(info, entries); err != nil {
			return nil, dictionaryMeta{}, err
		}
		fuzzyStore = fallbackFuzzyStore
	}

	loaded := &LoadedDictionary{MDX: mdxDict, FuzzyStore: fuzzyStore, PrefixStore: prefixStore, ManagedIndexStore: managedStore, Entries: entries, Info: info, RediSearchEnabled: rediSearchEnabled, slug: slug, mdxPath: mdxPath, indexKey: indexKey, mddPaths: mddPaths}
	// MDDs are loaded lazily via ensureMDDsLoaded on the first resource request.

	meta := dictionaryMeta{
		Name:        firstNonEmpty(mdxDict.Name(), filepath.Base(mdxPath)),
		Title:       firstNonEmpty(strings.TrimSpace(mdxDict.Title()), mdxDict.Name()),
		Description: strings.TrimSpace(mdxDict.Description()),
		Slug:        slug,
		EntryCount:  int(info.EntryCount),
	}
	return loaded, meta, nil
}

func externalIndexKey(mdxPath string) string {
	path := filepath.Clean(strings.TrimSpace(mdxPath))
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	digest := sha256.Sum256([]byte(path))
	return fmt.Sprintf("owl-%x", digest)
}

func legacyExternalIndexKey(mdxPath string) string {
	base := filepath.Base(filepath.Clean(strings.TrimSpace(mdxPath)))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// deleteLegacyExternalIndexes removes the pre-path-hash derived views only
// after the replacement namespace is healthy. New Owl versions never read the
// legacy basename/slug namespaces, so retaining them would only duplicate the
// index in SQL or Valkey after an upgrade.
func (s *Service) deleteLegacyExternalIndexes(ctx context.Context, item *ent.Dictionary, currentKey string) error {
	if s == nil || item == nil || !s.hasExternalIndexStore() {
		return nil
	}
	s.mu.RLock()
	_, cleaned := s.legacyIndexCleaned[item.MdxPath]
	s.mu.RUnlock()
	if cleaned {
		return nil
	}

	legacyKeys := []string{legacyExternalIndexKey(item.MdxPath), item.Slug}
	prefixStore := s.newManagedPrefixStore(ctx)
	seen := make(map[string]struct{}, len(legacyKeys))
	ownedLegacyKeys := make([]string, 0, len(legacyKeys))
	for _, legacyKey := range legacyKeys {
		legacyKey = sanitizeManagedDictionaryName(legacyKey)
		if legacyKey == "" || legacyKey == sanitizeManagedDictionaryName(currentKey) {
			continue
		}
		if _, ok := seen[legacyKey]; ok {
			continue
		}
		seen[legacyKey] = struct{}{}
		manifest, err := prefixStore.LoadManifest(legacyKey)
		if errors.Is(err, mdx.ErrIndexMiss) {
			continue
		}
		if err != nil {
			return fmt.Errorf("load legacy index manifest %q: %w", legacyKey, err)
		}
		if !sameDictionaryPath(manifest.SourcePath, item.MdxPath) {
			continue
		}
		ownedLegacyKeys = append(ownedLegacyKeys, legacyKey)
	}

	if len(ownedLegacyKeys) > 0 && s.legacySlugBelongsToDictionary(ctx, item) {
		legacySearch := s.newRedisSearchCleanupStore(ctx, item.Slug)
		if legacySearch != nil && legacySearch.indexName != newRedisSearchStore(s.redisClient, firstNonEmpty(s.redisSearchKeyPrefix, "owl:mdx:search"), currentKey).indexName {
			if err := s.deleteRedisSearchIndex(ctx, legacySearch, item.Slug); err != nil {
				return fmt.Errorf("delete legacy RediSearch index %q: %w", item.Slug, err)
			}
		}
	}
	for _, legacyKey := range ownedLegacyKeys {
		if err := prefixStore.DeleteDictionary(legacyKey); err != nil {
			return fmt.Errorf("delete legacy prefix index %q: %w", legacyKey, err)
		}
	}
	s.mu.Lock()
	s.legacyIndexCleaned[item.MdxPath] = struct{}{}
	s.mu.Unlock()
	return nil
}

func (s *Service) deleteRedisSearchIndex(ctx context.Context, store *redisSearchStore, dictionaryName string) error {
	if s.deleteRedisSearchForTest != nil {
		return s.deleteRedisSearchForTest(ctx, dictionaryName)
	}
	if store == nil {
		return nil
	}
	return store.DeleteDictionary(dictionaryName)
}

func (s *Service) legacySlugBelongsToDictionary(ctx context.Context, item *ent.Dictionary) bool {
	if s == nil || s.client == nil || item == nil || strings.TrimSpace(item.Slug) == "" {
		return false
	}
	items, err := s.client.Dictionary.Query().
		Where(entdict.SlugEQ(item.Slug)).
		Select(entdict.FieldMdxPath).
		Limit(2).
		All(ctx)
	return err == nil && len(items) == 1 && sameDictionaryPath(items[0].MdxPath, item.MdxPath)
}

func sameDictionaryPath(left, right string) bool {
	normalize := func(path string) string {
		path = filepath.Clean(strings.TrimSpace(path))
		if absolute, err := filepath.Abs(path); err == nil {
			return absolute
		}
		return path
	}
	return normalize(left) == normalize(right)
}

func loadedIndexKey(loaded *LoadedDictionary) string {
	if loaded == nil {
		return ""
	}
	return firstNonEmpty(loaded.indexKey, loaded.Info.Name, loaded.slug)
}

func (s *Service) externalIndexFingerprint(path string) (string, bool) {
	now := s.now()
	s.mu.RLock()
	entry, ok := s.externalIndexReady[path]
	s.mu.RUnlock()
	if ok && !now.Before(entry.validatedAt) && now.Sub(entry.validatedAt) < s.externalIndexCacheTTL {
		return entry.fingerprint, true
	}
	fingerprint, err := mdx.NewFileStatFingerprinter().Fingerprint(path)
	if err != nil {
		return "", false
	}
	return fingerprint, false
}

func (s *Service) markExternalIndexReady(path, fingerprint, manifestFingerprint string) {
	if fingerprint == "" {
		fingerprint = manifestFingerprint
	}
	if fingerprint == "" {
		return
	}
	s.mu.Lock()
	s.externalIndexReady[path] = externalIndexCacheEntry{fingerprint: fingerprint, validatedAt: s.now()}
	s.mu.Unlock()
}

func toSummary(item *ent.Dictionary) models.DictionarySummary {
	mddPaths := decodePaths(item.MddPathsJSON)
	if mddPaths == nil {
		mddPaths = []string{}
	}
	fileStatus, missingFiles := assessDictionaryFiles(item.MdxPath, mddPaths)
	if missingFiles == nil {
		missingFiles = []string{}
	}
	summary := models.DictionarySummary{
		ID:           item.ID,
		Name:         item.Name,
		Title:        item.Title,
		Description:  item.Description,
		EntryCount:   item.EntryCount,
		Enabled:      item.Enabled,
		Public:       item.Public,
		FileStatus:   fileStatus,
		MissingFiles: missingFiles,
		MdxPath:      item.MdxPath,
		MddPaths:     mddPaths,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    item.UpdatedAt.Format(time.RFC3339),
	}
	if item.Edges.Owner != nil {
		summary.OwnerID = item.Edges.Owner.ID
		summary.OwnerName = item.Edges.Owner.Username
	}
	return summary
}

func ptrSummary(item *ent.Dictionary) *models.DictionarySummary {
	s := toSummary(item)
	return &s
}

func decodePaths(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func saveUploadedFile(file *multipart.FileHeader, dir string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	name := filepath.Base(file.Filename)
	path := filepath.Join(dir, name)
	dst, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := dst.ReadFrom(src); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	if value == "" {
		return fmt.Sprintf("dict-%d", time.Now().UnixNano())
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func displayName(item *ent.Dictionary) string {
	if strings.TrimSpace(item.Title) != "" {
		return item.Title
	}
	return item.Name
}

func visibilityLabel(public bool) string {
	if public {
		return "public"
	}
	return "private"
}

func debugScopeLabel(userID int, guest bool) string {
	switch {
	case guest || userID == 0:
		return "guest-public"
	default:
		return "user-accessible"
	}
}

func (s *Service) fuzzyBackendName(loaded *LoadedDictionary) string {
	if loaded == nil || loaded.FuzzyStore == nil {
		return "none"
	}
	if loaded.RediSearchEnabled && s.rediSearchAvailable() {
		return "redisearch"
	}
	if loaded.RediSearchEnabled && loaded.PrefixStore != nil {
		return "redis-prefix"
	}
	if store, ok := loaded.FuzzyStore.(redisPrefixFuzzyStore); ok {
		return store.sourceName()
	}
	if _, ok := loaded.FuzzyStore.(*sqlDictionaryIndexStore); ok {
		return "sql-index"
	}
	return "memory-fuzzy"
}

func prefixBackendName(loaded *LoadedDictionary) string {
	if loaded == nil || loaded.PrefixStore == nil {
		return "none"
	}
	if _, ok := loaded.PrefixStore.(*managedDictionaryIndexStore); ok && !loaded.RediSearchEnabled {
		return "sql-prefix"
	}
	return "redis-prefix"
}

func resultRank(result models.SearchResult, params SearchParams) int {
	query := strings.ToLower(strings.TrimSpace(params.Query))
	word := strings.ToLower(strings.TrimSpace(result.Word))
	switch {
	case word == query:
		return 0
	case strings.HasPrefix(word, query):
		return 1
	case strings.Contains(word, query):
		return 2
	default:
		return 3
	}
}

func suggestionRank(word string, query string) int {
	normalizedWord := strings.ToLower(strings.TrimSpace(word))
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	switch {
	case normalizedWord == normalizedQuery:
		return 0
	case strings.HasPrefix(normalizedWord, normalizedQuery):
		return 1
	case strings.Contains(normalizedWord, normalizedQuery):
		return 2
	default:
		return 3
	}
}

func prefixScore(query string, word string) float64 {
	switch suggestionRank(word, query) {
	case 0:
		return 1.0
	case 1:
		return 0.95
	case 2:
		return 0.8
	default:
		return 0.5
	}
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizeLoadedDictionaryLimit(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func discoverPairedMDDs(mdxPath string, existing []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(existing)+1)
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}

	for _, path := range existing {
		add(path)
	}

	base := strings.TrimSuffix(mdxPath, filepath.Ext(mdxPath))
	matches, _ := filepath.Glob(base + ".mdd")
	for _, path := range matches {
		add(path)
	}
	return out
}

func assessDictionaryFiles(mdxPath string, mddPaths []string) (string, []string) {
	missing := make([]string, 0)
	mdxMissing := false
	if _, err := os.Stat(mdxPath); err != nil {
		mdxMissing = true
		missing = append(missing, mdxPath)
	}

	missingMDD := false
	for _, path := range mddPaths {
		if _, err := os.Stat(path); err != nil {
			missingMDD = true
			missing = append(missing, path)
		}
	}

	switch {
	case mdxMissing && (missingMDD || len(mddPaths) == 0):
		return "missing_all", missing
	case mdxMissing:
		return "missing_mdx", missing
	case missingMDD:
		return "missing_mdd", missing
	default:
		return "ok", nil
	}
}

type dictionaryPair struct {
	MDXPath  string
	MDDPaths []string
}

func scanDictionaryPairs(root string) ([]dictionaryPair, error) {
	type fileInfo struct {
		base string
		path string
		ext  string
	}

	files := make([]fileInfo, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".mdx" && ext != ".mdd" {
			return nil
		}
		files = append(files, fileInfo{
			base: strings.TrimSuffix(path, filepath.Ext(path)),
			path: path,
			ext:  ext,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	mdxByBase := make(map[string]string)
	mddByBase := make(map[string][]string)
	for _, file := range files {
		switch file.ext {
		case ".mdx":
			mdxByBase[file.base] = file.path
		case ".mdd":
			mddByBase[file.base] = append(mddByBase[file.base], file.path)
		}
	}
	out := make([]dictionaryPair, 0, len(mdxByBase))
	for base, mdxPath := range mdxByBase {
		out = append(out, dictionaryPair{
			MDXPath:  mdxPath,
			MDDPaths: discoverPairedMDDs(mdxPath, mddByBase[base]),
		})
	}
	return out, nil
}

func (s *Service) upsertDictionaryFromPair(ctx context.Context, pair dictionaryPair, userID int, isAdmin bool) (*models.DictionarySummary, string, error) {
	loaded, meta, err := s.buildLoadedDictionary(ctx, pair.MDXPath, pair.MDDPaths)
	if err != nil {
		return nil, "failed", err
	}
	rawPaths, err := json.Marshal(pair.MDDPaths)
	if err != nil {
		return nil, "failed", err
	}

	query := s.client.Dictionary.Query().Where(entdict.MdxPathEQ(pair.MDXPath))
	if !isAdmin {
		query = query.Where(entdict.HasOwnerWith(entuser.IDEQ(userID)))
	}
	existing, err := query.Only(ctx)
	if err == nil {
		updated, updateErr := s.client.Dictionary.UpdateOneID(existing.ID).
			SetName(meta.Name).
			SetTitle(meta.Title).
			SetDescription(meta.Description).
			SetSlug(meta.Slug).
			SetMddPathsJSON(string(rawPaths)).
			SetEntryCount(meta.EntryCount).
			Save(ctx)
		if updateErr != nil {
			return nil, "failed", updateErr
		}
		s.mu.Lock()
		s.loaded[updated.ID] = loaded
		s.mu.Unlock()
		return ptrSummary(updated), "updated", nil
	}

	created, createErr := s.client.Dictionary.Create().
		SetName(meta.Name).
		SetTitle(meta.Title).
		SetDescription(meta.Description).
		SetSlug(meta.Slug).
		SetMdxPath(pair.MDXPath).
		SetMddPathsJSON(string(rawPaths)).
		SetEntryCount(meta.EntryCount).
		SetPublic(true).
		SetOwnerID(userID).
		Save(ctx)
	if createErr != nil {
		return nil, "failed", createErr
	}
	s.mu.Lock()
	s.loaded[created.ID] = loaded
	s.mu.Unlock()
	return ptrSummary(created), "discovered", nil
}

func maintenanceMessage(action string) string {
	switch action {
	case "discovered":
		return "New dictionary discovered and imported"
	case "updated":
		return "Existing dictionary refreshed from local files"
	default:
		return "No changes applied"
	}
}

func detectResourceContentType(path string, data []byte) string {
	lowerExt := strings.ToLower(filepath.Ext(path))
	switch lowerExt {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg", ".spx":
		return "audio/ogg"
	case ".snd":
		return detectSndContentType(data)
	}
	return http.DetectContentType(data)
}

func detectSndContentType(data []byte) string {
	if len(data) >= 4 {
		if bytes.Equal(data[:4], []byte("RIFF")) && bytes.Contains(data[:16], []byte("WAVE")) {
			return "audio/wav"
		}
		if bytes.Equal(data[:3], []byte("ID3")) {
			return "audio/mpeg"
		}
		if data[0] == 0xFF && len(data) > 1 && (data[1]&0xE0) == 0xE0 {
			return "audio/mpeg"
		}
		if bytes.Equal(data[:4], []byte("OggS")) {
			return "audio/ogg"
		}
	}
	return "application/octet-stream"
}
