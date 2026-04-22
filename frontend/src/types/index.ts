export interface UserSummary {
  id: number
  username: string
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
  word: string
  html: string
  score: number
  source: string
}
