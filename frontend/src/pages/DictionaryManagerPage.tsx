import { CheckCircle, Eye, EyeSlash, GearSix, Key, MagicWand, PencilSimple, Question, TextAlignLeft, Trash, UploadSimple, X } from '@phosphor-icons/react'
import { useMemo, useState } from 'react'

import { SettingsPanel } from '../components/SettingsPanel'
import { useI18n } from '../i18n'
import type { DictionarySummary, MaintenanceReport, MCPTokenStatus, SystemSettings, UserPreferences } from '../types'

interface DictionaryManagerPageProps {
  dictionaries: DictionarySummary[]
  loading: boolean
  error: string
  maintenanceReport: MaintenanceReport | null
  preferences: UserPreferences
  isAdmin: boolean
  systemSettings: SystemSettings | null
  mcpTokenStatus: MCPTokenStatus | null
  onMCPTokenSave: (token: string) => Promise<MCPTokenStatus | null>
  onMCPTokenGenerate: () => Promise<MCPTokenStatus | null>
  onMCPTokenDelete: () => Promise<MCPTokenStatus | null>
  onSystemSettingsChange: (settings: SystemSettings) => Promise<void>
  onRefresh: () => Promise<void>
  onRefreshLibrary: () => Promise<void>
  onUpload: (mdxFile: File, mddFiles: File[]) => Promise<void>
  onToggle: (dictionary: DictionarySummary) => Promise<void>
  onTogglePublic: (dictionary: DictionarySummary) => Promise<void>
  onRefreshItem: (dictionary: DictionarySummary) => Promise<void>
  onDelete: (dictionary: DictionarySummary) => Promise<boolean>
  onLanguageChange: (language: UserPreferences['language']) => Promise<void>
  onThemeChange: (theme: UserPreferences['theme']) => Promise<void>
  onDisplayNameChange: (displayName: string) => Promise<void>
  onFontUpload: (file: File) => Promise<void>
  onDeleteFont: (name: string) => Promise<void>
  onAvatarUpload: (file: File) => Promise<void>
  onRecentSearchLimitChange: (limit: number) => Promise<void>
}

