import { ArrowUp, MagnifyingGlass, SignOut, SlidersHorizontal, StackSimple, UserCircle, X } from '@phosphor-icons/react'
import { useCallback, useEffect, useState } from 'react'

import { AuthPanel } from './components/AuthPanel'
import { SettingsPanel } from './components/SettingsPanel'
import { I18nContext, messages } from './i18n'
import { DictionaryManagerPage } from './pages/DictionaryManagerPage'
import { SearchPage } from './pages/SearchPage'
import { api, ApiError } from './services/api'
import type { DictionarySummary, HealthInfo, MaintenanceReport, SearchResult, SharedFont, SystemSettings, UserPreferences, UserSummary } from './types'
import './App.css'

type Page = 'search' | 'manage'

type ControlButtonProps = {
  active?: boolean
  icon: React.ReactNode
  label: string
  onClick: () => void
}

const TOKEN_KEY = 'owl-token'
const RECENT_SEARCHES_KEY = 'owl-recent-searches'
const PREFERENCES_KEY = 'owl-preferences'
const LOGO_SRC = '/android-chrome-192x192.png'

const DEFAULT_PREFERENCES: UserPreferences = {
  language: 'zh-CN',
  theme: 'system',
  font_mode: 'sans',
  display_name: '',
  avatar_url: '',
  custom_font_name: '',
  custom_font_family: '',
}


function normalizeThemePreference(theme: string): UserPreferences['theme'] {
  if (theme === 'paper' || theme === 'blue' || theme === 'green' || theme === 'retro' || theme === 'ibm' || theme === 'nokia' || theme === 'gameboy' || theme === 'blackberry' || theme === 'nintendo' || theme === 'dark' || theme === 'mono' || theme === 'system') {
    return theme
  }
  if (theme === 'light' || theme === 'sepia') {
    return 'paper'
  }
  return DEFAULT_PREFERENCES.theme
}

function normalizePreferences(preferences: Partial<UserPreferences>): UserPreferences {
  const normalized: UserPreferences = {
    ...DEFAULT_PREFERENCES,
    ...preferences,
    theme: normalizeThemePreference(preferences.theme ?? DEFAULT_PREFERENCES.theme),
    language: preferences.language === 'en' ? 'en' : 'zh-CN',
    font_mode:
      preferences.font_mode === 'serif' || preferences.font_mode === 'mono' || preferences.font_mode === 'custom'
        ? preferences.font_mode
        : 'sans',
  }
  if (normalized.font_mode === 'custom' && normalized.custom_font_name) {
    const selectedFont = normalized.available_fonts?.find((font) => font.name === normalized.custom_font_name)
    if (selectedFont) {
      normalized.custom_font_family = selectedFont.family
      normalized.custom_font_url = selectedFont.url
    }
  }
  return normalized
}

function mergeAvailableFonts(preferences: UserPreferences, availableFonts: SharedFont[]): UserPreferences {
  return normalizePreferences({ ...preferences, available_fonts: availableFonts })
}

function cssString(value: string) {
  return value.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
}

function fontFormatFromURL(url: string) {
  const cleanURL = url.split('?', 1)[0]?.toLowerCase() ?? ''
  if (cleanURL.endsWith('.woff2')) return 'woff2'
  if (cleanURL.endsWith('.woff')) return 'woff'
  if (cleanURL.endsWith('.otf')) return 'opentype'
  return 'truetype'
}
function SiteFooter({ settings, label }: { settings: Pick<SystemSettings, 'footer_extra' | 'copyright'> | null; label: string }) {
  const footerExtra = settings?.footer_extra?.trim() ?? ''
  const copyright = settings?.copyright?.trim() ?? ''
  if (!footerExtra && !copyright) return null

  return (
    <footer className="site-footer" aria-label={label}>
      {footerExtra ? <p>{footerExtra}</p> : null}
      {copyright ? <p>{copyright}</p> : null}
    </footer>
  )
}

