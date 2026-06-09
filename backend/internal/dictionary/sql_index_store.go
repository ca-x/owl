package dictionary

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"owl/backend/ent"
	entindexentry "owl/backend/ent/dictionaryindexentry"
	entindexmanifest "owl/backend/ent/dictionaryindexmanifest"
	"owl/backend/ent/predicate"

	"github.com/lib-x/mdx"
)

const sqlIndexInsertBatchSize = 500

type sqlDictionaryIndexStore struct {
	ctx    context.Context
	client *ent.Client
}

func newSQLDictionaryIndexStore(ctx context.Context, client *ent.Client) *sqlDictionaryIndexStore {
	if ctx == nil {
		ctx = context.Background()
	}
	return &sqlDictionaryIndexStore{ctx: ctx, client: client}
}

func (s *sqlDictionaryIndexStore) Put(info mdx.DictionaryInfo, entries []mdx.IndexEntry) error {
	if s == nil || s.client == nil {
		return errors.New("database client is required")
	}
	dictionaryName := sanitizeManagedDictionaryName(info.Name)
	if dictionaryName == "" {
		return errors.New("dictionary name is required")
	}
	tx, err := s.client.Tx(s.ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.DictionaryIndexEntry.Delete().
		Where(entindexentry.DictionaryNameEQ(dictionaryName)).
		Exec(s.ctx); err != nil {
		return err
	}

	batch := make([]*ent.DictionaryIndexEntryCreate, 0, sqlIndexInsertBatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := tx.DictionaryIndexEntry.CreateBulk(batch...).Exec(s.ctx)
		batch = batch[:0]
		return err
	}

	for _, entry := range entries {
		lookupKey := indexEntryLookupKey(entry)
		if strings.TrimSpace(lookupKey) == "" {
			continue
		}
		payload, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		batch = append(batch, tx.DictionaryIndexEntry.Create().
			SetDictionaryName(dictionaryName).
			SetKeyword(entry.Keyword).
			SetNormalizedKeyword(entry.NormalizedKeyword).
			SetLookupKey(lookupKey).
			SetLookupKeyLower(strings.ToLower(lookupKey)).
			SetRecordStartOffset(entry.RecordStartOffset).
			SetRecordEndOffset(entry.RecordEndOffset).
			SetKeyBlockIdx(entry.KeyBlockIdx).
			SetIsResource(entry.IsResource).
			SetPayload(string(payload)))
		if len(batch) >= sqlIndexInsertBatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *sqlDictionaryIndexStore) GetExact(dictionaryName, keyword string) (mdx.IndexEntry, error) {
	if s == nil || s.client == nil {
		return mdx.IndexEntry{}, mdx.ErrIndexMiss
	}
	item, err := s.client.DictionaryIndexEntry.Query().
		Where(
			entindexentry.DictionaryNameEQ(sanitizeManagedDictionaryName(dictionaryName)),
			entindexentry.LookupKeyEQ(strings.TrimSpace(keyword)),
		).
		First(s.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return mdx.IndexEntry{}, mdx.ErrIndexMiss
		}
		return mdx.IndexEntry{}, err
	}
	return decodeIndexEntryPayload(item.Payload)
}

func (s *sqlDictionaryIndexStore) PrefixSearch(dictionaryName, prefix string, limit int) ([]mdx.IndexEntry, error) {
	if s == nil || s.client == nil {
		return nil, mdx.ErrIndexMiss
	}
	prefixLower := strings.ToLower(strings.TrimSpace(prefix))
	query := s.client.DictionaryIndexEntry.Query().
		Where(entindexentry.DictionaryNameEQ(sanitizeManagedDictionaryName(dictionaryName))).
		Order(ent.Asc(entindexentry.FieldLookupKeyLower))
	if prefixLower != "" {
		query = query.Where(entindexentry.LookupKeyLowerHasPrefix(prefixLower))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	items, err := query.All(s.ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, mdx.ErrIndexMiss
	}
	out := make([]mdx.IndexEntry, 0, len(items))
	for _, item := range items {
		entry, err := decodeIndexEntryPayload(item.Payload)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *sqlDictionaryIndexStore) Search(dictionaryName, query string, limit int) ([]mdx.SearchHit, error) {
	if s == nil || s.client == nil {
		return nil, mdx.ErrIndexMiss
	}
	queryLower := strings.ToLower(strings.TrimSpace(query))
	if queryLower == "" {
		return nil, mdx.ErrIndexMiss
	}
	if limit <= 0 {
		limit = 8
	}

	hits := make([]mdx.SearchHit, 0, limit)
	seen := make(map[string]struct{}, limit)
	appendMatches := func(items []*ent.DictionaryIndexEntry, score float64, source string) error {
		for _, item := range items {
			if _, ok := seen[item.LookupKey]; ok {
				continue
			}
			entry, err := decodeIndexEntryPayload(item.Payload)
			if err != nil {
				return err
			}
			hits = append(hits, mdx.SearchHit{Entry: entry, Score: score, Source: source})
			seen[item.LookupKey] = struct{}{}
			if len(hits) >= limit {
				break
			}
		}
		return nil
	}

	exact, err := s.querySearchEntries(dictionaryName, limit, entindexentry.LookupKeyLowerEQ(queryLower))
	if err != nil {
		return nil, err
	}
	if err := appendMatches(exact, 1.0, "sql-exact"); err != nil {
		return nil, err
	}
	if len(hits) < limit {
		prefix, err := s.querySearchEntries(dictionaryName, limit, entindexentry.LookupKeyLowerHasPrefix(queryLower))
		if err != nil {
			return nil, err
		}
		if err := appendMatches(prefix, 0.95, "sql-prefix"); err != nil {
			return nil, err
		}
	}
	if len(hits) < limit {
		contains, err := s.querySearchEntries(dictionaryName, limit, entindexentry.LookupKeyLowerContains(queryLower))
		if err != nil {
			return nil, err
		}
		if err := appendMatches(contains, 0.8, "sql-contains"); err != nil {
			return nil, err
		}
	}
	if len(hits) == 0 {
		return nil, mdx.ErrIndexMiss
	}
	return hits, nil
}

func (s *sqlDictionaryIndexStore) querySearchEntries(dictionaryName string, limit int, predicates ...predicate.DictionaryIndexEntry) ([]*ent.DictionaryIndexEntry, error) {
	conditions := []predicate.DictionaryIndexEntry{entindexentry.DictionaryNameEQ(sanitizeManagedDictionaryName(dictionaryName))}
	conditions = append(conditions, predicates...)
	return s.client.DictionaryIndexEntry.Query().
		Where(conditions...).
		Order(ent.Asc(entindexentry.FieldLookupKeyLower)).
		Limit(limit).
		All(s.ctx)
}

func (s *sqlDictionaryIndexStore) LoadManifest(dictionaryName string) (mdx.IndexManifest, error) {
	if s == nil || s.client == nil {
		return mdx.IndexManifest{}, mdx.ErrIndexMiss
	}
	item, err := s.client.DictionaryIndexManifest.Query().
		Where(entindexmanifest.DictionaryNameEQ(sanitizeManagedDictionaryName(dictionaryName))).
		Only(s.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return mdx.IndexManifest{}, mdx.ErrIndexMiss
		}
		return mdx.IndexManifest{}, err
	}
	return mdx.IndexManifest{
		DictionaryName: item.DictionaryName,
		SourcePath:     item.SourcePath,
		Fingerprint:    item.Fingerprint,
		SchemaVersion:  item.SchemaVersion,
		BuiltAt:        item.BuiltAt,
		ExpiresAt:      item.ExpiresAt,
	}, nil
}

func (s *sqlDictionaryIndexStore) SaveManifest(manifest mdx.IndexManifest) error {
	if s == nil || s.client == nil {
		return errors.New("database client is required")
	}
	dictionaryName := sanitizeManagedDictionaryName(manifest.DictionaryName)
	if dictionaryName == "" {
		return errors.New("dictionary name is required")
	}
	existing, err := s.client.DictionaryIndexManifest.Query().
		Where(entindexmanifest.DictionaryNameEQ(dictionaryName)).
		Only(s.ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if ent.IsNotFound(err) {
		create := s.client.DictionaryIndexManifest.Create().
			SetDictionaryName(dictionaryName).
			SetSourcePath(manifest.SourcePath).
			SetFingerprint(manifest.Fingerprint).
			SetSchemaVersion(manifest.SchemaVersion).
			SetBuiltAt(manifest.BuiltAt)
		if manifest.ExpiresAt != nil {
			create = create.SetExpiresAt(*manifest.ExpiresAt)
		}
		return create.Exec(s.ctx)
	}
	update := s.client.DictionaryIndexManifest.UpdateOneID(existing.ID).
		SetSourcePath(manifest.SourcePath).
		SetFingerprint(manifest.Fingerprint).
		SetSchemaVersion(manifest.SchemaVersion).
		SetBuiltAt(manifest.BuiltAt)
	if manifest.ExpiresAt != nil {
		update = update.SetExpiresAt(*manifest.ExpiresAt)
	} else {
		update = update.ClearExpiresAt()
	}
	return update.Exec(s.ctx)
}

func (s *sqlDictionaryIndexStore) DeleteDictionary(dictionaryName string) error {
	if s == nil || s.client == nil {
		return nil
	}
	dictionaryName = sanitizeManagedDictionaryName(dictionaryName)
	tx, err := s.client.Tx(s.ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.DictionaryIndexEntry.Delete().
		Where(entindexentry.DictionaryNameEQ(dictionaryName)).
		Exec(s.ctx); err != nil {
		return err
	}
	if _, err := tx.DictionaryIndexManifest.Delete().
		Where(entindexmanifest.DictionaryNameEQ(dictionaryName)).
		Exec(s.ctx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func indexEntryLookupKey(entry mdx.IndexEntry) string {
	if entry.IsResource && entry.NormalizedKeyword != "" {
		return entry.NormalizedKeyword
	}
	return entry.Keyword
}

func decodeIndexEntryPayload(payload string) (mdx.IndexEntry, error) {
	var entry mdx.IndexEntry
	if err := json.Unmarshal([]byte(payload), &entry); err != nil {
		return mdx.IndexEntry{}, err
	}
	return entry, nil
}
