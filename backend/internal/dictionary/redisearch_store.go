package dictionary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib-x/mdx"
	"github.com/redis/go-redis/v9"
)

type redisSearchStore struct {
	ctx       context.Context
	client    *redis.Client
	indexName string
	docPrefix string
	keysSet   string
}

func newRedisSearchStore(client *redis.Client, keyPrefix string, dictionaryName string) *redisSearchStore {
	normalizedPrefix := strings.TrimSpace(keyPrefix)
	if normalizedPrefix == "" {
		normalizedPrefix = "owl:mdx:search"
	}
	dictKey := sanitizeRedisKey(dictionaryName)
	root := fmt.Sprintf("%s:%s", normalizedPrefix, dictKey)
	return &redisSearchStore{
		ctx:       context.Background(),
		client:    client,
		indexName: root + ":idx",
		docPrefix: root + ":doc:",
		keysSet:   root + ":keys",
	}
}

func (s *redisSearchStore) Put(info mdx.DictionaryInfo, entries []mdx.IndexEntry) error {
	if s == nil || s.client == nil {
		return errors.New("redis client is required")
	}
	if err := s.ensureIndex(); err != nil {
		return err
	}

	oldKeys, err := s.client.SMembers(s.ctx, s.keysSet).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if len(oldKeys) > 0 {
		if err := s.client.Del(s.ctx, oldKeys...).Err(); err != nil {
			return err
		}
	}
	if err := s.client.Del(s.ctx, s.keysSet).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	pipe := s.client.Pipeline()
	keys := make([]string, 0, len(entries))
	for idx, entry := range entries {
		keyword := strings.TrimSpace(entry.Keyword)
		if keyword == "" {
			continue
		}
		payload, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		key := s.docPrefix + strconv.Itoa(idx)
		keys = append(keys, key)
		pipe.HSet(s.ctx, key,
			"keyword", keyword,
			"normalized", strings.ToLower(keyword),
			"payload", string(payload),
		)
	}
	if len(keys) > 0 {
		members := make([]any, 0, len(keys))
		for _, key := range keys {
			members = append(members, key)
		}
		pipe.SAdd(s.ctx, s.keysSet, members...)
	}
	_, err = pipe.Exec(s.ctx)
	return err
}

func (s *redisSearchStore) Search(_ string, query string, limit int) ([]mdx.SearchHit, error) {
	if s == nil || s.client == nil {
		return nil, mdx.ErrIndexMiss
	}
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return nil, mdx.ErrIndexMiss
	}
	if limit <= 0 {
		limit = 8
	}

	searchQuery := buildRediSearchQuery(normalized)
	args := []any{
		"FT.SEARCH", s.indexName, searchQuery,
		"WITHSCORES",
		"LIMIT", 0, limit,
		"RETURN", 2, "keyword", "payload",
	}
	raw, err := s.client.Do(s.ctx, args...).Result()
	if err != nil {
		if isRediSearchUnavailable(err) {
			return nil, err
		}
		if strings.Contains(strings.ToLower(err.Error()), "unknown index name") {
			return nil, mdx.ErrIndexMiss
		}
		return nil, err
	}

	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil, mdx.ErrIndexMiss
	}
	count, _ := asInt(items[0])
	if count == 0 {
		return nil, mdx.ErrIndexMiss
	}

	hits := make([]mdx.SearchHit, 0, limit)
	for i := 1; i+2 < len(items); i += 3 {
		score, _ := strconv.ParseFloat(asString(items[i+1]), 64)
		fields, ok := items[i+2].([]any)
		if !ok {
			continue
		}
		fieldMap := asFieldMap(fields)
		payload := fieldMap["payload"]
		if payload == "" {
			continue
		}
		var entry mdx.IndexEntry
		if err := json.Unmarshal([]byte(payload), &entry); err != nil {
			continue
		}
		hits = append(hits, mdx.SearchHit{
			Entry:  entry,
			Score:  score,
			Source: "redisearch",
		})
		if limit > 0 && len(hits) >= limit {
			break
		}
	}
	if len(hits) == 0 {
		return nil, mdx.ErrIndexMiss
	}
	return hits, nil
}

func (s *redisSearchStore) ensureIndex() error {
	args := []any{
		"FT.CREATE", s.indexName,
		"ON", "HASH",
		"PREFIX", 1, s.docPrefix,
		"SCHEMA",
		"keyword", "TEXT", "NOSTEM", "WEIGHT", 5.0,
		"normalized", "TEXT", "NOSTEM", "WEIGHT", 3.0,
		"payload", "TEXT", "NOINDEX",
	}
	if err := s.client.Do(s.ctx, args...).Err(); err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "index already exists") {
			return nil
		}
		return err
	}
	return nil
}

func buildRediSearchQuery(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.Fields(normalized)
	if len(tokens) == 0 {
		return "*"
	}

	phrase := fmt.Sprintf("@keyword:\"%s\"", escapeRediSearchPhrase(query))
	clauses := []string{phrase}

	prefixTerms := make([]string, 0, len(tokens))
	fuzzyOneTerms := make([]string, 0, len(tokens))
	fuzzyTwoTerms := make([]string, 0, len(tokens))
	for _, token := range tokens {
		escaped := escapeRediSearchTerm(token)
		if escaped == "" {
			continue
		}
		prefixTerms = append(prefixTerms, escaped+"*")
		fuzzyOneTerms = append(fuzzyOneTerms, "%"+escaped+"%")
		fuzzyTwoTerms = append(fuzzyTwoTerms, "%%"+escaped+"%%")
	}
	if len(prefixTerms) > 0 {
		clauses = append(clauses, "@normalized:("+strings.Join(prefixTerms, " ")+")")
	}
	if len(fuzzyOneTerms) > 0 {
		clauses = append(clauses, "@normalized:("+strings.Join(fuzzyOneTerms, " ")+")")
	}
	if len(fuzzyTwoTerms) > 0 {
		clauses = append(clauses, "@normalized:("+strings.Join(fuzzyTwoTerms, " ")+")")
	}
	return "(" + strings.Join(clauses, ") | (") + ")"
}

func isRediSearchUnavailable(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unknown command 'ft.") ||
		strings.Contains(lower, "unknown command `ft.") ||
		strings.Contains(lower, "no such module") ||
		strings.Contains(lower, "module disabled")
}

func sanitizeRedisKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		":", "_",
		"/", "_",
		"\\", "_",
		" ", "_",
		"*", "_",
		"?", "_",
		"[", "_",
		"]", "_",
		"(", "_",
		")", "_",
		"{", "_",
		"}", "_",
		"|", "_",
		"@", "_",
		"-", "_",
	)
	value = replacer.Replace(value)
	if value == "" {
		return "default"
	}
	return value
}

func escapeRediSearchTerm(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"-", "\\-",
		"@", "\\@",
		"~", "\\~",
		"*", "\\*",
		"%", "\\%",
		"'", "\\'",
		"\"", "\\\"",
		":", "\\:",
		";", "\\;",
		"!", "\\!",
		"(", "\\(",
		")", "\\)",
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
		"|", "\\|",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func escapeRediSearchPhrase(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "\"", "\\\"")
}

func asFieldMap(fields []any) map[string]string {
	out := make(map[string]string, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		out[asString(fields[i])] = asString(fields[i+1])
	}
	return out
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func asInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case uint64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	case []byte:
		return strconv.Atoi(string(v))
	default:
		return 0, fmt.Errorf("unexpected int type %T", value)
	}
}