function readStoredPreferences(): UserPreferences {
  try {
    const raw = localStorage.getItem(PREFERENCES_KEY)
    if (!raw) return { ...DEFAULT_PREFERENCES }
    return normalizePreferences(JSON.parse(raw) as Partial<UserPreferences>)
  } catch {
    return { ...DEFAULT_PREFERENCES }
  }
}

function ControlButton({ active = false, icon, label, onClick }: ControlButtonProps) {
  return (
    <button className={active ? 'deck-key active' : 'deck-key'} type="button" onClick={onClick} title={label} aria-label={label}>
      <span className={active ? 'deck-key-led active' : 'deck-key-led'} aria-hidden="true" />
      <span className="deck-key-icon" aria-hidden="true">{icon}</span>
      <span className="deck-key-label">{label}</span>
    </button>
  )
}

function IconControlButton({ icon, label, onClick, danger = false }: { icon: React.ReactNode; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button
      className={danger ? 'transport-button danger' : 'transport-button'}
      type="button"
      onClick={onClick}
      title={label}
      aria-label={label}
    >
      {icon}
    </button>
  )
}

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
  const [systemSettings, setSystemSettings] = useState<SystemSettings | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [authOpen, setAuthOpen] = useState(false)
  const [showBackToTop, setShowBackToTop] = useState(false)
  const [preferences, setPreferences] = useState<UserPreferences>(() => readStoredPreferences())
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
    localStorage.setItem(PREFERENCES_KEY, JSON.stringify({
      language: preferences.language,
      theme: preferences.theme,
      font_mode: preferences.font_mode,
      custom_font_name: preferences.custom_font_name,
    }))
  }, [preferences.language, preferences.theme, preferences.font_mode, preferences.custom_font_name])

  useEffect(() => {
    const resolvedTheme = preferences.theme === 'system'
      ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'retro')
      : preferences.theme
    document.documentElement.dataset.theme = resolvedTheme
  }, [preferences.theme])

  useEffect(() => {
    const updateBackToTopVisibility = () => {
      const scrollTop = window.scrollY || document.documentElement.scrollTop || document.body.scrollTop || 0
      const shouldShow = scrollTop > 420
      setShowBackToTop((current) => (current === shouldShow ? current : shouldShow))
    }

    updateBackToTopVisibility()
    window.addEventListener('scroll', updateBackToTopVisibility, { passive: true })
    return () => window.removeEventListener('scroll', updateBackToTopVisibility)
  }, [])

  useEffect(() => {
    const fontMode = preferences.font_mode
    const root = document.documentElement
    let styleElement = document.getElementById('owl-custom-font-style') as HTMLStyleElement | null
    if (!styleElement) {
      styleElement = document.createElement('style')
      styleElement.id = 'owl-custom-font-style'
      document.head.appendChild(styleElement)
    }

    styleElement.textContent = (preferences.available_fonts ?? [])
      .filter((font) => font.url && font.family)
      .map((font) => `@font-face { font-family: '${cssString(font.family)}'; src: url('${cssString(font.url)}') format('${fontFormatFromURL(font.url)}'); font-display: swap; }`)
      .join('\n')

    root.dataset.fontMode = fontMode
    root.style.setProperty('--reader-font-family',
      fontMode === 'serif'
        ? "'Noto Serif', 'Source Han Serif SC', 'Songti SC', Georgia, serif"
        : fontMode === 'mono'
          ? "'JetBrains Mono', 'SFMono-Regular', Consolas, monospace"
          : fontMode === 'custom' && preferences.custom_font_family
            ? `'${preferences.custom_font_family}', system-ui, sans-serif`
            : "Inter, 'Noto Sans', 'Source Han Sans SC', system-ui, sans-serif")
  }, [preferences.font_mode, preferences.custom_font_family, preferences.available_fonts])

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

  const handleAuthFailure = useCallback((error: unknown) => {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
    setDictionaries([])
    setResults([])
    setSystemSettings(null)
    setAuthError(getErrorMessage(error, t.genericError))
  }, [t.genericError])

  useEffect(() => {
    if (!token) {
      let active = true
      async function loadGuestDictionaries() {
        try {
          const [dicts, fonts] = await Promise.all([api.listPublicDictionaries(), api.listPublicFonts()])
          if (!active) return
          setDictionaries(dicts)
          setPreferences((current) => mergeAvailableFonts(current, fonts))
          setDictionaryError('')
        } catch (error) {
          if (!active) return
          setDictionaryError(getErrorMessage(error, t.genericError))
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
        setPreferences(normalizePreferences(prefs))
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
  }, [handleAuthFailure, t.genericError, token])

  useEffect(() => {
    if (!token || !user?.is_admin) {
      return
    }
    let active = true
    async function loadSystemSettings() {
      try {
        const next = await api.getSystemSettings(token as string)
        if (!active) return
        setSystemSettings(next)
      } catch {
        if (!active) return
        setSystemSettings(null)
      }
    }
    void loadSystemSettings()
    return () => {
      active = false
    }
  }, [token, user?.is_admin])

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
      setAuthError(getErrorMessage(error, t.genericError))
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
      setAuthError(getErrorMessage(error, t.genericError))
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
      setDictionaryError(getErrorMessage(error, t.genericError))
    } finally {
      setDictionaryLoading(false)
    }
  }

  async function handleSearch(query: string, dictionaryId?: number) {
    const normalizedQuery = query.trim()
    if (!normalizedQuery) {
      setResults([])
      setSearchError('')
      setSearching(false)
      return
    }
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
      setSearchError(getErrorMessage(error, t.genericError))
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
    if (!token) return false
    if (!window.confirm(t.deleteDictionaryConfirm(dictionary.title || dictionary.name))) {
      return false
    }
    await api.deleteDictionary(token, dictionary.id)
    setMaintenanceReport(null)
    await refreshDictionaries()
    setResults((current) => current.filter((item) => item.dictionary_id !== dictionary.id))
    return true
  }

  function handleLogout() {
    void api.logout().catch(() => {})
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
    setDictionaries([])
    setResults([])
    setMaintenanceReport(null)
    setSystemSettings(null)
  }

  function handleBackToTop() {
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const scrollOptions: ScrollToOptions = { top: 0, left: 0, behavior: prefersReducedMotion ? 'auto' : 'smooth' }
    window.scrollTo(scrollOptions)
    document.scrollingElement?.scrollTo(scrollOptions)
  }

  async function updatePreferences(patch: Partial<Pick<UserPreferences, 'language' | 'theme' | 'font_mode' | 'display_name' | 'custom_font_name'>>) {
    const optimistic = normalizePreferences({ ...preferences, ...patch })
    setPreferences(optimistic)
    if (!token) {
      return
    }
    const next = await api.updatePreferences(token, {
      language: optimistic.language,
      theme: optimistic.theme,
      font_mode: optimistic.font_mode,
      display_name: optimistic.display_name,
      custom_font_name: optimistic.custom_font_name,
    })
    setPreferences(normalizePreferences(next))
  }

  async function handleFontUpload(file: File) {
    if (!token) return
    const next = await api.uploadFont(token, file)
    setPreferences(normalizePreferences(next))
  }

  async function handleAvatarUpload(file: File) {
    if (!token) return
    const next = await api.uploadAvatar(token, file)
    setPreferences(normalizePreferences(next))
    setUser((current) => (current ? { ...current, avatar_url: next.avatar_url } : current))
  }

  async function handleDeleteFont(name: string) {
    if (!token) return
    const next = await api.deleteFont(token, name)
    setPreferences(normalizePreferences(next))
  }

  async function handleSystemSettingsChange(next: SystemSettings) {
    if (!token || !user?.is_admin) return
    const updated = await api.updateSystemSettings(token, next)
    setSystemSettings(updated)
    setHealthInfo((current) => (current ? { ...current, allow_register: updated.allow_register, footer_extra: updated.footer_extra, copyright: updated.copyright } : current))
  }


  const userSubtitle = user?.display_name ? t.workspaceSubtitle(user.display_name) : t.genericWorkspaceSubtitle
  const publicSystemSettings = systemSettings ?? (healthInfo ? { allow_register: healthInfo.allow_register, footer_extra: healthInfo.footer_extra ?? '', copyright: healthInfo.copyright ?? '' } : null)

  const settingsButton = <IconControlButton icon={<SlidersHorizontal size={18} weight="bold" />} label={t.settings} onClick={() => setSettingsOpen(true)} />

  if (!token || !user) {
    return (
      <I18nContext.Provider value={{ language: preferences.language, t }}>
        <div className="app-shell">
          <header className="topbar recorder-topbar minimal-topbar">
            <div className="brand-block recorder-brand recorder-brand-minimal">
              <div className="brand-icon recorder-brand-icon">
                <img src={LOGO_SRC} alt={t.owlLogoAlt} className="brand-logo-image" />
              </div>
            </div>
            <div className="control-deck guest-control-deck compact-deck">
              <div className="mode-rail mode-rail-bare" aria-label={t.dictionaryModesLabel}>
                <ControlButton active icon={<MagnifyingGlass size={20} weight="fill" />} label={t.search} onClick={() => undefined} />
              </div>
              <div className="transport-cluster">{settingsButton}</div>
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
            </section>
          </main>

          <SiteFooter settings={publicSystemSettings} label={t.siteFooter} />

          {authOpen ? (
            <div className="settings-overlay" role="presentation" onClick={() => setAuthOpen(false)}>
              <div className="auth-modal-shell recorder-modal-shell" role="dialog" aria-modal="true" aria-label={t.login} onClick={(event) => event.stopPropagation()}>
                <div className="settings-drawer-head recorder-drawer-head">
                  <div>
                    <div className="eyebrow">Owl</div>
                  </div>
                  <IconControlButton icon={<X size={18} weight="bold" />} label={t.close} onClick={() => setAuthOpen(false)} />
                </div>
                <AuthPanel loading={authLoading} error={authError} allowRegister={healthInfo?.allow_register ?? true} onLogin={handleLogin} onRegister={handleRegister} />
              </div>
            </div>
          ) : null}

          {settingsOpen ? (
            <div className="settings-overlay" role="presentation" onClick={() => setSettingsOpen(false)}>
              <div className="settings-drawer recorder-drawer" role="dialog" aria-modal="true" aria-label={t.settings} onClick={(event) => event.stopPropagation()}>
                <div className="settings-drawer-head recorder-drawer-head">
                  <div>
                    <div className="eyebrow">{t.settings}</div>
                    <h3>{t.preferences}</h3>
                    <p className="muted drawer-subcopy">{healthInfo ? `${t.versionLabel}: ${healthInfo.version}` : t.appTagline}</p>
                  </div>
                  <IconControlButton icon={<X size={18} weight="bold" />} label={t.close} onClick={() => setSettingsOpen(false)} />
                </div>
                <div className="drawer-user-card guest-drawer-card compact-user-card">
                  <UserCircle size={28} weight="duotone" />
                  <div className="drawer-user-meta">
                    <strong>{t.guest}</strong>
                    <span className="muted">{t.scopeAllPublic}</span>
                  </div>
                </div>
                <div className="guest-auth-actions">
                  <button className="primary-button guest-login-button" type="button" onClick={() => { setSettingsOpen(false); setAuthOpen(true) }}>
                    {t.login}
                  </button>
                </div>
                <SettingsPanel
                  preferences={preferences}
                  onLanguageChange={async (language) => updatePreferences({ language })}
                  onThemeChange={async (theme) => updatePreferences({ theme })}
                  onFontModeChange={async (font_mode) => updatePreferences({ font_mode })}
                  onCustomFontSelect={async (custom_font_name) => updatePreferences({ font_mode: 'custom', custom_font_name })}
                  onDisplayNameChange={async () => Promise.resolve()}
                  onAvatarUpload={async () => Promise.resolve()}
                  showProfileEditor={false}
                />
              </div>
            </div>
          ) : null}

          {showBackToTop ? (
            <button className="back-to-top-button" type="button" onClick={handleBackToTop} aria-label={t.scrollToTopLabel}>
              <ArrowUp size={22} weight="bold" aria-hidden="true" />
            </button>
          ) : null}
        </div>
      </I18nContext.Provider>
    )
  }

  return (
    <I18nContext.Provider value={{ language: preferences.language, t }}>
      <div className="app-shell">
        <header className="topbar recorder-topbar compact-topbar">
          <div className="control-deck compact-deck main-control-deck">
            <div className="mode-rail mode-rail-bare" aria-label={t.dictionaryModesLabel}>
              <ControlButton active={page === 'search'} icon={<MagnifyingGlass size={20} weight={page === 'search' ? 'fill' : 'regular'} />} label={t.search} onClick={() => setPage('search')} />
              <ControlButton active={page === 'manage'} icon={<StackSimple size={20} weight={page === 'manage' ? 'fill' : 'regular'} />} label={t.manage} onClick={() => setPage('manage')} />
            </div>
            <div className="transport-cluster">{settingsButton}</div>
          </div>
        </header>

        <main className="dashboard-main recorder-main-shell">
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
              preferences={preferences}
              isAdmin={user.is_admin}
              systemSettings={publicSystemSettings}
              onSystemSettingsChange={handleSystemSettingsChange}
              onRefresh={refreshDictionaries}
              onRefreshLibrary={handleRefreshLibrary}
              onUpload={handleUpload}
              onToggle={handleToggle}
              onTogglePublic={handleTogglePublic}
              onRefreshItem={handleRefreshDictionary}
              onDelete={handleDelete}
              onLanguageChange={async (language) => updatePreferences({ language })}
              onThemeChange={async (theme) => updatePreferences({ theme })}
              onDisplayNameChange={async (display_name) => updatePreferences({ display_name })}
              onFontUpload={handleFontUpload}
              onDeleteFont={handleDeleteFont}
              onAvatarUpload={handleAvatarUpload}
            />
          )}
        </main>

        <SiteFooter settings={publicSystemSettings} label={t.siteFooter} />

        {settingsOpen ? (
          <div className="settings-overlay" role="presentation" onClick={() => setSettingsOpen(false)}>
            <div className="settings-drawer recorder-drawer" role="dialog" aria-modal="true" aria-label={t.settings} onClick={(event) => event.stopPropagation()}>
              <div className="settings-drawer-head recorder-drawer-head">
                <div>
                  <div className="eyebrow">{t.settings}</div>
                  <h3>{t.preferences}</h3>
                  <p className="muted drawer-subcopy">{healthInfo ? `${t.versionLabel}: ${healthInfo.version}` : userSubtitle}</p>
                </div>
                <div className="drawer-window-controls">
                  <IconControlButton icon={<SignOut size={18} weight="bold" />} label={t.logout} onClick={handleLogout} danger />
                  <IconControlButton icon={<X size={18} weight="bold" />} label={t.close} onClick={() => setSettingsOpen(false)} />
                </div>
              </div>
              <div className="drawer-user-card compact-user-card">
                {user.avatar_url ? <img src={user.avatar_url} alt={user.display_name || user.username} className="drawer-avatar" /> : <UserCircle size={32} weight="duotone" />}
                <div className="drawer-user-meta">
                  <strong>{user.display_name || user.username}</strong>
                  <span className="muted">@{user.username}</span>
                </div>
              </div>
              <SettingsPanel
                preferences={preferences}
                onLanguageChange={async (language) => updatePreferences({ language })}
                onThemeChange={async (theme) => updatePreferences({ theme })}
                onFontModeChange={async (font_mode) => updatePreferences({ font_mode })}
                onDisplayNameChange={async (display_name) => updatePreferences({ display_name })}
                onCustomFontSelect={async (custom_font_name) => updatePreferences({ font_mode: 'custom', custom_font_name })}
                onAvatarUpload={handleAvatarUpload}
              />
            </div>
          </div>
        ) : null}

        {showBackToTop ? (
          <button className="back-to-top-button" type="button" onClick={handleBackToTop} aria-label={t.scrollToTopLabel}>
            <ArrowUp size={22} weight="bold" aria-hidden="true" />
          </button>
        ) : null}
      </div>
    </I18nContext.Provider>
  )
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    return error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return fallback
}
