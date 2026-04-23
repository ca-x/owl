import { useEffect, useMemo, useRef, useState } from 'react'

import { DictionaryEntryHtml } from '../components/DictionaryEntryHtml'
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

function updateSearchURL(query: string, dictionaryId?: number, mode: 'replace' | 'push' = 'replace') {
  const params = new URLSearchParams()
  const normalizedQuery = query.trim()
  if (normalizedQuery) {
    params.set('q', normalizedQuery)
  }
  if (dictionaryId !== undefined) {
    params.set('dict', String(dictionaryId))
  }
  const next = params.toString()
  const target = next ? `/search?${next}` : '/search'
  if (`${window.location.pathname}${window.location.search}` === target) {
    return
  }
  if (mode === 'push') {
    window.history.pushState(null, '', target)
    return
  }
  window.history.replaceState(null, '', target)
}

function readSearchURLState() {
  const params = new URLSearchParams(window.location.search)
  const query = params.get('q')?.trim() ?? ''
  const rawDict = params.get('dict')?.trim() ?? ''
  const dictionaryId = rawDict !== '' && /^\d+$/.test(rawDict) ? Number(rawDict) : undefined
  return { query, dictionaryId }
}

export function SearchPage({ dictionaries, loading, searching, results, error, isGuest, token, recentSearches, onSearch }: SearchPageProps) {
  const { t } = useI18n()
  const initialURLState = useMemo(() => readSearchURLState(), [])
  const hydratedFromURL = useRef(false)
  const [query, setQuery] = useState(initialURLState.query)
  const [dictionaryId, setDictionaryId] = useState<number | undefined>(initialURLState.dictionaryId)
  const [filtersExpanded, setFiltersExpanded] = useState(false)
  const [suggestions, setSuggestions] = useState<SearchSuggestion[]>([])
  const [activeSuggestion, setActiveSuggestion] = useState<number>(-1)
  const [suggestionsDismissed, setSuggestionsDismissed] = useState(initialURLState.query.length > 0)

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
    return { topResult: bestMatch ?? null, groupedResults: rest }
  }, [results])

  async function runQuickSearch(nextQuery: string, nextDictionaryId?: number) {
    const normalizedQuery = nextQuery.trim()
    if (!normalizedQuery) return
    const resolvedDictionaryId = nextDictionaryId ?? dictionaryId
    setQuery(normalizedQuery)
    setSuggestionsDismissed(true)
    setActiveSuggestion(-1)
    setDictionaryId(resolvedDictionaryId)
    updateSearchURL(normalizedQuery, resolvedDictionaryId, 'push')
    await onSearch(normalizedQuery, resolvedDictionaryId)
  }

  useEffect(() => {
    if (hydratedFromURL.current) return
    hydratedFromURL.current = true

    const applyURLState = (state: ReturnType<typeof readSearchURLState>) => {
      setQuery(state.query)
      setDictionaryId(state.dictionaryId)
      setSuggestionsDismissed(state.query.length > 0)
      setActiveSuggestion(-1)
      void onSearch(state.query, state.dictionaryId)
    }

    applyURLState(initialURLState)

    const handlePopState = () => {
      applyURLState(readSearchURLState())
    }

    window.addEventListener('popstate', handlePopState)
    return () => {
      window.removeEventListener('popstate', handlePopState)
    }
  }, [initialURLState, onSearch])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const keyword = activeSuggestion >= 0 ? visibleSuggestions[activeSuggestion]?.word ?? query : query
    await runQuickSearch(keyword, dictionaryId)
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
        setActiveSuggestion(nextSuggestions.length > 0 ? 0 : -1)
      } catch {
        if (!active) return
        setSuggestions([])
        setActiveSuggestion(-1)
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
    if ((event.key === 'Enter' || event.key === 'Tab') && activeSuggestion >= 0) {
      event.preventDefault()
      const suggestion = visibleSuggestions[activeSuggestion]
      if (!suggestion) return
      await runQuickSearch(suggestion.word)
    }
  }

  return (
    <section className="page-section">
      <section className="lookup-hero card">
        <div className="lookup-hero-copy">
          <div className="eyebrow">{t.dictionaryLookup}</div>
          <h2>{t.lookupTitle}</h2>
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
            {visibleSuggestions.length > 0 ? (
              <div className="autocomplete-popover">
                <div className="autocomplete-head">
                  <span className="recent-label">{t.suggestions}</span>
                  <span className="muted">{t.pressEnterHint}</span>
                </div>
                <div className="autocomplete-list">
                  {visibleSuggestions.map((item, index) => (
                    <div
                      key={`${item.word}-${index}`}
                      className={activeSuggestion === index ? 'autocomplete-group active' : 'autocomplete-group'}
                      onMouseEnter={() => setActiveSuggestion(index)}
                    >
                      <button
                        className="autocomplete-item autocomplete-word-button"
                        type="button"
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => void runQuickSearch(item.word)}
                      >
                        <div className="suggestion-main">
                          <strong>{item.word}</strong>
                          <span className="suggestion-subline">{item.sources.map((source) => source.dictionary_name).join(' · ')}</span>
                        </div>
                      </button>
                      <div className="autocomplete-source-list">
                        {item.sources.map((source) => (
                          <button
                            key={`${item.word}-${source.dictionary_id}`}
                            className="autocomplete-source-chip"
                            type="button"
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={() => void runQuickSearch(item.word, source.dictionary_id)}
                          >
                            <span>{source.dictionary_name}</span>
                            <span className={source.visibility === 'public' ? 'status-pill info-pill' : 'status-pill muted-pill'}>
                              {source.visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}
                            </span>
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </label>

          <button className="primary-button hero-search-button" type="submit" disabled={loading || searching}>
            {searching ? t.searching : t.lookupAction}
          </button>
        </form>

        <div className="scope-banner">
          <span className="scope-label">{t.currentScope}</span>
          <strong>{scopeLabel}</strong>
          <button className="text-chip" type="button" onClick={() => setFiltersExpanded((current) => !current)}>
            {filtersExpanded ? t.hideFilters : t.showFilters}
          </button>
        </div>

        {filtersExpanded ? (
          <div className="dictionary-filter-strip filter-strip-panel">
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
            <section className="card hero-result reading-surface">
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
              <DictionaryEntryHtml html={topResult.html} className="definition-html main-definition" onLookup={runQuickSearch} />
            </section>
          ) : null}

          {groupedResults.length > 0 ? (
            <section className="result-visibility-group">
              <div className="result-visibility-header">
                <div className="eyebrow">{t.moreMatches}</div>
                <h3>{groupedResults.length}</h3>
              </div>
              <div className="result-list simple-result-list">
                {groupedResults.map((item, index) => (
                  <article key={`${item.dictionary_id}-${item.word}-${index}`} className="card simple-result-card">
                    <div className="result-meta compact-result-meta">
                      <div className="result-title-stack">
                        <strong>{item.word}</strong>
                        <span className="result-source-inline">
                          {item.dictionary_name} · {item.visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}
                        </span>
                      </div>
                      <button className="text-chip" type="button" onClick={() => void runQuickSearch(item.word, item.dictionary_id)}>
                        {t.searchOnlyThisDictionary}
                      </button>
                    </div>
                    <DictionaryEntryHtml html={item.html} className="definition-html compare-definition" onLookup={runQuickSearch} />
                  </article>
                ))}
              </div>
            </section>
          ) : null}
        </div>
      )}
    </section>
  )
}
