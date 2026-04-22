import { useEffect, useState } from 'react'

import { AuthPanel } from './components/AuthPanel'
import { SettingsPanel } from './components/SettingsPanel'
import { I18nContext, messages } from './i18n'
import { DictionaryManagerPage } from './pages/DictionaryManagerPage'
import { SearchPage } from './pages/SearchPage'
import { api, ApiError } from './services/api'
import type { DictionarySummary, HealthInfo, MaintenanceReport, SearchResult, UserPreferences, UserSummary } from './types'
import './App.css'

type Page = 'search' | 'manage'

const TOKEN_KEY = 'owl-token'
const RECENT_SEARCHES_KEY = 'owl-recent-searches'
const LOGO_SRC = '/android-chrome-192x192.png'

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY))
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
  const [maintenanceReport, setMaintenanceReport] = useState<MaintenanceReport | null>(null)
  const [healthInfo, setHealthInfo] = useState<HealthInfo | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [authOpen, setAuthOpen] = useState(false)
  const [preferences, setPreferences] = useState<UserPreferences>({
    language: 'zh-CN',
    theme: 'system',
    font_mode: 'sans',
    custom_font_name: '',
    custom_font_family: '',
  })
  const [recentSearches, setRecentSearches] = useState<string[]>(() => {
    try {
      const raw = localStorage.getItem(RECENT_SEARCHES_KEY)
      return raw ? (JSON.parse(raw) as string[]) : []
    } catch {
      return []
    }
  })

  const t = messages[preferences.language]

  useEffect(() => {
    const resolvedTheme = preferences.theme === 'system'
      ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
      : preferences.theme
    document.documentElement.dataset.theme = resolvedTheme
  }, [preferences.theme])

  useEffect(() => {
    const fontMode = preferences.font_mode
    const root = document.documentElement
    let styleElement = document.getElementById('owl-custom-font-style') as HTMLStyleElement | null
    if (!styleElement) {
      styleElement = document.createElement('style')
      styleElement.id = 'owl-custom-font-style'
      document.head.appendChild(styleElement)
    }

    if (fontMode === 'custom' && preferences.custom_font_url && preferences.custom_font_family) {
      styleElement.textContent = `@font-face { font-family: '${preferences.custom_font_family}'; src: url('${preferences.custom_font_url}') format('woff2'); font-display: swap; }`
    } else {
      styleElement.textContent = ''
    }

    root.dataset.fontMode = fontMode
    root.style.setProperty('--reader-font-family',
      fontMode === 'serif'
        ? "'Noto Serif', 'Source Han Serif SC', 'Songti SC', Georgia, serif"
        : fontMode === 'mono'
          ? "'JetBrains Mono', 'SFMono-Regular', Consolas, monospace"
          : fontMode === 'custom' && preferences.custom_font_family
            ? `'${preferences.custom_font_family}', system-ui, sans-serif`
            : "Inter, 'Noto Sans', 'Source Han Sans SC', system-ui, sans-serif")
  }, [preferences])

  useEffect(() => {
    let active = true
    async function loadHealth() {
      try {
        const info = await api.health()
        if (!active) return
        setHealthInfo(info)
      } catch {
        if (!active) return
        setHealthInfo(null)
      }
    }
    void loadHealth()
    return () => {
      active = false
    }
  }, [])

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
      let active = true
      async function loadGuestDictionaries() {
        try {
          const dicts = await api.listPublicDictionaries()
          if (!active) return
          setDictionaries(dicts)
          setDictionaryError('')
        } catch (error) {
          if (!active) return
          setDictionaryError(getErrorMessage(error))
        }
      }
      void loadGuestDictionaries()
      return () => {
        active = false
      }
    }
    const authToken = token

    let active = true
    async function bootstrap() {
      try {
        const [me, dicts, prefs] = await Promise.all([
          api.me(authToken),
          api.listDictionaries(authToken),
          api.getPreferences(authToken),
        ])
        if (!active) return
        setUser(me)
        setDictionaries(dicts)
        setPreferences(prefs)
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


  async function handleLogin(username: string, password: string) {
    setAuthLoading(true)
    setAuthError('')
    try {
      const response = await api.login(username, password)
      setToken(response.token)
      localStorage.setItem(TOKEN_KEY, response.token)
      setUser(response.user)
      setAuthOpen(false)
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
      setAuthOpen(false)
    } catch (error) {
      setAuthError(getErrorMessage(error))
    } finally {
      setAuthLoading(false)
    }
  }

  async function refreshDictionaries() {
    setDictionaryLoading(true)
    setDictionaryError('')
    try {
      const dicts = token ? await api.listDictionaries(token) : await api.listPublicDictionaries()
      setDictionaries(dicts)
    } catch (error) {
      setDictionaryError(getErrorMessage(error))
    } finally {
      setDictionaryLoading(false)
    }
  }

  async function handleSearch(query: string, dictionaryId?: number) {
    const normalizedQuery = query.trim()
    if (!normalizedQuery) return
    setSearching(true)
    setSearchError('')
    try {
      const data = token
        ? await api.search(token, normalizedQuery, dictionaryId)
        : await api.publicSearch(normalizedQuery, dictionaryId)
      setResults(data)
      setRecentSearches((current) => {
        const next = [normalizedQuery, ...current.filter((item) => item !== normalizedQuery)].slice(0, 8)
        localStorage.setItem(RECENT_SEARCHES_KEY, JSON.stringify(next))
        return next
      })
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
    setMaintenanceReport(null)
    await refreshDictionaries()
  }

  async function handleToggle(dictionary: DictionarySummary) {
    if (!token) return
    await api.toggleDictionary(token, dictionary.id, !dictionary.enabled)
    setMaintenanceReport(null)
    await refreshDictionaries()
  }

  async function handleTogglePublic(dictionary: DictionarySummary) {
    if (!token) return
    await api.setDictionaryPublic(token, dictionary.id, !dictionary.public)
    setMaintenanceReport(null)
    await refreshDictionaries()
  }

  async function handleRefreshDictionary(dictionary: DictionarySummary) {
    if (!token) return
    const report = await api.refreshDictionary(token, dictionary.id)
    setMaintenanceReport(report)
    await refreshDictionaries()
  }

  async function handleRefreshLibrary() {
    if (!token) return
    const report = await api.refreshLibrary(token)
    setMaintenanceReport(report)
    await refreshDictionaries()
  }

  async function handleDelete(dictionary: DictionarySummary) {
    if (!token) return
    if (!window.confirm(t.deleteDictionaryConfirm(dictionary.title || dictionary.name))) {
      return
    }
    await api.deleteDictionary(token, dictionary.id)
    setMaintenanceReport(null)
    await refreshDictionaries()
    setResults((current) => current.filter((item) => item.dictionary_id !== dictionary.id))
  }

  function handleLogout() {
    void api.logout().catch(() => {})
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
    setDictionaries([])
    setResults([])
    setMaintenanceReport(null)
  }

  async function updatePreferences(patch: Partial<Pick<UserPreferences, 'language' | 'theme' | 'font_mode'>>) {
    if (!token) {
      setPreferences((current) => ({
        ...current,
        ...patch,
      }))
      return
    }
    const next = await api.updatePreferences(token, {
      language: patch.language ?? preferences.language,
      theme: patch.theme ?? preferences.theme,
      font_mode: patch.font_mode ?? preferences.font_mode,
    })
    setPreferences(next)
  }

  async function handleFontUpload(file: File) {
    if (!token) return
    const next = await api.uploadFont(token, file)
    setPreferences(next)
  }

  if (!token || !user) {
    return (
      <I18nContext.Provider value={{ language: preferences.language, t }}>
        <div className="app-shell">
          <header className="topbar minimal-topbar">
            <div className="brand-block">
              <div className="brand-icon">
                <img src={LOGO_SRC} alt="Owl logo" className="brand-logo-image" />
              </div>
              <div>
                <strong>Owl</strong>
                <p>{healthInfo ? `${healthInfo.full_version} · ${healthInfo.os}/${healthInfo.arch}` : t.appTagline}</p>
              </div>
            </div>
            <div className="toolbar-actions">
              <button className="secondary-button" type="button" onClick={() => setAuthOpen(true)}>
                {t.login}
              </button>
              <button className="secondary-button" type="button" onClick={() => setSettingsOpen(true)}>
                {t.settings}
              </button>
            </div>
          </header>

          <main className="auth-main">
            <section className="guest-search-shell">
              <SearchPage
                dictionaries={dictionaries}
                loading={dictionaryLoading}
                searching={searching}
                results={results}
                error={searchError || dictionaryError}
                isGuest
                token={null}
                recentSearches={recentSearches}
                onSearch={handleSearch}
              />
              <p className="guest-search-hint">{t.guestSearchHint}</p>
            </section>
          </main>

          {authOpen ? (
            <div className="settings-overlay" role="presentation" onClick={() => setAuthOpen(false)}>
              <div className="auth-modal-shell" role="dialog" aria-modal="true" aria-label={t.login} onClick={(event) => event.stopPropagation()}>
                <div className="settings-drawer-head">
                  <div>
                    <div className="eyebrow">Owl</div>
                    <h3>{t.login} / {t.register}</h3>
                  </div>
                  <button className="secondary-button" type="button" onClick={() => setAuthOpen(false)}>
                    {t.close}
                  </button>
                </div>
                <AuthPanel loading={authLoading} error={authError} onLogin={handleLogin} onRegister={handleRegister} />
              </div>
            </div>
          ) : null}

          {settingsOpen ? (
            <div className="settings-overlay" role="presentation" onClick={() => setSettingsOpen(false)}>
              <div className="settings-drawer" role="dialog" aria-modal="true" aria-label={t.settings} onClick={(event) => event.stopPropagation()}>
                <div className="settings-drawer-head">
                  <div>
                    <div className="eyebrow">{t.settings}</div>
                    <h3>{t.preferences}</h3>
                  </div>
                  <button className="secondary-button" type="button" onClick={() => setSettingsOpen(false)}>
                    {t.close}
                  </button>
                </div>
                {healthInfo ? (
                  <p className="settings-build-meta">{t.versionLabel}: {healthInfo.version}</p>
                ) : null}
                <SettingsPanel
                  preferences={preferences}
                  onLanguageChange={async (language) => updatePreferences({ language })}
                  onThemeChange={async (theme) => updatePreferences({ theme })}
                  onFontModeChange={async (font_mode) => updatePreferences({ font_mode })}
                  onFontUpload={handleFontUpload}
                />
              </div>
            </div>
          ) : null}
        </div>
      </I18nContext.Provider>
    )
  }

  return (
    <I18nContext.Provider value={{ language: preferences.language, t }}>
      <div className="app-shell">
        <header className="topbar">
          <div className="brand-block">
            <div className="brand-icon">
              <img src={LOGO_SRC} alt="Owl logo" className="brand-logo-image" />
            </div>
            <div>
              <strong>Owl</strong>
              <p>{user.username ? t.workspaceSubtitle(user.username) : t.genericWorkspaceSubtitle}</p>
            </div>
          </div>

        <nav className="nav-tabs">
          <button className={page === 'search' ? 'active' : ''} type="button" onClick={() => setPage('search')}>
            {t.search}
          </button>
          <button className={page === 'manage' ? 'active' : ''} type="button" onClick={() => setPage('manage')}>
            {t.manage}
          </button>
        </nav>

          <div className="toolbar-actions">
            {user.is_admin ? <span className="status-pill active">{t.admin}</span> : null}
            <button className="secondary-button" type="button" onClick={() => setSettingsOpen(true)}>
              {t.settings}
            </button>
            <button className="secondary-button" type="button" onClick={handleLogout}>
              {t.logout}
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
              isGuest={false}
              token={token}
              recentSearches={recentSearches}
              onSearch={handleSearch}
            />
          ) : (
            <DictionaryManagerPage
              dictionaries={dictionaries}
              loading={dictionaryLoading}
              error={dictionaryError}
              maintenanceReport={maintenanceReport}
              onRefresh={refreshDictionaries}
              onRefreshLibrary={handleRefreshLibrary}
              onUpload={handleUpload}
              onToggle={handleToggle}
              onTogglePublic={handleTogglePublic}
              onRefreshItem={handleRefreshDictionary}
              onDelete={handleDelete}
            />
          )}
        </main>

        {settingsOpen ? (
          <div className="settings-overlay" role="presentation" onClick={() => setSettingsOpen(false)}>
            <div className="settings-drawer" role="dialog" aria-modal="true" aria-label={t.settings} onClick={(event) => event.stopPropagation()}>
              <div className="settings-drawer-head">
                <div>
                  <div className="eyebrow">{t.settings}</div>
                  <h3>{t.preferences}</h3>
                </div>
                <button className="secondary-button" type="button" onClick={() => setSettingsOpen(false)}>
                  {t.close}
                </button>
              </div>
              <SettingsPanel
                preferences={preferences}
                onLanguageChange={async (language) => updatePreferences({ language })}
                onThemeChange={async (theme) => updatePreferences({ theme })}
                onFontModeChange={async (font_mode) => updatePreferences({ font_mode })}
                onFontUpload={handleFontUpload}
              />
            </div>
          </div>
        ) : null}
      </div>
    </I18nContext.Provider>
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
