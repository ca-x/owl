package dictionary

import (
	"bytes"
	"context"
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
	"sort"
	"strings"
	"sync"
	"time"

	"owl/backend/ent"
	entdict "owl/backend/ent/dictionary"
	entuser "owl/backend/ent/user"
	"owl/backend/internal/models"

	"github.com/lib-x/mdx"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	client               *ent.Client
	uploadsDir           string
	libraryDir           string
	redisClient          *redis.Client
	redisKeyPrefix       string
	redisPrefixMaxLen    int
	redisSearchKeyPrefix string
	redisSearchEnabled   bool
	audioTranscoder      *audioTranscoder
	mu                   sync.RWMutex
	loaded               map[int]*LoadedDictionary
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
}

type SearchParams struct {
	UserID       int
	IsAdmin      bool
	Query        string
	DictionaryID int
	Guest        bool
}

func NewService(client *ent.Client, uploadsDir string, libraryDir string, redisClient *redis.Client, redisKeyPrefix string, redisPrefixMaxLen int, redisSearchKeyPrefix string, redisSearchEnabled bool, audioCacheDir string, ffmpegBin string) *Service {
	return &Service{
		client:               client,
		uploadsDir:           uploadsDir,
		libraryDir:           libraryDir,
		redisClient:          redisClient,
		redisKeyPrefix:       strings.TrimSpace(redisKeyPrefix),
		redisPrefixMaxLen:    redisPrefixMaxLen,
		redisSearchKeyPrefix: strings.TrimSpace(redisSearchKeyPrefix),
		redisSearchEnabled:   redisSearchEnabled,
		audioTranscoder:      newAudioTranscoder(audioCacheDir, firstNonEmpty(strings.TrimSpace(ffmpegBin), resolveFFmpegBinary())),
		loaded:               make(map[int]*LoadedDictionary),
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
	for _, item := range dicts {
		if _, err := s.ensureLoaded(item); err != nil {
			return err
		}
	}
	return nil
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
		RedisSearchEnabled: s.redisSearchEnabled,
		Scope:              debugScopeLabel(userID, guest),
		Dictionaries:       make([]models.SearchBackendDictionary, 0, len(dicts)),
	}
	for _, item := range dicts {
		loaded, err := s.ensureLoaded(item)
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
			FuzzyBackend:   fuzzyBackendName(loaded),
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
	loaded, meta, err := s.buildLoadedDictionary(mdxPath, mddPaths)
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
	s.loaded[created.ID] = loaded
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
	results := make([]models.SearchResult, 0)
	seen := make(map[string]struct{})
	for _, item := range dicts {
		loaded, err := s.ensureLoaded(item)
		if err != nil {
			continue
		}

		if exactEntry, ok := loaded.MDX.FindExactEntry(params.Query); ok && exactEntry != nil {
			result, buildErr := buildSearchResult(item, loaded, mdx.IndexEntry{
				Keyword:           exactEntry.KeyWord,
				RecordStartOffset: exactEntry.RecordStartOffset,
				RecordEndOffset:   exactEntry.RecordEndOffset,
				KeyBlockIdx:       exactEntry.KeyBlockIdx,
			}, 1.0, "exact")
			if buildErr == nil {
				key := fmt.Sprintf("%d:%s", item.ID, strings.ToLower(result.Word))
				seen[key] = struct{}{}
				results = append(results, result)
			}
		} else if comparableEntry, ok := loaded.MDX.FindComparableEntry(params.Query); ok && comparableEntry != nil {
			result, buildErr := buildSearchResult(item, loaded, mdx.IndexEntry{
				Keyword:           comparableEntry.KeyWord,
				RecordStartOffset: comparableEntry.RecordStartOffset,
				RecordEndOffset:   comparableEntry.RecordEndOffset,
				KeyBlockIdx:       comparableEntry.KeyBlockIdx,
			}, 0.99, "comparable")
			if buildErr == nil {
				key := fmt.Sprintf("%d:%s", item.ID, strings.ToLower(result.Word))
				seen[key] = struct{}{}
				results = append(results, result)
			}
		}

		hits, err := loaded.FuzzyStore.Search(item.Slug, params.Query, 10)
		if err != nil {
			continue
		}
		for _, hit := range hits {
			key := fmt.Sprintf("%d:%s", item.ID, strings.ToLower(hit.Entry.Keyword))
			if _, exists := seen[key]; exists {
				continue
			}
			result, buildErr := buildSearchResult(item, loaded, hit.Entry, hit.Score, hit.Source)
			if buildErr != nil {
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

func buildSearchResult(item *ent.Dictionary, loaded *LoadedDictionary, entry mdx.IndexEntry, score float64, source string) (models.SearchResult, error) {
	htmlContent, err := resolveEntryHTML(loaded, entry, 0, nil)
	if err != nil {
		return models.SearchResult{}, err
	}
	assetBase := fmt.Sprintf("/api/dictionaries/%d/resource", item.ID)
	if item.Public {
		assetBase = fmt.Sprintf("/api/public/dictionaries/%d/resource", item.ID)
	}
	rewritten := mdx.RewriteEntryResourceURLs([]byte(htmlContent), assetBase)
	rewritten = mdx.RewriteEntryInternalLinks(rewritten)
	rewritten = mdx.RewriteEntryLookupLinks(rewritten, "/search?q=")
	rewritten = mdx.RewriteEntryAudioLinks(rewritten, assetBase)
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
	if depth > 6 {
		return "", fmt.Errorf("link depth exceeded")
	}
	content, err := loaded.MDX.Resolve(entry)
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

	targetContent, lookupErr := lookupEntryContent(loaded, target)
	if lookupErr != nil {
		return fmt.Sprintf("<p>%s</p>", html.EscapeString(target)), nil
	}

	targetText := strings.TrimSpace(string(targetContent))
	if strings.HasPrefix(targetText, "@@@LINK=") {
		nextEntry := mdx.IndexEntry{Keyword: target}
		return resolveEntryHTML(loaded, nextEntry, depth+1, seen)
	}
	return targetText, nil
}

func lookupEntryContent(loaded *LoadedDictionary, word string) ([]byte, error) {
	if loaded == nil || loaded.MDX == nil {
		return nil, fmt.Errorf("dictionary not loaded")
	}
	content, err := loaded.MDX.Lookup(word)
	if err == nil {
		return content, nil
	}
	if loaded.PrefixStore == nil {
		return nil, err
	}
	entry, indexErr := loaded.PrefixStore.GetExact(loaded.Info.Name, strings.TrimSpace(word))
	if indexErr != nil {
		return nil, err
	}
	return loaded.MDX.Resolve(entry)
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

	for _, item := range dicts {
		loaded, loadErr := s.ensureLoaded(item)
		if loadErr != nil {
			continue
		}

		var hits []mdx.SearchHit
		if loaded.FuzzyStore != nil {
			searchHits, searchErr := loaded.FuzzyStore.Search(item.Slug, params.Query, max(limit*3, limit))
			if searchErr == nil {
				hits = searchHits
			} else if loaded.RediSearchEnabled && !errors.Is(searchErr, mdx.ErrIndexMiss) && !isRediSearchUnavailable(searchErr) {
				continue
			}
		}
		if len(hits) == 0 && loaded.PrefixStore != nil {
			entries, prefixErr := loaded.PrefixStore.PrefixSearch(item.Slug, params.Query, max(limit*4, limit))
			if prefixErr == nil {
				for _, entry := range entries {
					hits = append(hits, mdx.SearchHit{Entry: entry, Score: prefixScore(params.Query, entry.Keyword), Source: "redis-prefix"})
				}
			} else if !errors.Is(prefixErr, mdx.ErrIndexMiss) {
				continue
			}
		}
		for _, hit := range hits {
			word := strings.TrimSpace(hit.Entry.Keyword)
			if word == "" {
				continue
			}

			normalizedWord := strings.ToLower(word)
			sourceKey := fmt.Sprintf("%s:%d", normalizedWord, item.ID)
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
				DictionaryID:   item.ID,
				DictionaryName: displayName(item),
				Visibility:     visibilityLabel(item.Public),
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

func (s *Service) OpenResource(ctx context.Context, id int, userID int, isAdmin bool, guest bool, resourcePath string) ([]byte, string, error) {
	item, err := s.getAccessibleDictionary(ctx, id, userID, isAdmin, guest)
	if err != nil {
		return nil, "", err
	}
	loaded, err := s.ensureLoaded(item)
	if err != nil {
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

func (s *Service) ensureLoaded(item *ent.Dictionary) (*LoadedDictionary, error) {
	s.mu.RLock()
	loaded, ok := s.loaded[item.ID]
	s.mu.RUnlock()
	if ok {
		return loaded, nil
	}
	fresh, _, err := s.buildLoadedDictionary(item.MdxPath, decodePaths(item.MddPathsJSON))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.loaded[item.ID] = fresh
	s.mu.Unlock()
	return fresh, nil
}

func (s *Service) unload(id int) {
	s.mu.Lock()
	delete(s.loaded, id)
	s.mu.Unlock()
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
	loaded, meta, err := s.buildLoadedDictionary(item.MdxPath, mddPaths)
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
	s.loaded[item.ID] = loaded
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
}

type dictionaryMeta struct {
	Name        string
	Title       string
	Description string
	Slug        string
	EntryCount  int
}

func (s *Service) buildLoadedDictionary(mdxPath string, mddPaths []string) (*LoadedDictionary, dictionaryMeta, error) {
	mdxDict, err := mdx.New(mdxPath)
	if err != nil {
		return nil, dictionaryMeta{}, err
	}
	if err := mdxDict.PrepareForExternalIndex(); err != nil {
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
	if s.redisClient != nil {
		redisPrefixStore := mdx.NewRedisIndexStore(s.redisClient,
			mdx.WithRedisIndexContext(context.Background()),
			mdx.WithRedisKeyPrefix(firstNonEmpty(s.redisKeyPrefix, "owl:mdx:index")),
			mdx.WithRedisPrefixIndexMaxLen(max(s.redisPrefixMaxLen, 1)),
		)
		var searchStore *redisSearchStore
		if s.redisSearchEnabled {
			searchStore = newRedisSearchStore(s.redisClient, firstNonEmpty(s.redisSearchKeyPrefix, "owl:mdx:search"), info.Name)
		}
		managed := newManagedDictionaryIndexStore(redisPrefixStore, searchStore)
		ensureResult, err := mdx.EnsureDictionaryIndex(mdxPath, managed, mdx.WithReuseIfUnchanged(true))
		if err != nil {
			return nil, dictionaryMeta{}, err
		}
		log.Printf("dictionary index sync name=%s reused=%t rebuilt=%t schema=%s source=%s", ensureResult.DictionaryName, ensureResult.Reused, ensureResult.Rebuilt, ensureResult.Manifest.SchemaVersion, ensureResult.Manifest.SourcePath)
		prefixStore = redisPrefixStore
		managedStore = managed
		if searchStore != nil {
			fuzzyStore = searchStore
			rediSearchEnabled = true
		} else {
			fuzzyStore = redisPrefixFuzzyStore{store: redisPrefixStore}
		}
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

	loaded := &LoadedDictionary{MDX: mdxDict, FuzzyStore: fuzzyStore, PrefixStore: prefixStore, ManagedIndexStore: managedStore, Entries: entries, Info: info, RediSearchEnabled: rediSearchEnabled}
	for _, mddPath := range discoverPairedMDDs(mdxPath, mddPaths) {
		mddDict, err := mdx.New(mddPath)
		if err != nil {
			return nil, dictionaryMeta{}, err
		}
		if err := mddDict.BuildIndex(); err != nil {
			return nil, dictionaryMeta{}, err
		}
		loaded.MDDs = append(loaded.MDDs, mddDict)
	}
	mdx.ConfigureDictionaryPairAssets(mdx.DictionarySpec{ID: slug, Name: slug, MDXPath: mdxPath, MDDPaths: discoverPairedMDDs(mdxPath, mddPaths)}, mdxDict, loaded.MDDs...)

	meta := dictionaryMeta{
		Name:        firstNonEmpty(mdxDict.Name(), filepath.Base(mdxPath)),
		Title:       firstNonEmpty(strings.TrimSpace(mdxDict.Title()), mdxDict.Name()),
		Description: strings.TrimSpace(mdxDict.Description()),
		Slug:        slug,
		EntryCount:  int(info.EntryCount),
	}
	return loaded, meta, nil
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

func fuzzyBackendName(loaded *LoadedDictionary) string {
	if loaded == nil || loaded.FuzzyStore == nil {
		return "none"
	}
	if loaded.RediSearchEnabled {
		return "redisearch"
	}
	if _, ok := loaded.FuzzyStore.(redisPrefixFuzzyStore); ok {
		return "redis-prefix"
	}
	return "memory-fuzzy"
}

func prefixBackendName(loaded *LoadedDictionary) string {
	if loaded == nil || loaded.PrefixStore == nil {
		return "none"
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
	loaded, meta, err := s.buildLoadedDictionary(pair.MDXPath, pair.MDDPaths)
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
