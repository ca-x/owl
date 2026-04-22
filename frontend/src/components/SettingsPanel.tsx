import { useState } from 'react'

import { useI18n } from '../i18n'
import type { UserPreferences } from '../types'

interface SettingsPanelProps {
  preferences: UserPreferences
  onLanguageChange: (language: UserPreferences['language']) => Promise<void>
  onThemeChange: (theme: UserPreferences['theme']) => Promise<void>
  onFontModeChange: (fontMode: UserPreferences['font_mode']) => Promise<void>
  onFontUpload: (file: File) => Promise<void>
}

export function SettingsPanel({
  preferences,
  onLanguageChange,
  onThemeChange,
  onFontModeChange,
  onFontUpload,
}: SettingsPanelProps) {
  const { t } = useI18n()
  const [uploading, setUploading] = useState(false)

  async function handleFontUpload(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    setUploading(true)
    try {
      await onFontUpload(file)
    } finally {
      setUploading(false)
      event.target.value = ''
    }
  }

  return (
    <div className="settings-panel card">
      <div className="settings-section">
        <span className="settings-label">{t.language}</span>
        <div className="settings-chip-row">
          <button
            className={preferences.language === 'zh-CN' ? 'filter-chip active' : 'filter-chip'}
            type="button"
            onClick={() => void onLanguageChange('zh-CN')}
          >
            简体中文
          </button>
          <button
            className={preferences.language === 'en' ? 'filter-chip active' : 'filter-chip'}
            type="button"
            onClick={() => void onLanguageChange('en')}
          >
            English
          </button>
        </div>
      </div>

      <div className="settings-section">
        <span className="settings-label">{t.theme}</span>
        <div className="settings-chip-row">
          {(['system', 'light', 'dark', 'sepia'] as const).map((theme) => (
            <button
              key={theme}
              className={preferences.theme === theme ? 'filter-chip active' : 'filter-chip'}
              type="button"
              onClick={() => void onThemeChange(theme)}
            >
              {t[theme]}
            </button>
          ))}
        </div>
      </div>

      <div className="settings-section">
        <span className="settings-label">{t.readingFont}</span>
        <div className="settings-chip-row wrap">
          {(['sans', 'serif', 'mono'] as const).map((fontMode) => (
            <button
              key={fontMode}
              className={preferences.font_mode === fontMode ? 'filter-chip active' : 'filter-chip'}
              type="button"
              onClick={() => void onFontModeChange(fontMode)}
            >
              {t[fontMode]}
            </button>
          ))}
          <button
            className={preferences.font_mode === 'custom' ? 'filter-chip active' : 'filter-chip'}
            type="button"
            onClick={() => preferences.custom_font_name && void onFontModeChange('custom')}
            disabled={!preferences.custom_font_name}
          >
            {t.custom}
          </button>
        </div>
        <label className="font-upload-inline">
          <span>{preferences.custom_font_name ? `${t.customFontPrefix}${preferences.custom_font_name}` : t.customFontUpload}</span>
          <input type="file" accept=".ttf,.otf,.woff,.woff2" onChange={handleFontUpload} disabled={uploading} />
        </label>
      </div>
    </div>
  )
}
