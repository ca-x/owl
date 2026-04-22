import { useMemo, useState } from 'react'

import type { DictionarySummary, SearchResult } from '../types'

interface SearchPageProps {
  dictionaries: DictionarySummary[]
  loading: boolean
  searching: boolean
  results: SearchResult[]
  error: string
  onSearch: (query: string, dictionaryId?: number) => Promise<void>
}

export function SearchPage({ dictionaries, loading, searching, results, error, onSearch }: SearchPageProps) {
  const [query, setQuery] = useState('')
  const [dictionaryId, setDictionaryId] = useState<number | undefined>()

  const grouped = useMemo(() => {
    return results.reduce<Record<string, SearchResult[]>>((accumulator, item) => {
      if (!accumulator[item.dictionary_name]) {
        accumulator[item.dictionary_name] = []
      }
      accumulator[item.dictionary_name].push(item)
      return accumulator
    }, {})
  }, [results])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await onSearch(query, dictionaryId)
  }

  return (
    <section className="page-section">
      <div className="page-header">
        <div>
          <div className="eyebrow">Word Lookup</div>
          <h2>Search across your enabled dictionaries</h2>
          <p className="muted">Results are grouped by dictionary and render entry HTML directly.</p>
        </div>
      </div>

      <form className="search-bar card" onSubmit={handleSubmit}>
        <label className="field grow">
          <span>Query</span>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="hello, 能力, ability…" required />
        </label>

        <label className="field compact-field">
          <span>Dictionary</span>
          <select
            value={dictionaryId ?? ''}
            onChange={(event) => setDictionaryId(event.target.value ? Number(event.target.value) : undefined)}
          >
            <option value="">All enabled dictionaries</option>
            {dictionaries.filter((item) => item.enabled).map((item) => (
              <option key={item.id} value={item.id}>
                {item.title || item.name}
              </option>
            ))}
          </select>
        </label>

        <button className="primary-button search-button" type="submit" disabled={loading || searching}>
          {searching ? 'Searching…' : 'Search'}
        </button>
      </form>

      {error ? <div className="error-banner">{error}</div> : null}

      {results.length === 0 ? (
        <div className="empty-state card">
          <h3>No results yet</h3>
          <p className="muted">Run a search after uploading and enabling at least one dictionary.</p>
        </div>
      ) : (
        <div className="results-grid">
          {Object.entries(grouped).map(([dictionaryName, items]) => (
            <section key={dictionaryName} className="card result-group">
              <div className="result-group-header">
                <h3>{dictionaryName}</h3>
                <span>{items.length} result(s)</span>
              </div>
              <div className="result-list">
                {items.map((item, index) => (
                  <article key={`${item.dictionary_id}-${item.word}-${index}`} className="result-card">
                    <div className="result-meta">
                      <strong>{item.word}</strong>
                      <span>
                        {item.source} · score {item.score.toFixed(2)}
                      </span>
                    </div>
                    <div className="definition-html" dangerouslySetInnerHTML={{ __html: item.html }} />
                  </article>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </section>
  )
}
