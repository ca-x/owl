package dictionary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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
)

type Service struct {
	client     *ent.Client
	uploadsDir string
	mu         sync.RWMutex
	loaded     map[int]*LoadedDictionary
}

type LoadedDictionary struct {
	MDX        *mdx.Mdict
	MDDs       []*mdx.Mdict
	FuzzyStore *mdx.MemoryFuzzyIndexStore
	Entries    []mdx.IndexEntry
}

type SearchParams struct {
	UserID int
	IsAdmin bool
	Query string
	DictionaryID int
}

func NewService(client *ent.Client, uploadsDir string) *Service {
	return &Service{client: client, uploadsDir: uploadsDir, loaded: make(map[int]*LoadedDictionary)}
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
	loaded, meta, err := buildLoadedDictionary(mdxPath, mddPaths)
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
		query = query.Where(entdict.HasOwnerWith(entuser.IDEQ(params.UserID)))
	}
	dicts, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]models.SearchResult, 0)
	for _, item := range dicts {
		loaded, err := s.ensureLoaded(item)
		if err != nil {
			continue
		}
		hits, err := loaded.FuzzyStore.Search(item.Slug, params.Query, 10)
		if err != nil {
			continue
		}
		for _, hit := range hits {
			htmlBytes, err := loaded.MDX.Resolve(hit.Entry)
			if err != nil {
				continue
			}
			assetBase := fmt.Sprintf("/api/dictionaries/%d/resource", item.ID)
			html := string(mdx.RewriteEntryResourceURLs(htmlBytes, assetBase))
			results = append(results, models.SearchResult{
				DictionaryID:   item.ID,
				DictionaryName: displayName(item),
				Word:           hit.Entry.Keyword,
				HTML:           html,
				Score:          hit.Score,
				Source:         hit.Source,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].DictionaryName == results[j].DictionaryName {
			return results[i].Score > results[j].Score
		}
		return results[i].DictionaryName < results[j].DictionaryName
	})
	return results, nil
}

func (s *Service) OpenResource(ctx context.Context, id int, userID int, isAdmin bool, resourcePath string) ([]byte, string, error) {
	item, err := s.getOwnedDictionary(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, "", err
	}
	loaded, err := s.ensureLoaded(item)
	if err != nil {
		return nil, "", err
	}
	resourcePath = strings.TrimPrefix(resourcePath, "/")
	for _, dict := range loaded.MDDs {
		file, err := mdx.NewMdictFS(dict).Open(resourcePath)
		if err != nil {
			continue
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			continue
		}
		return data, http.DetectContentType(data), nil
	}
	return nil, "", fmt.Errorf("resource not found")
}

func (s *Service) ensureLoaded(item *ent.Dictionary) (*LoadedDictionary, error) {
	s.mu.RLock()
	loaded, ok := s.loaded[item.ID]
	s.mu.RUnlock()
	if ok {
		return loaded, nil
	}
	fresh, _, err := buildLoadedDictionary(item.MdxPath, decodePaths(item.MddPathsJSON))
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

type dictionaryMeta struct {
	Name string
	Title string
	Description string
	Slug string
	EntryCount int
}

func buildLoadedDictionary(mdxPath string, mddPaths []string) (*LoadedDictionary, dictionaryMeta, error) {
	mdxDict, err := mdx.New(mdxPath)
	if err != nil {
		return nil, dictionaryMeta{}, err
	}
	if err := mdxDict.BuildIndex(); err != nil {
		return nil, dictionaryMeta{}, err
	}
	entries, err := mdxDict.ExportEntries()
	if err != nil {
		return nil, dictionaryMeta{}, err
	}
	store := mdx.NewMemoryFuzzyIndexStore()
	info := mdxDict.DictionaryInfo()
	slug := sanitizeSlug(firstNonEmpty(info.Name, mdxDict.Name(), strings.TrimSuffix(filepath.Base(mdxPath), filepath.Ext(mdxPath))))
	info.Name = slug
	if err := store.Put(info, entries); err != nil {
		return nil, dictionaryMeta{}, err
	}
	loaded := &LoadedDictionary{MDX: mdxDict, FuzzyStore: store, Entries: entries}
	for _, mddPath := range mddPaths {
		mddDict, err := mdx.New(mddPath)
		if err != nil {
			return nil, dictionaryMeta{}, err
		}
		if err := mddDict.BuildIndex(); err != nil {
			return nil, dictionaryMeta{}, err
		}
		loaded.MDDs = append(loaded.MDDs, mddDict)
	}
	meta := dictionaryMeta{
		Name: firstNonEmpty(mdxDict.Name(), filepath.Base(mdxPath)),
		Title: firstNonEmpty(strings.TrimSpace(mdxDict.Title()), mdxDict.Name()),
		Description: strings.TrimSpace(mdxDict.Description()),
		Slug: slug,
		EntryCount: int(info.EntryCount),
	}
	return loaded, meta, nil
}

func toSummary(item *ent.Dictionary) models.DictionarySummary {
	summary := models.DictionarySummary{
		ID: item.ID,
		Name: item.Name,
		Title: item.Title,
		Description: item.Description,
		EntryCount: item.EntryCount,
		Enabled: item.Enabled,
		MdxPath: item.MdxPath,
		MddPaths: decodePaths(item.MddPathsJSON),
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
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