export function DictionaryManagerPage({
  dictionaries,
  loading,
  error,
  maintenanceReport,
  preferences,
  isAdmin,
  systemSettings,
  mcpTokenStatus,
  onMCPTokenSave,
  onMCPTokenGenerate,
  onMCPTokenDelete,
  onSystemSettingsChange,
  onRefresh,
  onRefreshLibrary,
  onUpload,
  onToggle,
  onTogglePublic,
  onRefreshItem,
  onDelete,
  onLanguageChange,
  onThemeChange,
  onDisplayNameChange,
  onFontUpload,
  onDeleteFont,
  onAvatarUpload,
  onRecentSearchLimitChange,
}: DictionaryManagerPageProps) {
  const { t } = useI18n()
  const [mdxFile, setMdxFile] = useState<File | null>(null)
  const [mddFiles, setMddFiles] = useState<File[]>([])
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [systemSettingsSaving, setSystemSettingsSaving] = useState(false)
  const [systemSettingsError, setSystemSettingsError] = useState('')
  const [actionNotice, setActionNotice] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  const [mcpHelpOpen, setMCPHelpOpen] = useState(false)
  const enabledCount = useMemo(() => dictionaries.filter((item) => item.enabled).length, [dictionaries])


  async function runManagerAction(action: () => Promise<void | boolean>, successMessage: string) {
    setActionNotice(null)
    try {
      const completed = await action()
      if (completed === false) return
      setActionNotice({ type: 'success', message: successMessage })
    } catch (error) {
      setActionNotice({ type: 'error', message: error instanceof Error ? error.message : t.genericError })
    }
  }


  async function handleRecentSearchLimitSave(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formData = new FormData(event.currentTarget)
    const limit = Number(formData.get('recent_search_limit') ?? preferences.recent_search_limit)
    await runManagerAction(async () => {
      await onRecentSearchLimitChange(limit)
    }, t.recentSearchLimitSaved)
  }

  async function handleMCPTokenSave(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formData = new FormData(event.currentTarget)
    const nextToken = String(formData.get('mcp_token') ?? '')
    await runManagerAction(async () => {
      await onMCPTokenSave(nextToken)
    }, t.mcpTokenSaved)
  }

  async function handleMCPTokenGenerate() {
    await runManagerAction(async () => {
      await onMCPTokenGenerate()
    }, t.mcpTokenSaved)
  }

  async function handleMCPTokenDelete() {
    if (!window.confirm(t.deleteMCPTokenConfirm)) return
    await runManagerAction(async () => {
      await onMCPTokenDelete()
    }, t.mcpTokenDeleted)
  }

  const mcpEndpoint = `${window.location.origin}/api/mcp/sse`
  const mcpTokenExample = mcpTokenStatus?.token || t.mcpTokenExample

  async function handleFooterSettingsSave(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!systemSettings) return
    const formData = new FormData(event.currentTarget)
    setSystemSettingsSaving(true)
    setSystemSettingsError('')
    try {
      await onSystemSettingsChange({
        ...systemSettings,
        footer_extra: String(formData.get('footer_extra') ?? ''),
        copyright: String(formData.get('copyright') ?? ''),
      })
      setActionNotice({ type: 'success', message: t.footerSettingsSaved })
    } catch (settingsErr) {
      setSystemSettingsError(settingsErr instanceof Error ? settingsErr.message : t.updateFailed)
    } finally {
      setSystemSettingsSaving(false)
    }
  }

  async function handleRegistrationToggle() {
    if (!systemSettings) return
    setSystemSettingsSaving(true)
    setSystemSettingsError('')
    try {
      await onSystemSettingsChange({ ...systemSettings, allow_register: !systemSettings.allow_register })
      setActionNotice({ type: 'success', message: t.settingsSaved })
    } catch (settingsErr) {
      setSystemSettingsError(settingsErr instanceof Error ? settingsErr.message : t.updateFailed)
    } finally {
      setSystemSettingsSaving(false)
    }
  }

  async function handleUpload(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!mdxFile) {
      setUploadError(t.mdxFileHint)
      return
    }
    setUploading(true)
    setUploadError('')
    try {
      await onUpload(mdxFile, mddFiles)
      setActionNotice({ type: 'success', message: t.dictionaryUploaded })
      setMdxFile(null)
      setMddFiles([])
      await onRefresh()
    } catch (uploadErr) {
      setUploadError(uploadErr instanceof Error ? uploadErr.message : t.uploadFailed)
    } finally {
      setUploading(false)
    }
  }

  return (
    <section className="page-section">
      <div className="page-header manager-header">
        <div>
          <div className="eyebrow">{t.dictionaryManager}</div>
          <h2>{t.managerTitle}</h2>
          <p className="muted">{t.uploadedEnabled(dictionaries.length, enabledCount)}</p>
          <p className="muted">{t.maintenanceTip}</p>
        </div>
        <div className="actions-row wrap">
          <button className="secondary-button" type="button" onClick={() => void runManagerAction(onRefresh, t.dictionaryRefreshed)} disabled={loading}>
            {t.refresh}
          </button>
          <button className="secondary-button" type="button" onClick={() => void runManagerAction(onRefreshLibrary, t.libraryRefreshed)} disabled={loading}>
            {t.refreshLibrary}
          </button>
        </div>
      </div>

      {actionNotice ? (
        <div className={actionNotice.type === 'success' ? 'manager-feedback-toast success' : 'manager-feedback-toast error'} role="status" aria-live="polite">
          {actionNotice.message}
        </div>
      ) : null}


      <section className="card manager-utility-card mcp-access-card">
        <div className="utility-card-head">
          <div>
            <div className="eyebrow">{t.mcpAccess}</div>
            <h3>{t.mcpAccessTitle}</h3>
            <p className="muted">{t.mcpAccessDescription}</p>
          </div>
          <span className="utility-card-icon"><Key size={22} weight="duotone" /></span>
        </div>
        <form key={`${mcpTokenStatus?.hint ?? ''}:${mcpTokenStatus?.token ?? ''}`} className="mcp-token-form" onSubmit={handleMCPTokenSave}>
          <label className="field footer-settings-field">
            <span>{t.mcpToken}</span>
            <input name="mcp_token" defaultValue={mcpTokenStatus?.token ?? ''} placeholder={t.mcpTokenPlaceholder} autoComplete="off" />
          </label>
          <div className="mcp-token-meta">
            <span className={mcpTokenStatus?.configured ? 'status-pill active' : 'status-pill muted-pill'}>
              {mcpTokenStatus?.configured ? `${t.mcpTokenConfigured}${mcpTokenStatus.hint}` : t.mcpTokenNotConfigured}
            </span>
          </div>
          <div className="actions-row wrap mcp-token-actions">
            <button className="primary-button" type="submit">{t.saveMCPToken}</button>
            <button className="secondary-button" type="button" onClick={() => void handleMCPTokenGenerate()}>
              <MagicWand size={16} weight="bold" aria-hidden="true" />
              <span>{t.generateMCPToken}</span>
            </button>
            <button className="danger-button" type="button" onClick={() => void handleMCPTokenDelete()} disabled={!mcpTokenStatus?.configured}>
              <Trash size={16} weight="bold" aria-hidden="true" />
              <span>{t.deleteMCPToken}</span>
            </button>
            <button className="secondary-button" type="button" onClick={() => setMCPHelpOpen(true)}>
              <Question size={16} weight="bold" aria-hidden="true" />
              <span>{t.mcpHelp}</span>
            </button>
          </div>
        </form>
      </section>

      {mcpHelpOpen ? (
        <div className="settings-overlay mcp-help-overlay" role="presentation" onClick={() => setMCPHelpOpen(false)}>
          <div className="mcp-help-modal card" role="dialog" aria-modal="true" aria-labelledby="mcp-help-title" onClick={(event) => event.stopPropagation()}>
            <div className="settings-drawer-head recorder-drawer-head">
              <div>
                <div className="eyebrow">{t.mcpAccess}</div>
                <h3 id="mcp-help-title">{t.mcpHelpTitle}</h3>
                <p className="muted drawer-subcopy">{t.mcpHelpDescription}</p>
              </div>
              <button className="transport-button" type="button" onClick={() => setMCPHelpOpen(false)} aria-label={t.close}>
                <X size={18} weight="bold" aria-hidden="true" />
              </button>
            </div>
            <div className="mcp-help-content">
              <div>
                <span className="settings-label">{t.mcpEndpoint}</span>
                <code>{mcpEndpoint}</code>
              </div>
              <div>
                <span className="settings-label">{t.mcpAuthorization}</span>
                <code>Authorization: Bearer {mcpTokenExample}</code>
                <code>{mcpEndpoint}?token={encodeURIComponent(mcpTokenExample)}</code>
                <p className="muted">{t.mcpAuthorizationHeader}</p>
              </div>
              <div>
                <span className="settings-label">{t.mcpAvailableTools}</span>
                <ul>
                  <li>{t.mcpListDictionariesHelp}</li>
                  <li>{t.mcpSearchDictionaryHelp}</li>
                </ul>
              </div>
              <p className="warning-banner">{t.mcpHelpTokenNotice}</p>
            </div>
          </div>
        </div>
      ) : null}

      <section className="card manager-utility-card">
        <div className="utility-card-head">
          <div>
            <div className="eyebrow">{t.sharedControls}</div>
            <h3>{t.sharedControls}</h3>
            <p className="muted">{t.sharedControlsDescription}</p>
          </div>
          <span className="utility-card-icon"><GearSix size={22} weight="duotone" /></span>
        </div>
        <SettingsPanel
          preferences={preferences}
          onLanguageChange={onLanguageChange}
          onThemeChange={onThemeChange}
          onDisplayNameChange={onDisplayNameChange}
          onCustomFontSelect={async () => Promise.resolve()}
          onAvatarUpload={onAvatarUpload}
          showProfileEditor={false}
          showFontMode={false}
        />
        <form className="settings-section recent-limit-settings" onSubmit={handleRecentSearchLimitSave}>
          <label className="field compact-profile-field recent-limit-field">
            <span>{t.recentSearchLimit}</span>
            <input
              name="recent_search_limit"
              type="number"
              min={0}
              max={20}
              step={1}
              defaultValue={preferences.recent_search_limit}
              aria-describedby="recent-search-limit-help"
            />
          </label>
          <p id="recent-search-limit-help" className="muted settings-helper-text">{t.recentSearchLimitDescription}</p>
          <div className="actions-row footer-settings-actions">
            <button className="primary-button" type="submit">{t.saveRecentSearchLimit}</button>
          </div>
        </form>
      </section>


      {isAdmin && systemSettings ? (
        <>
        <section className="card manager-utility-card system-access-card">
          <div className="utility-card-head">
            <div>
              <div className="eyebrow">{t.systemAccess}</div>
              <h3>{t.registrationGate}</h3>
              <p className="muted">{t.registrationGateDescription}</p>
            </div>
            <span className={systemSettings.allow_register ? 'status-pill active' : 'status-pill muted-pill'}>
              {systemSettings.allow_register ? t.registrationOpen : t.registrationClosed}
            </span>
          </div>
          <button
            className={systemSettings.allow_register ? 'toggle-chip active registration-toggle-chip' : 'toggle-chip registration-toggle-chip'}
            type="button"
            onClick={() => void handleRegistrationToggle()}
            disabled={systemSettingsSaving}
            aria-pressed={systemSettings.allow_register}
          >
            <span className="toggle-mark"><CheckCircle size={16} weight="fill" /></span>
            <span>{systemSettings.allow_register ? t.registrationOpen : t.registrationClosed}</span>
          </button>
          {systemSettingsError ? <div className="error-banner">{systemSettingsError}</div> : null}
        </section>

        <form key={`${systemSettings.footer_extra}:${systemSettings.copyright}`} className="card manager-utility-card system-footer-card" onSubmit={handleFooterSettingsSave}>
          <div className="utility-card-head">
            <div>
              <div className="eyebrow">{t.siteFooter}</div>
              <h3>{t.footerContent}</h3>
              <p className="muted">{t.footerContentDescription}</p>
            </div>
            <span className="utility-card-icon"><TextAlignLeft size={22} weight="duotone" /></span>
          </div>
          <div className="settings-section footer-settings-grid">
            <label className="field footer-settings-field">
              <span>{t.footerExtraInfo}</span>
              <textarea name="footer_extra" defaultValue={systemSettings.footer_extra} placeholder={t.footerExtraPlaceholder} rows={3} />
            </label>
            <label className="field footer-settings-field">
              <span>{t.footerCopyright}</span>
              <input name="copyright" defaultValue={systemSettings.copyright} placeholder={t.footerCopyrightPlaceholder} />
            </label>
          </div>
          <div className="actions-row footer-settings-actions">
            <button className="primary-button" type="submit" disabled={systemSettingsSaving}>{t.saveFooterSettings}</button>
          </div>
        </form>
        </>
      ) : null}

      <section className="card manager-utility-card font-management-card">
        <div className="utility-card-head">
          <div>
            <div className="eyebrow">{t.fontManagement}</div>
            <h3>{t.fontManagement}</h3>
            <p className="muted">{t.fontManagementDescription}</p>
          </div>
          <span className="utility-card-icon"><UploadSimple size={22} weight="duotone" /></span>
        </div>
        <div className="settings-section font-management-section">
          <span className="settings-label">{t.fontManagement}</span>
          <div className="manager-font-tools">
            <label className="font-upload-inline compact-upload manager-font-upload always-visible-upload">
              <span>{t.customFontUpload}</span>
              <input type="file" accept=".ttf,.otf,.woff,.woff2" onChange={(event) => {
                const file = event.target.files?.[0]
                if (!file) return
                void runManagerAction(() => onFontUpload(file), t.fontUploaded)
                event.target.value = ''
              }} />
            </label>
            {preferences.available_fonts && preferences.available_fonts.length > 0 ? (
              <div className="managed-font-list">
                {preferences.available_fonts.map((font) => (
                  <div key={font.name} className={preferences.custom_font_name === font.name ? 'managed-font-item active' : 'managed-font-item'}>
                    <div className="managed-font-copy">
                      <strong>{font.family}</strong>
                      <span className="muted">{font.name}</span>
                    </div>
                    <button className="secondary-button icon-action-button" type="button" title={t.deleteFont} aria-label={t.deleteFont} onClick={() => void runManagerAction(() => onDeleteFont(font.name), t.fontDeleted)}>
                      <Trash size={16} weight="bold" />
                    </button>
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        </div>
      </section>

      <form className="card upload-card" onSubmit={handleUpload}>
        <div className="upload-grid">
          <label className="dropzone">
            <span className="dropzone-title">{t.mdxFile}</span>
            <span className="muted">{t.mdxFileHint}</span>
            <input type="file" accept=".mdx" onChange={(event) => setMdxFile(event.target.files?.[0] ?? null)} required />
            <strong>{mdxFile ? mdxFile.name : t.chooseMdx}</strong>
          </label>

          <label className="dropzone">
            <span className="dropzone-title">{t.mddResources}</span>
            <span className="muted">{t.mddHint}</span>
            <input type="file" accept=".mdd" multiple onChange={(event) => setMddFiles(Array.from(event.target.files ?? []))} />
            <strong>{mddFiles.length > 0 ? t.selectedFileCount(mddFiles.length) : t.chooseMdd}</strong>
          </label>
        </div>

        {uploadError ? <div className="error-banner">{uploadError}</div> : null}
        {error ? <div className="error-banner">{error}</div> : null}

        <div className="actions-row">
          <button className="primary-button" type="submit" disabled={uploading}>
            {uploading ? t.uploading : t.uploadDictionary}
          </button>
        </div>
      </form>

      {maintenanceReport ? (
        <section className="card maintenance-report-card">
          <div className="result-group-header">
            <div>
              <div className="eyebrow">{t.maintenanceReportTitle}</div>
              <h3>{maintenanceReport.summary}</h3>
            </div>
          </div>
          <div className="maintenance-stats">
            <span>{t.discoveredCount}: {maintenanceReport.discovered}</span>
            <span>{t.updatedCount}: {maintenanceReport.updated}</span>
            <span>{t.skippedCount}: {maintenanceReport.skipped}</span>
            <span>{t.failedCount}: {maintenanceReport.failed}</span>
          </div>
          <div className="maintenance-item-list">
            {maintenanceReport.items.map((item, index) => (
              <article
                key={`${item.name}-${item.status}-${index}`}
                className={item.dictionary_id ? 'maintenance-item actionable-maintenance-item' : 'maintenance-item'}
                onClick={() => {
                  if (!item.dictionary_id) return
                  const target = document.getElementById(`dictionary-card-${item.dictionary_id}`)
                  target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
                }}
              >
                <strong>{item.name}</strong>
                <p className="muted">{item.message}</p>
              </article>
            ))}
          </div>
        </section>
      ) : null}

      <div className="dictionary-grid">
        {dictionaries.map((item) => {
          const mddPaths = item.mdd_paths ?? []
          const missingFiles = item.missing_files ?? []
          return (
            <article key={item.id} id={`dictionary-card-${item.id}`} className="card dictionary-card">
              <div className="dictionary-card-head">
                <div>
                  <h3>{item.title || item.name}</h3>
                  <p className="muted dictionary-description">{summarizeDescription(item.description, t.noDescription)}</p>
                </div>
              </div>

              <div className="dictionary-status-row device-status-row">
                <label className={item.enabled ? 'toggle-chip active' : 'toggle-chip'}>
                  <input type="checkbox" checked={item.enabled} onChange={() => void runManagerAction(() => onToggle(item), t.dictionaryStatusUpdated)} />
                  <span className="toggle-mark"><CheckCircle size={16} weight="fill" /></span>
                  <span>{item.enabled ? t.enabled : t.disabled}</span>
                </label>
                <label className={item.public ? 'toggle-chip info' : 'toggle-chip'}>
                  <input type="checkbox" checked={item.public} onChange={() => void runManagerAction(() => onTogglePublic(item), t.dictionaryVisibilityUpdated)} />
                  <span className="toggle-mark">{item.public ? <Eye size={16} weight="fill" /> : <EyeSlash size={16} weight="fill" />}</span>
                  <span>{item.public ? t.public : t.private}</span>
                </label>
                <span className={statusClassName(item.file_status)}>{statusLabel(item.file_status, t)}</span>
              </div>

              {missingFiles.length > 0 ? (
                <div className="warning-banner">
                  <strong>{t.missingFiles}</strong>
                  <ul className="missing-file-list">
                    {missingFiles.map((path) => (
                      <li key={path}>{path}</li>
                    ))}
                  </ul>
                </div>
              ) : null}

              <dl className="meta-grid">
                <div>
                  <dt>{t.entries}</dt>
                  <dd>{item.entry_count}</dd>
                </div>
                <div>
                  <dt>{t.mddFiles}</dt>
                  <dd>{mddPaths.length}</dd>
                </div>
                <div>
                  <dt>{t.uploadedAt}</dt>
                  <dd>{new Date(item.created_at).toLocaleString()}</dd>
                </div>
                <div>
                  <dt>{t.owner}</dt>
                  <dd>{item.owner_name || t.you}</dd>
                </div>
              </dl>

              <div className="actions-row wrap icon-action-row">
                <button className="secondary-button icon-action-button" type="button" title={t.refreshItem} aria-label={t.refreshItem} onClick={() => void runManagerAction(() => onRefreshItem(item), t.dictionaryRefreshed)}>
                  <PencilSimple size={18} weight="bold" />
                </button>
                <button className="secondary-button icon-action-button" type="button" title={t.uploadDictionary} aria-label={t.uploadDictionary} onClick={() => document.querySelector('form.upload-card input[type=file]')?.dispatchEvent(new MouseEvent('click'))}>
                  <UploadSimple size={18} weight="bold" />
                </button>
                <button className="danger-button icon-action-button" type="button" title={t.delete} aria-label={t.delete} onClick={() => void runManagerAction(() => onDelete(item), t.dictionaryDeleted)}>
                  <Trash size={18} weight="bold" />
                </button>
              </div>
            </article>
          )
        })}

        {dictionaries.length === 0 ? (
          <div className="card empty-state">
            <h3>{t.noDictionariesYet}</h3>
            <p className="muted">{t.uploadFirstDictionary}</p>
          </div>
        ) : null}
      </div>
    </section>
  )
}

function summarizeDescription(description: string, fallback: string) {
  const plain = description
    ?.replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

  if (!plain) return fallback
  if (plain.length <= 180) return plain
  return `${plain.slice(0, 180).trim()}…`
}

function statusClassName(status: DictionarySummary['file_status']) {
  switch (status) {
    case 'missing_mdx':
    case 'missing_all':
      return 'status-pill danger-pill'
    case 'missing_mdd':
      return 'status-pill warning-pill'
    default:
      return 'status-pill active'
  }
}

function statusLabel(status: DictionarySummary['file_status'], t: ReturnType<typeof useI18n>['t']) {
  switch (status) {
    case 'missing_mdx':
      return t.fileStatusMissingMdx
    case 'missing_mdd':
      return t.fileStatusMissingMdd
    case 'missing_all':
      return t.fileStatusMissingAll
    default:
      return t.fileStatusOk
  }
}
