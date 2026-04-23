import { PencilSimple, X } from '@phosphor-icons/react'
import { useState } from 'react'

import { useI18n } from '../i18n'
import type { UserPreferences } from '../types'

interface SettingsPanelProps {
  preferences: UserPreferences
  onLanguageChange: (language: UserPreferences['language']) => Promise<void>
  onThemeChange: (theme: UserPreferences['theme']) => Promise<void>
  onFontModeChange?: (fontMode: UserPreferences['font_mode']) => Promise<void>
  onDisplayNameChange: (displayName: string) => Promise<void>
  onCustomFontSelect?: (fontName: string) => Promise<void>
  onAvatarUpload: (file: File) => Promise<void>
  showProfileEditor?: boolean
  showFontMode?: boolean
}

export function SettingsPanel({
  preferences,
  onLanguageChange,
  onThemeChange,
  onFontModeChange,
  onDisplayNameChange,
  onCustomFontSelect,
  onAvatarUpload,
  showProfileEditor = true,
  showFontMode = true,
}: SettingsPanelProps) {
  const { t } = useI18n()
  const [avatarUploading, setAvatarUploading] = useState(false)
  const [displayName, setDisplayName] = useState(preferences.display_name || '')
  const [selectedAvatar, setSelectedAvatar] = useState<File | null>(null)
  const [profileEditorOpen, setProfileEditorOpen] = useState(false)

  async function handleAvatarSelection(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    setSelectedAvatar(file)
    event.target.value = ''
  }

  async function handleSaveProfile() {
    const normalized = displayName.trim()
    if (normalized && normalized !== preferences.display_name) {
      await onDisplayNameChange(normalized)
    }
    if (selectedAvatar) {
      setAvatarUploading(true)
      try {
        await onAvatarUpload(selectedAvatar)
        setSelectedAvatar(null)
      } finally {
        setAvatarUploading(false)
      }
    }
  }

  return (
    <div className="settings-panel">
      {showProfileEditor ? (
        <div className="settings-section profile-settings-section">
          <div className="profile-settings-header">
            <span className="settings-label">{t.profileSettings}</span>
            <button
              className={profileEditorOpen ? 'secondary-button icon-action-button profile-toggle-button active' : 'secondary-button icon-action-button profile-toggle-button'}
              type="button"
              onClick={() => setProfileEditorOpen((current) => !current)}
              title={profileEditorOpen ? t.hideProfileEditor : t.showProfileEditor}
              aria-label={profileEditorOpen ? t.hideProfileEditor : t.showProfileEditor}
            >
              {profileEditorOpen ? <X size={16} weight="bold" /> : <PencilSimple size={16} weight="bold" />}
            </button>
          </div>
          {profileEditorOpen ? (
            <div className="profile-settings-grid stacked-profile-grid">
              <label className="field compact-profile-field">
                <span>{t.displayName}</span>
                <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder={t.displayNamePlaceholder} />
              </label>
              <div className="avatar-control-block">
                <div className="settings-avatar-preview">
                  {preferences.avatar_url ? (
                    <img src={preferences.avatar_url} alt={displayName || t.displayName} className="settings-avatar-image" />
                  ) : (
                    <div className="settings-avatar-fallback">{(displayName || 'U').slice(0, 1).toUpperCase()}</div>
                  )}
                </div>
                <label className="font-upload-inline compact-upload avatar-upload-inline">
                  <span>{selectedAvatar ? `${t.selectedAvatar}: ${selectedAvatar.name}` : preferences.avatar_url ? t.updateAvatar : t.uploadAvatar}</span>
                  <input type="file" accept=".png,.jpg,.jpeg,.webp" onChange={handleAvatarSelection} disabled={avatarUploading} />
                </label>
              </div>
              <div className="profile-save-row">
                <button className="primary-button" type="button" onClick={() => void handleSaveProfile()} disabled={avatarUploading}>
                  {avatarUploading ? t.pleaseWait : t.saveProfile}
                </button>
              </div>
            </div>
          ) : null}
        </div>
      ) : null}

      <div className="settings-section">
        <span className="settings-label">{t.language}</span>
        <div className="settings-chip-row">
          <button className={preferences.language === 'zh-CN' ? 'filter-chip active' : 'filter-chip'} type="button" onClick={() => void onLanguageChange('zh-CN')}>
            简体中文
          </button>
          <button className={preferences.language === 'en' ? 'filter-chip active' : 'filter-chip'} type="button" onClick={() => void onLanguageChange('en')}>
            English
          </button>
        </div>
      </div>

      <div className="settings-section">
        <span className="settings-label">{t.theme}</span>
        <div className="settings-chip-row">
          {(['system', 'paper', 'blue', 'green', 'dark', 'mono'] as const).map((theme) => (
            <button
              key={theme}
              className={preferences.theme === theme ? 'filter-chip active' : 'filter-chip'}
              type="button"
              onClick={() => void onThemeChange(theme)}
            >
              {theme === 'mono' ? t.mono_theme : t[theme]}
            </button>
          ))}
        </div>
      </div>

      {showFontMode ? (
        <div className="settings-section">
          <span className="settings-label">{t.readingFont}</span>
          <div className="settings-chip-row wrap">
            {(['sans', 'serif', 'mono'] as const).map((fontMode) => (
              <button
                key={fontMode}
                className={preferences.font_mode === fontMode ? 'filter-chip active' : 'filter-chip'}
                type="button"
                onClick={() => void onFontModeChange?.(fontMode)}
              >
                {t[fontMode]}
              </button>
            ))}
            {preferences.available_fonts && preferences.available_fonts.length > 0 ? (
              <button
                className={preferences.font_mode === 'custom' ? 'filter-chip active' : 'filter-chip'}
                type="button"
                onClick={() => void onFontModeChange?.('custom')}
              >
                {preferences.custom_font_name ? `${t.custom} · ${preferences.custom_font_name}` : t.custom}
              </button>
            ) : null}
          </div>
          {preferences.font_mode === 'custom' && preferences.available_fonts && preferences.available_fonts.length > 0 ? (
            <div className="font-picker-list" role="listbox" aria-label={t.fontManagement}>
              {preferences.available_fonts.map((font) => (
                <button
                  key={font.name}
                  className={preferences.custom_font_name === font.name ? 'font-picker-item active' : 'font-picker-item'}
                  type="button"
                  onClick={() => void onCustomFontSelect?.(font.name)}
                  onMouseDown={(event) => {
                    event.preventDefault()
                  }}
                >
                  {font.family}
                </button>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
