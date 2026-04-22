import { useEffect, useMemo, useState } from 'react'

import { useI18n } from '../i18n'
import { api } from '../services/api'
import type { DictionarySummary, SearchResult, SearchSuggestion } from '../types'

interface SearchPageProps {
  dictionaries: DictionarySummary[]
  loading: boolean
  searching: boolean
  results: SearchResult[]
  error: string
  isGuest: boolean
  token: string | null
  recentSearches: string[]
  onSearch: (query: string, dictionaryId?: number) => Promise<void>
}

export function SearchPage({ dictionaries, loading, searching, results, error, isGuest, token, recentSearches, onSearch }: SearchPageProps) {
  const { t } = useI18n()
  const [query, setQuery] = useState('')
  const [dictionaryId, setDictionaryId] = useState<number | undefined>()
  const [suggestions, setSuggestions] = useState<SearchSuggestion[]>([])
  const [activeSuggestion, setActiveSuggestion] = useState<number>(-1)
  const [suggestionsDismissed, setSuggestionsDismissed] = useState(false)

  const enabledDictionaries = useMemo(() => dictionaries.filter((item) => item.enabled), [dictionaries])
  const visibleSuggestions = useMemo(
    () => (query.trim().length >= 2 && !suggestionsDismissed ? suggestions : []),
    [query, suggestions, suggestionsDismissed],
  )

  const scopeLabel = useMemo(() => {
    if (dictionaryId !== undefined) {
      const selected = enabledDictionaries.find((item) => item.id === dictionaryId)
      return t.scopeSpecificDictionary(selected?.title || selected?.name || `#${dictionaryId}`)
    }
    return isGuest ? t.scopeAllPublic : t.scopeAllAccessible
  }, [dictionaryId, enabledDictionaries, isGuest, t])

  const { topResult, groupedResults } = useMemo(() => {
    const [bestMatch, ...rest] = results
    const grouped = rest.reduce<Record<string, Record<string, SearchResult[]>>>((accumulator, item) => {
      const visibilityGroup = item.visibility
      if (!accumulator[visibilityGroup]) {
        accumulator[visibilityGroup] = {}
      }
      if (!accumulator[visibilityGroup][item.dictionary_name]) {
        accumulator[visibilityGroup][item.dictionary_name] = []
      }
      accumulator[visibilityGroup][item.dictionary_name].push(item)
      return accumulator
    }, { public: {}, private: {} })
    return { topResult: bestMatch ?? null, groupedResults: grouped }
  }, [results])

  const sameHeadwordGroups = useMemo(() => {
    if (!topResult) return []
    const canonical = topResult.word.trim().toLowerCase()
    if (!canonical) return []
    return results.filter((item) => item.word.trim().toLowerCase() === canonical)
  }, [results, topResult])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const keyword = activeSuggestion >= 0 ? suggestions[activeSuggestion]?.word ?? query : query
    await onSearch(keyword, dictionaryId)
  }

  async function runQuickSearch(nextQuery: string, nextDictionaryId?: number) {
    setQuery(nextQuery)
    setSuggestionsDismissed(true)
    if (nextDictionaryId !== undefined) {
      setDictionaryId(nextDictionaryId)
    }
    await onSearch(nextQuery, nextDictionaryId ?? dictionaryId)
  }

  useEffect(() => {
    const normalizedQuery = query.trim()
    if (normalizedQuery.length < 2) {
      return
    }
    let active = true
    const timer = window.setTimeout(async () => {
      try {
        const nextSuggestions = isGuest
          ? await api.publicSuggest(normalizedQuery, dictionaryId)
          : token
            ? await api.suggest(token, normalizedQuery, dictionaryId)
            : []
        if (!active) return
        setSuggestions(nextSuggestions)
        setActiveSuggestion(-1)
        setSuggestionsDismissed(false)
      } catch {
        if (!active) return
        setSuggestions([])
      }
    }, 180)

    return () => {
      active = false
      window.clearTimeout(timer)
    }
  }, [dictionaryId, isGuest, query, token])

  async function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape') {
      setSuggestionsDismissed(true)
      setActiveSuggestion(-1)
      return
    }
    if (visibleSuggestions.length === 0) return
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setActiveSuggestion((current) => (current + 1) % visibleSuggestions.length)
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      setActiveSuggestion((current) => (current <= 0 ? visibleSuggestions.length - 1 : current - 1))
      return
    }
    if (event.key === 'Enter' && activeSuggestion >= 0) {
      event.preventDefault()
      const suggestion = visibleSuggestions[activeSuggestion]
      if (!suggestion) return
      await runQuickSearch(suggestion.word, suggestion.dictionary_id)
      return
    }
    if (event.key === 'Tab' && activeSuggestion >= 0) {
      const suggestion = visibleSuggestions[activeSuggestion]
      if (!suggestion) return
      event.preventDefault()
      await runQuickSearch(suggestion.word, suggestion.dictionary_id)
    }
  }

  return (
    <section className="page-section">
      <section className="lookup-hero card">
        <div className="lookup-hero-copy">
          <div className="eyebrow">{t.dictionaryLookup}</div>
          <h2>{t.lookupTitle}</h2>
          <p className="muted">{t.lookupDescription}</p>
        </div>

        <form className="lookup-form" onSubmit={handleSubmit}>
          <label className="lookup-input">
            <span className="sr-only">Search query</span>
            <input
              autoFocus
              value={query}
              onChange={(event) => {
                setQuery(event.target.value)
                setSuggestionsDismissed(false)
              }}
              onKeyDown={(event) => void handleKeyDown(event)}
              onFocus={() => setSuggestionsDismissed(false)}
              onBlur={() => {
                window.setTimeout(() => {
                  setSuggestionsDismissed(true)
                  setActiveSuggestion(-1)
                }, 120)
              }}
              placeholder={t.searchPlaceholder}
              required
            />
          </label>

          <button className="primary-button hero-search-button" type="submit" disabled={loading || searching}>
            {searching ? t.searching : t.lookupAction}
          </button>
        </form>

        <div className="dictionary-filter-strip">
          <button
            className={dictionaryId === undefined ? 'filter-chip active' : 'filter-chip'}
            type="button"
            onClick={() => setDictionaryId(undefined)}
          >
            {t.allDictionaries}
          </button>
          {enabledDictionaries.map((item) => (
            <button
              key={item.id}
              className={dictionaryId === item.id ? 'filter-chip active' : 'filter-chip'}
              type="button"
              onClick={() => setDictionaryId(item.id)}
            >
              {item.title || item.name}
            </button>
          ))}
        </div>

        <div className="scope-banner">
          <span className="scope-label">{t.currentScope}</span>
          <strong>{scopeLabel}</strong>
        </div>

        {visibleSuggestions.length > 0 ? (
          <div className="suggestions-card">
            <div className="suggestions-head">
              <span className="recent-label">{t.suggestions}</span>
              <span className="muted">{t.pressEnterHint}</span>
            </div>
            <div className="suggestion-list">
              {visibleSuggestions.map((item, index) => (
                <button
                  key={`${item.dictionary_id}-${item.word}-${index}`}
                  className={activeSuggestion === index ? 'suggestion-item active' : 'suggestion-item'}
                  type="button"
                  onMouseEnter={() => setActiveSuggestion(index)}
                  onClick={() => void runQuickSearch(item.word, item.dictionary_id)}
                >
                  <div className="suggestion-main">
                    <strong>{item.word}</strong>
                    <span className="muted">{item.dictionary_name}</span>
                  </div>
                  <span className={item.visibility === 'public' ? 'status-pill info-pill' : 'status-pill muted-pill'}>
                    {item.visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}
                  </span>
                </button>
              ))}
            </div>
          </div>
        ) : null}

        {recentSearches.length > 0 ? (
          <div className="recent-searches">
            <span className="recent-label">{t.recent}</span>
            <div className="recent-search-list">
              {recentSearches.map((item) => (
                <button key={item} className="recent-chip" type="button" onClick={() => void runQuickSearch(item)}>
                  {item}
                </button>
              ))}
            </div>
          </div>
        ) : null}
      </section>

      {error ? <div className="error-banner">{error}</div> : null}

      {results.length === 0 && query ? (
        <div className="empty-state card">
          <h3>{t.noResultTitle}</h3>
          <p className="muted">{t.noResultDescription}</p>
        </div>
      ) : results.length === 0 ? (
        <div className="empty-state card">
          <h3>{t.readyTitle}</h3>
          <p className="muted">{t.readyDescription}</p>
        </div>
      ) : (
        <div className="results-grid">
          {topResult ? (
            <section className="card hero-result">
              <div className="hero-result-head">
                <div>
                  <div className="eyebrow">{t.bestMatch}</div>
                  <h3>{topResult.word}</h3>
                </div>
                <div className="result-badges">
                  <span className="dictionary-badge">{topResult.dictionary_name}</span>
                  <span className={topResult.visibility === 'public' ? 'status-pill info-pill' : 'status-pill muted-pill'}>
                    {topResult.visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}
                  </span>
                </div>
              </div>
              <div className="definition-html main-definition" dangerouslySetInnerHTML={{ __html: topResult.html }} />
            </section>
          ) : null}

          {sameHeadwordGroups.length > 1 ? (
            <section className="result-visibility-group">
              <div className="result-visibility-header">
                <div className="eyebrow">{t.compareSameHeadword}</div>
                <h3>{topResult?.word}</h3>
              </div>
              <div className="results-compare-grid">
                {sameHeadwordGroups.map((item, index) => (
                  <section key={`${item.dictionary_id}-${index}`} className="card result-group compare-card">
                    <div className="result-group-header">
                      <div>
                        <h3>{item.dictionary_name}</h3>
                        <p className="muted">{item.visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}</p>
                      </div>
                    </div>
                    <div className="definition-html" dangerouslySetInnerHTML={{ __html: item.html }} />
                  </section>
                ))}
              </div>
            </section>
          ) : null}

          {(['public', 'private'] as const).map((visibility) => {
            const groups = groupedResults[visibility]
            const entries = Object.entries(groups)
            if (entries.length === 0) return null
            return (
              <section key={visibility} className="result-visibility-group">
                <div className="result-visibility-header">
                  <div className="eyebrow">{t.compareAcrossDictionaries}</div>
                  <h3>{visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}</h3>
                </div>
                <div className="results-compare-grid">
                  {entries.map(([dictionaryName, items]) => (
                    <section key={dictionaryName} className="card result-group compare-card">
                      <div className="result-group-header">
                        <div>
                          <h3>{dictionaryName}</h3>
                          <p className="muted">{t.moreEntriesFromDictionary}</p>
                        </div>
                        <span>{items.length}</span>
                      </div>
                      <div className="result-list">
                        {items.map((item, index) => (
                          <article key={`${item.dictionary_id}-${item.word}-${index}`} className="result-card compact-result-card">
                            <div className="result-meta compact-result-meta">
                              <div className="result-title-stack">
                                <strong>{item.word}</strong>
                                <span className="result-visibility-inline">
                                  {item.visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}
                                </span>
                              </div>
                              <button className="text-chip" type="button" onClick={() => void runQuickSearch(item.word, item.dictionary_id)}>
                                {t.searchOnlyThisDictionary}
                              </button>
                            </div>
                            <div className="definition-html" dangerouslySetInnerHTML={{ __html: item.html }} />
                          </article>
                        ))}
                      </div>
                    </section>
                  ))}
                </div>
              </section>
            )
          })}
        </div>
      )}
    </section>
  )
}
