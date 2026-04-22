import { useEffect, useMemo, useState } from 'react'

import { AuthPanel } from './components/AuthPanel'
import { ThemeToggle } from './components/ThemeToggle'
import { DictionaryManagerPage } from './pages/DictionaryManagerPage'
import { SearchPage } from './pages/SearchPage'
import { api, ApiError } from './services/api'
import type { DictionarySummary, SearchResult, UserSummary } from './types'
import './App.css'

type Page = 'search' | 'manage'
type Theme = 'light' | 'dark'

const TOKEN_KEY = 'owl-token'
const THEME_KEY = 'owl-theme'

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY))
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem(THEME_KEY) as Theme) || 'dark')
  const [user, setUser] = useState<UserSummary | null>(null)
  const [page, setPage] = useState<Page>('search')
  const [authLoading, setAuthLoading] = useState(false)
  const [authError, setAuthError] = useState('')
  const [dictionaries, setDictionaries] = useState<DictionarySummary[]>([])
  const [dictionaryLoading, setDictionaryLoading] = useState(false)
  const [dictionaryError, setDictionaryError] = useState('')
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem(THEME_KEY, theme)
  }, [theme])

  function handleAuthFailure(error: unknown) {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
    setDictionaries([])
    setResults([])
    setAuthError(getErrorMessage(error))
  }

  useEffect(() => {
    if (!token) {
      return
    }
    const authToken = token

    let active = true
    async function bootstrap() {
      try {
        const [me, dicts] = await Promise.all([api.me(authToken), api.listDictionaries(authToken)])
        if (!active) return
        setUser(me)
        setDictionaries(dicts)
        setDictionaryError('')
      } catch (error) {
        if (!active) return
        handleAuthFailure(error)
      }
    }
    void bootstrap()
    return () => {
      active = false
    }
  }, [token])

  const enabledDictionaryCount = useMemo(() => dictionaries.filter((item) => item.enabled).length, [dictionaries])

  async function handleLogin(username: string, password: string) {
    setAuthLoading(true)
    setAuthError('')
    try {
      const response = await api.login(username, password)
      setToken(response.token)
      localStorage.setItem(TOKEN_KEY, response.token)
      setUser(response.user)
    } catch (error) {
      setAuthError(getErrorMessage(error))
    } finally {
      setAuthLoading(false)
    }
  }

  async function handleRegister(username: string, password: string) {
    setAuthLoading(true)
    setAuthError('')
    try {
      const response = await api.register(username, password)
      setToken(response.token)
      localStorage.setItem(TOKEN_KEY, response.token)
      setUser(response.user)
    } catch (error) {
      setAuthError(getErrorMessage(error))
    } finally {
      setAuthLoading(false)
    }
  }

  async function refreshDictionaries() {
    if (!token) return
    setDictionaryLoading(true)
    setDictionaryError('')
    try {
      const dicts = await api.listDictionaries(token)
      setDictionaries(dicts)
    } catch (error) {
      setDictionaryError(getErrorMessage(error))
    } finally {
      setDictionaryLoading(false)
    }
  }

  async function handleSearch(query: string, dictionaryId?: number) {
    if (!token) return
    setSearching(true)
    setSearchError('')
    try {
      const data = await api.search(token, query, dictionaryId)
      setResults(data)
    } catch (error) {
      setResults([])
      setSearchError(getErrorMessage(error))
    } finally {
      setSearching(false)
    }
  }

  async function handleUpload(mdxFile: File, mddFiles: File[]) {
    if (!token) return
    await api.uploadDictionary(token, mdxFile, mddFiles)
    await refreshDictionaries()
  }

  async function handleToggle(dictionary: DictionarySummary) {
    if (!token) return
    await api.toggleDictionary(token, dictionary.id, !dictionary.enabled)
    await refreshDictionaries()
  }

  async function handleDelete(dictionary: DictionarySummary) {
    if (!token) return
    if (!window.confirm(`Delete dictionary “${dictionary.title || dictionary.name}”?`)) {
      return
    }
    await api.deleteDictionary(token, dictionary.id)
    await refreshDictionaries()
    setResults((current) => current.filter((item) => item.dictionary_id !== dictionary.id))
  }

  function handleLogout() {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
    setDictionaries([])
    setResults([])
  }

  if (!token || !user) {
    return (
      <div className="app-shell">
        <header className="topbar minimal-topbar">
          <div className="brand-block">
            <div className="brand-icon">🦉</div>
            <div>
              <strong>Owl</strong>
              <p>Web dictionary for MDX / MDD</p>
            </div>
          </div>
          <ThemeToggle theme={theme} onToggle={() => setTheme((current) => (current === 'dark' ? 'light' : 'dark'))} />
        </header>

        <main className="auth-main">
          <AuthPanel loading={authLoading} error={authError} onLogin={handleLogin} onRegister={handleRegister} />
        </main>
      </div>
    )
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand-block">
          <div className="brand-icon">🦉</div>
          <div>
            <strong>Owl</strong>
            <p>
              {user.username} · {enabledDictionaryCount} enabled dictionaries
            </p>
          </div>
        </div>

        <nav className="nav-tabs">
          <button className={page === 'search' ? 'active' : ''} type="button" onClick={() => setPage('search')}>
            Search
          </button>
          <button className={page === 'manage' ? 'active' : ''} type="button" onClick={() => setPage('manage')}>
            Manage
          </button>
        </nav>

        <div className="toolbar-actions">
          {user.is_admin ? <span className="status-pill active">Admin</span> : null}
          <ThemeToggle theme={theme} onToggle={() => setTheme((current) => (current === 'dark' ? 'light' : 'dark'))} />
          <button className="secondary-button" type="button" onClick={handleLogout}>
            Logout
          </button>
        </div>
      </header>

      <main className="dashboard-main">
        {page === 'search' ? (
          <SearchPage
            dictionaries={dictionaries}
            loading={dictionaryLoading}
            searching={searching}
            results={results}
            error={searchError}
            onSearch={handleSearch}
          />
        ) : (
          <DictionaryManagerPage
            dictionaries={dictionaries}
            loading={dictionaryLoading}
            error={dictionaryError}
            onRefresh={refreshDictionaries}
            onUpload={handleUpload}
            onToggle={handleToggle}
            onDelete={handleDelete}
          />
        )}
      </main>
    </div>
  )
}

function getErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    return error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return 'Something went wrong'
}
