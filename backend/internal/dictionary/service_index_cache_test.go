package dictionary

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"owl/backend/ent"
	entindexentry "owl/backend/ent/dictionaryindexentry"

	_ "github.com/lib-x/entsqlite"
	"github.com/lib-x/mdx"
	"github.com/redis/go-redis/v9"
)

func TestEnsureExternalIndexCachesValidatedFingerprintUntilTTL(t *testing.T) {
	ctx := t.Context()
	client, err := ent.Open("sqlite3", "file:index_ready_cache?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "demo.mdx")
	if err := os.WriteFile(path, []byte("not parsed when manifest is reused"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := mdx.NewFileStatFingerprinter().Fingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	indexKey := externalIndexKey(path)
	store := newSQLDictionaryIndexStore(ctx, client)
	if err := store.Put(mdx.DictionaryInfo{Name: indexKey}, []mdx.IndexEntry{{Keyword: "ability"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveManifest(mdx.IndexManifest{
		DictionaryName: indexKey,
		SourcePath:     path,
		Fingerprint:    fingerprint,
		SchemaVersion:  mdx.DefaultIndexSyncConfig().SchemaVersion,
		BuiltAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(client, "", "", nil, "", 0, "", false, 0, "", "")
	now := time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.externalIndexCacheTTL = time.Minute
	item := &ent.Dictionary{ID: 42, Slug: "demo", MdxPath: path}
	if err := svc.ensureExternalIndex(ctx, item); err != nil {
		t.Fatalf("initial manifest validation failed: %v", err)
	}
	if _, err := client.DictionaryIndexEntry.Delete().
		Where(entindexentry.DictionaryNameEQ(indexKey)).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := svc.ensureExternalIndex(ctx, item); err != nil {
		t.Fatalf("unchanged source should use the validated in-process fingerprint: %v", err)
	}

	now = now.Add(svc.externalIndexCacheTTL)
	if err := svc.ensureExternalIndex(ctx, item); err == nil {
		t.Fatal("expired cache should detect missing derived index rows")
	}

	store = newSQLDictionaryIndexStore(ctx, client)
	if err := store.Put(mdx.DictionaryInfo{Name: indexKey}, []mdx.IndexEntry{{Keyword: "ability"}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ensureExternalIndex(ctx, item); err != nil {
		t.Fatalf("restored derived index rows should pass revalidation: %v", err)
	}

	if err := os.WriteFile(path, []byte("changed source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.ensureExternalIndex(ctx, item); err != nil {
		t.Fatalf("source changes inside the cache TTL should avoid file metadata I/O: %v", err)
	}
	now = now.Add(svc.externalIndexCacheTTL)
	if err := svc.ensureExternalIndex(ctx, item); err == nil {
		t.Fatal("changed source should invalidate the cache after its TTL")
	}
}

func TestExternalIndexKeyIsStableAndPathScoped(t *testing.T) {
	first := filepath.Join(t.TempDir(), "shared.mdx")
	second := filepath.Join(t.TempDir(), "shared.mdx")
	if got, want := externalIndexKey(first), externalIndexKey(filepath.Clean(first)); got != want {
		t.Fatalf("equivalent paths produced different index keys: %q != %q", got, want)
	}
	if externalIndexKey(first) == externalIndexKey(second) {
		t.Fatal("same-basename dictionaries in different paths shared an index key")
	}
	if got := len(externalIndexKey(first)); got != len("owl-")+sha256.Size*2 {
		t.Fatalf("unexpected external index key length: %d", got)
	}
}

func TestDeleteLegacyExternalIndexesKeepsCurrentSQLNamespace(t *testing.T) {
	client, err := ent.Open("sqlite3", "file:legacy_sql_cleanup?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil {
		t.Fatal(err)
	}

	service := NewService(client, "", "", nil, "", 0, "", false, 0, "", "")
	path := filepath.Join(t.TempDir(), "Shared Dictionary.mdx")
	item := &ent.Dictionary{Slug: "custom-slug", MdxPath: path}
	currentKey := externalIndexKey(path)
	store := newSQLDictionaryIndexStore(t.Context(), client)
	for _, key := range []string{legacyExternalIndexKey(path), item.Slug, currentKey} {
		if err := store.Put(mdx.DictionaryInfo{Name: key}, []mdx.IndexEntry{{Keyword: key}}); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveManifest(mdx.IndexManifest{DictionaryName: key, SourcePath: path, Fingerprint: "stat:1"}); err != nil {
			t.Fatal(err)
		}
	}
	collisionKey := "collision-owned-by-another-path"
	if err := store.Put(mdx.DictionaryInfo{Name: collisionKey}, []mdx.IndexEntry{{Keyword: collisionKey}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveManifest(mdx.IndexManifest{DictionaryName: collisionKey, SourcePath: filepath.Join(t.TempDir(), "other.mdx"), Fingerprint: "stat:2"}); err != nil {
		t.Fatal(err)
	}
	item.Slug = collisionKey

	for range 2 {
		if err := service.deleteLegacyExternalIndexes(t.Context(), item, currentKey); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{legacyExternalIndexKey(path)} {
		if _, err := store.GetExact(key, key); !errors.Is(err, mdx.ErrIndexMiss) {
			t.Fatalf("legacy namespace %q still readable: %v", key, err)
		}
		if _, err := store.LoadManifest(key); !errors.Is(err, mdx.ErrIndexMiss) {
			t.Fatalf("legacy manifest %q still readable: %v", key, err)
		}
	}
	if entry, err := store.GetExact(currentKey, currentKey); err != nil || entry.Keyword != currentKey {
		t.Fatalf("current namespace was deleted: entry=%+v err=%v", entry, err)
	}
	if entry, err := store.GetExact(collisionKey, collisionKey); err != nil || entry.Keyword != collisionKey {
		t.Fatalf("colliding namespace owned by another source was deleted: entry=%+v err=%v", entry, err)
	}
}

func TestRediSearchUnavailableGateCachesOnlyDefinitiveCapabilityErrors(t *testing.T) {
	svc := NewService(nil, "", "", redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}), "", 0, "", true, 0, "", "")
	if !svc.rediSearchAvailable() {
		t.Fatal("configured RediSearch should start available")
	}

	svc.markRediSearchUnavailable(errors.New("dial tcp: connection refused"))
	if !svc.rediSearchAvailable() {
		t.Fatal("transient network errors must not permanently disable RediSearch")
	}

	svc.markRediSearchUnavailable(errors.New("ERR unknown command 'FT.SEARCH'"))
	if svc.rediSearchAvailable() {
		t.Fatal("definitive FT command errors should disable RediSearch for this service")
	}
	if store := svc.newRedisSearchStore(t.Context(), "demo"); store != nil {
		t.Fatal("disabled RediSearch should not create stores that issue FT commands")
	}

	aclService := NewService(nil, "", "", redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}), "", 0, "", true, 0, "", "")
	aclService.markRediSearchUnavailable(errors.New("NOPERM this user has no permissions to run the 'FT.SEARCH' command"))
	if aclService.rediSearchAvailable() {
		t.Fatal("FT-specific ACL errors should use the prefix fallback")
	}
}

func TestServiceValkeyCapabilityFallback(t *testing.T) {
	address := os.Getenv("OWL_TEST_VALKEY_ADDR")
	if address == "" {
		t.Skip("set OWL_TEST_VALKEY_ADDR to run the Valkey capability integration test")
	}

	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	hook := newFTCommandCounter()
	client.AddHook(hook)
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatal(err)
	}

	service := NewService(nil, "", "", client, "owl:test:valkey:prefix", 64, "owl:test:valkey:search", true, 0, "", "")
	indexKey := "capability-fallback"
	store := service.newRedisPrefixStore(t.Context())
	t.Cleanup(func() { _ = store.DeleteDictionary(indexKey) })
	if err := store.Put(mdx.DictionaryInfo{Name: indexKey}, []mdx.IndexEntry{{Keyword: "ability"}}); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		hits, err := service.searchExternalStoreHits(t.Context(), indexKey, "abi", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].Entry.Keyword != "ability" || hits[0].Source != "redis-prefix" {
			t.Fatalf("unexpected fallback hits: %#v", hits)
		}
	}
	if got := hook.ftCommands.Load(); got != 1 {
		t.Fatalf("got %d FT commands, want one capability probe", got)
	}
	if service.rediSearchAvailable() {
		t.Fatal("plain Valkey should disable RediSearch after the capability probe")
	}

	path := filepath.Join(t.TempDir(), "Legacy Dictionary.mdx")
	item := &ent.Dictionary{Slug: "legacy-slug", MdxPath: path}
	currentKey := externalIndexKey(path)
	legacyStore := newManagedDictionaryIndexStore(store, nil)
	for _, key := range []string{legacyExternalIndexKey(path), item.Slug, currentKey} {
		if err := legacyStore.Put(mdx.DictionaryInfo{Name: key}, []mdx.IndexEntry{{Keyword: key}}); err != nil {
			t.Fatal(err)
		}
		if err := legacyStore.SaveManifest(mdx.IndexManifest{DictionaryName: key, SourcePath: path, Fingerprint: "stat:1"}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.DeleteDictionary(key) })
	}
	if err := service.deleteLegacyExternalIndexes(t.Context(), item, currentKey); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{legacyExternalIndexKey(path), item.Slug} {
		if _, err := legacyStore.GetExact(key, key); !errors.Is(err, mdx.ErrIndexMiss) {
			t.Fatalf("legacy Valkey namespace %q still readable: %v", key, err)
		}
	}
	if entry, err := store.GetExact(currentKey, currentKey); err != nil || entry.Keyword != currentKey {
		t.Fatalf("current Valkey namespace was deleted: entry=%+v err=%v", entry, err)
	}
}

type ftCommandCounter struct {
	ftCommands atomic.Int32
}

func newFTCommandCounter() *ftCommandCounter { return &ftCommandCounter{} }

func (h *ftCommandCounter) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return next(ctx, network, address)
	}
}

func (h *ftCommandCounter) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if strings.HasPrefix(strings.ToUpper(cmd.Name()), "FT.") {
			h.ftCommands.Add(1)
		}
		return next(ctx, cmd)
	}
}

func (h *ftCommandCounter) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			if strings.HasPrefix(strings.ToUpper(cmd.Name()), "FT.") {
				h.ftCommands.Add(1)
			}
		}
		return next(ctx, cmds)
	}
}

