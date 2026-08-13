package dictionary

import (
	"fmt"
	"testing"

	"github.com/lib-x/mdx"
)

var benchmarkSearchHTML string

func BenchmarkLoadedDictionaryCacheSequentialGlobalSearch(b *testing.B) {
	for _, dictionaryCount := range []int{8, 16, 32} {
		for _, cacheLimit := range []int{8, 0} {
			name := fmt.Sprintf("dictionaries=%d/cache=%d", dictionaryCount, cacheLimit)
			b.Run(name, func(b *testing.B) {
				svc := NewService(nil, "", "", nil, "", 0, "", false, cacheLimit, "", "")
				loads := 0
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					for id := 1; id <= dictionaryCount; id++ {
						svc.mu.Lock()
						if _, ok := svc.loaded[id]; ok {
							svc.touchLoadedLocked(id)
						} else {
							loads++
							svc.cacheLoadedLocked(id, &LoadedDictionary{})
						}
						svc.mu.Unlock()
					}
				}
				b.ReportMetric(float64(loads)/float64(b.N), "loads/op")
			})
		}
	}
}

func BenchmarkSearchHTMLRewriteFanout(b *testing.B) {
	const definition = `<article class="entry"><h1>ability</h1><p>Definition text with <a href="entry://capability">an internal link</a>.</p><img src="image.png"><a href="sound://voice.spx">play</a></article>`
	for _, dictionaryCount := range []int{8, 16, 32} {
		for _, hitsPerDictionary := range []int{1, 10} {
			name := fmt.Sprintf("dictionaries=%d/hits=%d", dictionaryCount, hitsPerDictionary)
			b.Run(name, func(b *testing.B) {
				b.ReportAllocs()
				var rendered string
				for b.Loop() {
					for dictionaryID := 1; dictionaryID <= dictionaryCount; dictionaryID++ {
						assetBase := fmt.Sprintf("/api/dictionaries/%d/resource", dictionaryID)
						for range hitsPerDictionary {
							rewritten := mdx.RewriteEntryHTML([]byte(definition), assetBase, "/search?q=")
							rendered = string(rewritten)
						}
					}
				}
				benchmarkSearchHTML = rendered
			})
		}
	}
}
