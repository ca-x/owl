import { BookOpen, Check, CopySimple, MagnifyingGlass, X, GlobeHemisphereEast, LockKey } from '@phosphor-icons/react'
import { useEffect, useEffectEvent, useMemo, useRef, useState } from 'react'

import { DictionaryEntryHtml } from '../components/DictionaryEntryHtml'
import { useI18n } from '../i18n'
import { api } from '../services/api'
import type { DictionarySummary, SearchResult, SearchSuggestion } from '../types'

interface SearchPageProps {
  dictionaries: DictionarySummary[]
  loading: boolean
  error: string
  isGuest: boolean
  token: string | null
  recentSearches: string[]
  onSearched: (query: string) => void
}

type LookupState =
  | { status: 'idle' | 'loading' }
  | { status: 'done'; word: string; results: SearchResult[] }
  | { status: 'error'; message: string }

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

export function SearchPage({ dictionaries, loading, error, isGuest, token, recentSearches, onSearched }: SearchPageProps) {
  const { t } = useI18n()
  const initialURLState = useMemo(() => readSearchURLState(), [])
  const request = useRef<AbortController | null>(null)
  const input = useRef<HTMLInputElement>(null)
  const [lookup, setLookup] = useState<LookupState>({ status: initialURLState.query ? 'loading' : 'idle' })
  const [activeSource, setActiveSource] = useState(-1)
  const [copyError, setCopyError] = useState('')
  const pageTopRef = useRef<HTMLElement | null>(null)
  const resultsRef = useRef<HTMLDivElement | null>(null)
  const [query, setQuery] = useState(initialURLState.query)
  const [dictionaryId, setDictionaryId] = useState<number | undefined>(initialURLState.dictionaryId)
  const [filtersExpanded, setFiltersExpanded] = useState(false)
  const [suggestions, setSuggestions] = useState<SearchSuggestion[]>([])
  const [activeSuggestion, setActiveSuggestion] = useState<number>(-1)
  const [suggestionsDismissed, setSuggestionsDismissed] = useState(initialURLState.query.length > 0)
  const [copiedResultKey, setCopiedResultKey] = useState('')

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

  const [topResult, ...groupedResults] = lookup.status === 'done' ? lookup.results : []
  const searching = lookup.status === 'loading'
  const message = lookup.status === 'error' ? lookup.message : error

  function isMobileViewport() {
    return window.matchMedia('(max-width: 720px)').matches
  }

  function scrollAfterSearch() {
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const target = isMobileViewport() ? resultsRef.current : pageTopRef.current
    const targetTop = target?.getBoundingClientRect().top ?? 0
    const offset = isMobileViewport() ? 10 : 12
    const top = Math.max(0, window.scrollY + targetTop - offset)
    window.scrollTo({ top, behavior: prefersReducedMotion ? 'auto' : 'smooth' })
  }

  function handleDictionarySelection(value: string) {
    const nextScope = value ? Number(value) : undefined
    if (lookup.status === 'done' && query.trim() === lookup.word) {
      updateSearchURL(lookup.word, nextScope, 'push')
      void search(lookup.word, nextScope, false)
      return
    }
    setDictionaryId(nextScope)
    setSuggestionsDismissed(true)
    setSuggestions([])
    setActiveSuggestion(-1)
  }

  function definitionText(result: SearchResult) {
    const container = document.createElement('div')
    container.innerHTML = result.html
    return `${result.word} · ${result.dictionary_name}\n\n${container.textContent?.replace(/\s+/g, ' ').trim() ?? ''}`.trim()
  }

  async function copyResultDefinition(result: SearchResult, key: string) {
    setCopyError('')
    try {
      await navigator.clipboard.writeText(definitionText(result))
      setCopiedResultKey(key)
      window.setTimeout(() => {
        setCopiedResultKey((current) => (current === key ? '' : current))
      }, 1600)
    } catch {
      setCopyError(t.copyFailed)
      setCopiedResultKey('')
    }
  }

  async function search(word: string, scope: number | undefined, scroll: boolean) {
    request.current?.abort()
    setCopyError('')
    setCopiedResultKey('')
    setSuggestionsDismissed(true)
    setActiveSuggestion(-1)
    setActiveSource(-1)
    setQuery(word)
    setDictionaryId(scope)
    if (!word) {
      setLookup({ status: 'idle' })
      return
    }
    setLookup({ status: 'loading' })
    await fetchWord(word, scope, scroll)
  }

  async function fetchWord(word: string, scope: number | undefined, scroll: boolean) {
    const controller = new AbortController()
    request.current = controller
    try {
      const results = token
        ? await api.search(token, word, scope, controller.signal)
        : await api.publicSearch(word, scope, controller.signal)
      if (controller.signal.aborted) return
      setLookup({ status: 'done', word, results })
      onSearched(word)
      if (scroll) window.requestAnimationFrame(() => { if (!controller.signal.aborted) scrollAfterSearch() })
    } catch (err) {
      if (!controller.signal.aborted) setLookup({ status: 'error', message: err instanceof Error ? err.message : t.genericError })
    }
  }

  async function runQuickSearch(nextQuery: string, scope = dictionaryId) {
    const word = nextQuery.trim()
    if (!word) return
    updateSearchURL(word, scope, 'push')
    await search(word, scope, true)
  }

  const fetchInitialWord = useEffectEvent(() => {
    const { query, dictionaryId } = readSearchURLState()
    if (query) void fetchWord(query, dictionaryId, false)
  })

  const restoreURL = useEffectEvent(() => {
    const state = readSearchURLState()
    void search(state.query, state.dictionaryId, false)
  })

  useEffect(() => {
    fetchInitialWord()
    const onPopState = () => restoreURL()
    const onShortcut = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.isComposing || event.altKey || event.metaKey || event.ctrlKey) return
      if (document.querySelector('dialog[open]') || (event.target instanceof HTMLElement &&
        (event.target.isContentEditable || event.target.closest('input, textarea, select')))) return
      event.preventDefault()
      input.current?.focus()
      input.current?.select()
    }
    window.addEventListener('popstate', onPopState)
    window.addEventListener('keydown', onShortcut)
    return () => {
      request.current?.abort()
      window.removeEventListener('popstate', onPopState)
      window.removeEventListener('keydown', onShortcut)
    }
  }, [token])

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const item = visibleSuggestions[activeSuggestion]
    void runQuickSearch(item?.word ?? query, item?.sources[activeSource]?.dictionary_id)
  }

  useEffect(() => {
    const word = query.trim()
    if (word.length < 2 || suggestionsDismissed) return
    const controller = new AbortController()
    const timer = window.setTimeout(async () => {
      try {
        const items = token
          ? await api.suggest(token, word, dictionaryId, controller.signal)
          : await api.publicSuggest(word, dictionaryId, controller.signal)
        if (!controller.signal.aborted) setSuggestions(items)
      } catch {
        if (!controller.signal.aborted) setSuggestions([])
      }
    }, 180)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [dictionaryId, query, suggestionsDismissed, token])

  function changeQuery(value: string) {
    setQuery(value)
    setSuggestions([])
    setSuggestionsDismissed(false)
    setActiveSuggestion(-1)
    setActiveSource(-1)
  }

  const activeCell = visibleSuggestions[activeSuggestion] ? `suggestion-${activeSuggestion}-${activeSource}` : undefined
  useEffect(() => {
    if (activeCell) document.getElementById(activeCell)?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  }, [activeCell])

  function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return
    if (event.key === 'Escape') {
      setSuggestionsDismissed(true)
      setActiveSuggestion(-1)
      return
    }
    if (!visibleSuggestions.length) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const count = visibleSuggestions.length
      setActiveSuggestion(event.key === 'ArrowDown' ? (activeSuggestion + 1) % count : (activeSuggestion <= 0 ? count - 1 : activeSuggestion - 1))
      setActiveSource(-1)
    } else if (activeSuggestion >= 0 && (event.key === 'ArrowLeft' || event.key === 'ArrowRight')) {
      event.preventDefault()
      setActiveSource(Math.max(-1, Math.min(visibleSuggestions[activeSuggestion].sources.length - 1, activeSource + (event.key === 'ArrowRight' ? 1 : -1))))
    }
  }

  return (
    <section className="page-section" ref={pageTopRef}>
      <section className="lookup-hero card">
        <div className="lookup-hero-copy">
          <h1>{t.lookupTitle}</h1>
        </div>

        <form className="lookup-form" role="search" onSubmit={handleSubmit}>
          <div className="lookup-input">
            <MagnifyingGlass className="lookup-glass" size={21} aria-hidden="true" />
            <input
              autoFocus={!initialURLState.query && window.matchMedia('(hover: hover) and (pointer: fine)').matches}
              ref={input}
              id="lookup-query"
              aria-label={t.searchQueryLabel}
              role="combobox"
              aria-autocomplete="list"
              aria-haspopup="grid"
              aria-expanded={visibleSuggestions.length > 0}
              aria-controls={visibleSuggestions.length ? 'lookup-suggestions' : undefined}
              aria-activedescendant={activeCell}
              autoComplete="off"
              enterKeyHint="search"
              spellCheck={false}
              value={query}
              onChange={(event) => changeQuery(event.target.value)}
              onKeyDown={handleKeyDown}
              onFocus={() => setSuggestionsDismissed(false)}
              onBlur={() => { setSuggestionsDismissed(true); setActiveSuggestion(-1) }}
              placeholder={t.searchPlaceholder}
              required
            />
            {query ? <button className="clear-query" type="button" aria-label={t.clearQuery} onClick={() => {
              updateSearchURL('', dictionaryId, 'push')
              void search('', dictionaryId, false)
              input.current?.focus()
            }}><X size={16} /></button> : <kbd className="lookup-shortcut" aria-hidden="true">/</kbd>}
            {visibleSuggestions.length > 0 ? (
              <div className="autocomplete-popover">
                <div className="autocomplete-head">
                  <span className="recent-label">{t.suggestions}</span>
                  <span className="muted">{t.pressEnterHint}</span>
                </div>
                <div className="autocomplete-list" id="lookup-suggestions" role="grid" aria-label={t.suggestions}>
                  {visibleSuggestions.map((item, index) => (
                    <div
                      key={`${item.word}-${index}`}
                      className={activeSuggestion === index ? 'autocomplete-group active' : 'autocomplete-group'}
                      role="row"
                    >
                      <button
                        className="autocomplete-item autocomplete-word-button"
                        role="gridcell"
                        tabIndex={-1}
                        id={`suggestion-${index}--1`}
                        aria-selected={activeSuggestion === index && activeSource === -1}
                        type="button"
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => void runQuickSearch(item.word)}
                      >
                        <div className="suggestion-main">
                          <strong>{item.word}</strong>
                        </div>
                      </button>
                      <div className="autocomplete-source-list" role="presentation">
                        {item.sources.map((source, sourceIndex) => (
                          <button
                            key={`${item.word}-${source.dictionary_id}`}
                            className="autocomplete-source-chip"
                            role="gridcell"
                            tabIndex={-1}
                            id={`suggestion-${index}-${sourceIndex}`}
                            aria-selected={activeSuggestion === index && activeSource === sourceIndex}
                            type="button"
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={() => void runQuickSearch(item.word, source.dictionary_id)}
                          >
                            <span className="autocomplete-source-name">{source.dictionary_name}</span>
                            <span className={source.visibility === 'public' ? 'source-visibility-icon public' : 'source-visibility-icon private'} aria-hidden="true">
                              {source.visibility === 'public' ? <GlobeHemisphereEast size={12} weight="fill" /> : <LockKey size={12} weight="fill" />}
                            </span>
                            <span className="sr-only">{source.visibility === 'public' ? t.resultVisibilityPublic : t.resultVisibilityPrivate}</span>
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </div>

          <button className="primary-button hero-search-button" type="submit" disabled={loading}>
            {searching ? t.searching : t.lookupAction}
          </button>
        </form>

        <div className="scope-banner">
          <span className="scope-label">{t.currentScope}</span>
          <strong>{scopeLabel}</strong>
          <label className="mobile-dictionary-select">
            <span>{t.mobileDictionaryFilter}</span>
            <select value={dictionaryId ?? ''} onChange={(event) => handleDictionarySelection(event.target.value)}>
              <option value="">{t.allDictionaries}</option>
              {enabledDictionaries.map((item) => (
                <option key={item.id} value={item.id}>{item.title || item.name}</option>
              ))}
            </select>
          </label>
          <button className="text-chip desktop-filter-toggle" aria-expanded={filtersExpanded} type="button" onClick={() => setFiltersExpanded((current) => !current)}>
            {filtersExpanded ? t.hideFilters : t.showFilters}
          </button>
        </div>

        {filtersExpanded ? (
          <div className="dictionary-filter-strip filter-strip-panel">
            <button
              className={dictionaryId === undefined ? 'filter-chip active' : 'filter-chip'}
              type="button"
              aria-pressed={dictionaryId === undefined}
              onClick={() => handleDictionarySelection('')}
            >
              {t.allDictionaries}
            </button>
            {enabledDictionaries.map((item) => (
              <button
                key={item.id}
                className={dictionaryId === item.id ? 'filter-chip active' : 'filter-chip'}
                type="button"
                aria-pressed={dictionaryId === item.id}
                onClick={() => handleDictionarySelection(String(item.id))}
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

      {message && <div className="error-banner" role="alert">{message}</div>}
      {copyError && <div className="error-banner" role="alert">{copyError}</div>}
      <div ref={resultsRef} className="mobile-results-anchor" aria-hidden="true" />
      {searching ? (
        <div className="lookup-status" role="status">{t.searching}</div>
      ) : lookup.status === 'done' && !topResult ? (
        <div className="empty-state card lookup-empty" role="status">
          <MagnifyingGlass size={28} aria-hidden="true" />
          <div><h3>{t.noResultTitle} · {lookup.word}</h3><p className="muted">{t.noResultDescription}</p></div>
        </div>
      ) : lookup.status === 'idle' && !error ? (
        <div className="empty-state card lookup-empty">
          <BookOpen size={28} aria-hidden="true" />
          <div><h3>{enabledDictionaries.length ? t.readyTitle : t.noDictionariesYet}</h3>
          <p className="muted">{enabledDictionaries.length ? t.readyDescription : isGuest ? t.noPublicDictionaries : t.uploadFirstDictionary}</p></div>
        </div>
      ) : lookup.status === 'done' ? (
        <div className="results-grid">
          <div className="result-output-summary" aria-live="polite">
            <span className="scope-label">{t.resultCount(lookup.results.length)}</span>
            <strong>{lookup.word}</strong>
          </div>
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
                  <button className="text-chip definition-copy-button" type="button" onClick={() => void copyResultDefinition(topResult, `top-${topResult.dictionary_id}-${topResult.word}`)}>
                    {copiedResultKey === `top-${topResult.dictionary_id}-${topResult.word}` ? <Check size={14} weight="bold" aria-hidden="true" /> : <CopySimple size={14} weight="bold" aria-hidden="true" />}
                    <span>{copiedResultKey === `top-${topResult.dictionary_id}-${topResult.word}` ? t.definitionCopied : t.copyDefinition}</span>
                  </button>
                </div>
              </div>
              <DictionaryEntryHtml html={topResult.html} className="definition-html main-definition" onLookup={runQuickSearch} />
            </section>
          ) : null}

          {groupedResults.length > 0 ? (
            <section className="result-visibility-group">
              <div className="result-visibility-header">
                <h3>{t.moreMatches} <span className="muted">({groupedResults.length})</span></h3>
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
                      <div className="result-card-actions">
                        <button className="text-chip" type="button" onClick={() => void runQuickSearch(item.word, item.dictionary_id)}>
                          {t.searchOnlyThisDictionary}
                        </button>
                        <button className="text-chip definition-copy-button" type="button" onClick={() => void copyResultDefinition(item, `${item.dictionary_id}-${item.word}-${index}`)}>
                          {copiedResultKey === `${item.dictionary_id}-${item.word}-${index}` ? <Check size={14} weight="bold" aria-hidden="true" /> : <CopySimple size={14} weight="bold" aria-hidden="true" />}
                          <span>{copiedResultKey === `${item.dictionary_id}-${item.word}-${index}` ? t.definitionCopied : t.copyDefinition}</span>
                        </button>
                      </div>
                    </div>
                    <DictionaryEntryHtml html={item.html} className="definition-html compare-definition" onLookup={runQuickSearch} />
                  </article>
                ))}
              </div>
            </section>
          ) : null}
        </div>
      ) : null}
    </section>
  )
}