func TestMapDictionariesBoundsConcurrencyAndPreservesOrder(t *testing.T) {
	const dictionaryCount = 32
	dictionaries := make([]*ent.Dictionary, 0, dictionaryCount)
	for id := 1; id <= dictionaryCount; id++ {
		dictionaries = append(dictionaries, &ent.Dictionary{ID: id})
	}

	var active atomic.Int32
	var peak atomic.Int32
	start := make(chan struct{})
	releaseWorkers := sync.OnceFunc(func() { close(start) })
	t.Cleanup(releaseWorkers)
	started := make(chan struct{}, dictionaryQueryConcurrency)
	semaphore := make(chan struct{}, dictionaryQueryConcurrency)
	resultsDone := make(chan []int, 1)
	go func() {
		resultsDone <- mapDictionaries(t.Context(), semaphore, dictionaries, func(item *ent.Dictionary) int {
			current := active.Add(1)
			for {
				previous := peak.Load()
				if current <= previous || peak.CompareAndSwap(previous, current) {
					break
				}
			}
			select {
			case started <- struct{}{}:
			default:
			}
			<-start
			active.Add(-1)
			return item.ID
		})
	}()

	for range dictionaryQueryConcurrency {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not reach the configured concurrency")
		}
	}
	releaseWorkers()

	var results []int
	select {
	case results = <-resultsDone:
	case <-time.After(time.Second):
		t.Fatal("dictionary mapping did not finish")
	}
	if got := int(peak.Load()); got != dictionaryQueryConcurrency {
		t.Fatalf("expected peak concurrency %d, got %d", dictionaryQueryConcurrency, got)
	}
	for index, id := range results {
		if want := index + 1; id != want {
			t.Fatalf("result order changed at %d: got %d, want %d", index, id, want)
		}
	}
}

