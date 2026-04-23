export interface UserSummary {
  id: number
  username: string
  display_name: string
  avatar_url?: string
  is_admin: boolean
}

export interface AuthResponse {
  token: string
  user: UserSummary
}

export interface DictionarySummary {
  id: number
  name: string
  title: string
  description: string
  entry_count: number
  enabled: boolean
  public: boolean
  file_status: 'ok' | 'missing_mdx' | 'missing_mdd' | 'missing_all'
  missing_files: string[]
  mdx_path: string
  mdd_paths: string[]
  created_at: string
  updated_at: string
  owner_id: number
  owner_name?: string
}

export interface SearchResult {
  dictionary_id: number
  dictionary_name: string
  visibility: 'public' | 'private'
  word: string
  html: string
  score: number
  source: string
}

export interface SearchSuggestionSource {
  dictionary_id: number
  dictionary_name: string
  visibility: 'public' | 'private'
  source: string
}

export interface SearchSuggestion {
  word: string
  sources: SearchSuggestionSource[]
}

export interface SharedFont {
  name: string
  family: string
}

export interface HealthInfo {
  status: string
  version: string
  full_version: string
  commit: string
  build_time: string
  go_version: string
  os: string
  arch: string
  allow_register: boolean
}

export interface UserPreferences {
  language: 'zh-CN' | 'en'
  theme: 'paper' | 'blue' | 'green' | 'dark' | 'mono' | 'system'
  font_mode: 'sans' | 'serif' | 'mono' | 'custom'
  display_name: string
  avatar_url?: string
  custom_font_name: string
  custom_font_family: string
  custom_font_url?: string
  available_fonts?: SharedFont[]
}

export interface MaintenanceItemReport {
  dictionary_id?: number
  name: string
  action: string
  status: string
  message: string
  dictionary?: DictionarySummary
}

export interface MaintenanceReport {
  summary: string
  discovered: number
  updated: number
  skipped: number
  failed: number
  items: MaintenanceItemReport[]
}