func TestMapDictionariesSharesConcurrencyAcrossRequests(t *testing.T) {
	const dictionaryCount = 32
	dictionaries := make([]*ent.Dictionary, 0, dictionaryCount)
	for id := 1; id <= dictionaryCount; id++ {
		dictionaries = append(dictionaries, &ent.Dictionary{ID: id})
	}

	semaphore := make(chan struct{}, dictionaryQueryConcurrency)
	var active atomic.Int32
	var peak atomic.Int32
	started := make(chan struct{}, dictionaryQueryConcurrency*2)
	release := make(chan struct{})
	releaseWorkers := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseWorkers)
	work := func(item *ent.Dictionary) int {
		current := active.Add(1)
		for {
			previous := peak.Load()
			if current <= previous || peak.CompareAndSwap(previous, current) {
				break
			}
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		active.Add(-1)
		return item.ID
	}

	var wg sync.WaitGroup
	wg.Go(func() { mapDictionaries(t.Context(), semaphore, dictionaries, work) })
	wg.Go(func() { mapDictionaries(t.Context(), semaphore, dictionaries, work) })
	for range dictionaryQueryConcurrency {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("requests did not fill the shared concurrency budget")
		}
	}
	select {
	case <-started:
		t.Fatal("concurrent requests exceeded the shared concurrency budget")
	case <-time.After(50 * time.Millisecond):
	}
	releaseWorkers()
	wg.Wait()

	if got := int(peak.Load()); got != dictionaryQueryConcurrency {
		t.Fatalf("expected shared peak concurrency %d, got %d", dictionaryQueryConcurrency, got)
	}
}

func TestMapDictionariesCancellationDoesNotLeakSemaphore(t *testing.T) {
	t.Run("waiting_for_token", func(t *testing.T) {
		semaphore := make(chan struct{}, 1)
		semaphore <- struct{}{}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan []int, 1)
		go func() {
			done <- mapDictionaries(ctx, semaphore, []*ent.Dictionary{{ID: 1}}, func(item *ent.Dictionary) int {
				return item.ID
			})
		}()

		cancel()
		select {
		case results := <-done:
			if results[0] != 0 {
				t.Fatalf("canceled work ran unexpectedly: %v", results)
			}
		case <-time.After(time.Second):
			t.Fatal("mapping did not stop while waiting for a token")
		}
		if got := len(semaphore); got != 1 {
			t.Fatalf("canceled waiter changed semaphore occupancy: got %d, want 1", got)
		}

		<-semaphore
		results := mapDictionaries(t.Context(), semaphore, []*ent.Dictionary{{ID: 2}}, func(item *ent.Dictionary) int {
			return item.ID
		})
		if results[0] != 2 {
			t.Fatalf("semaphore was not reusable after cancellation: %v", results)
		}
		if got := len(semaphore); got != 0 {
			t.Fatalf("completed mapping leaked %d semaphore tokens", got)
		}
	})

	t.Run("active_worker", func(t *testing.T) {
		semaphore := make(chan struct{}, 1)
		ctx, cancel := context.WithCancel(t.Context())
		started := make(chan struct{})
		done := make(chan []int, 1)
		go func() {
			done <- mapDictionaries(ctx, semaphore, []*ent.Dictionary{{ID: 1}}, func(item *ent.Dictionary) int {
				close(started)
				<-ctx.Done()
				return item.ID
			})
		}()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker did not start")
		}
		cancel()
		select {
		case results := <-done:
			if results[0] != 1 {
				t.Fatalf("active work returned unexpected result: %v", results)
			}
		case <-time.After(time.Second):
			t.Fatal("active worker did not observe cancellation")
		}
		if got := len(semaphore); got != 0 {
			t.Fatalf("canceled active worker leaked %d semaphore tokens", got)
		}
	})
}

func TestEnsureLoadedWaiterHonorsCancellation(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 0, "", "")
	call := &dictionaryLoadCall{done: make(chan struct{})}
	t.Cleanup(func() { close(call.done) })
	svc.loading[1] = call
	item := &ent.Dictionary{ID: 1}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := svc.ensureLoaded(ctx, item)
		done <- err
	}()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("dictionary load waiter did not honor cancellation")
	}
}

func TestEnsureLoadedActiveWaiterRetriesAfterLeaderCancellation(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, 0, "", "")
	item := &ent.Dictionary{ID: 1, MdxPath: filepath.Join(t.TempDir(), "missing.mdx")}
	call := &dictionaryLoadCall{done: make(chan struct{})}
	svc.loading[item.ID] = call

	ctx := &doneObservingContext{
		Context:      t.Context(),
		doneObserved: make(chan struct{}),
	}
	result := make(chan error, 1)
	go func() {
		_, err := svc.ensureLoaded(ctx, item)
		result <- err
	}()

	select {
	case <-ctx.doneObserved:
	case <-time.After(time.Second):
		t.Fatal("waiter did not begin waiting for the leader")
	}
	svc.mu.Lock()
	call.err = context.Canceled
	delete(svc.loading, item.ID)
	close(call.done)
	svc.mu.Unlock()

	select {
	case err := <-result:
		if err == nil || isContextCancellation(err) {
			t.Fatalf("active waiter inherited leader cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("active waiter did not retry after leader cancellation")
	}

	svc.mu.RLock()
	_, stillLoading := svc.loading[item.ID]
	svc.mu.RUnlock()
	if stillLoading {
		t.Fatal("retry left an inflight dictionary load behind")
	}
}

type doneObservingContext struct {
	context.Context
	doneObserved chan struct{}
	once         sync.Once
}

func (c *doneObservingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.doneObserved) })
	return c.Context.Done()
}

func BenchmarkDictionaryFanout32(b *testing.B) {
	const dictionaryCount = 32
	dictionaries := make([]*ent.Dictionary, 0, dictionaryCount)
	for id := 1; id <= dictionaryCount; id++ {
		dictionaries = append(dictionaries, &ent.Dictionary{ID: id})
	}

	work := func(item *ent.Dictionary) int {
		time.Sleep(time.Millisecond)
		return item.ID
	}

	b.Run("serial", func(b *testing.B) {
		for b.Loop() {
			results := make([]int, 0, len(dictionaries))
			for _, item := range dictionaries {
				results = append(results, work(item))
			}
			if len(results) != dictionaryCount {
				b.Fatalf("got %d results, want %d", len(results), dictionaryCount)
			}
		}
	})

	b.Run("bounded_parallel", func(b *testing.B) {
		semaphore := make(chan struct{}, dictionaryQueryConcurrency)
		for b.Loop() {
			results := mapDictionaries(b.Context(), semaphore, dictionaries, work)
			if len(results) != dictionaryCount {
				b.Fatalf("got %d results, want %d", len(results), dictionaryCount)
			}
		}
	})
}
