import type { AuthResponse, DictionarySummary, HealthInfo, MaintenanceReport, SearchResult, SearchSuggestion, SharedFont, SystemSettings, MCPTokenStatus, UserPreferences, UserSummary } from '../types'

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api'

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(init.headers)
  if (!headers.has('Content-Type') && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'same-origin',
  })

  if (!response.ok) {
    let message = response.statusText
    try {
      const body = (await response.json()) as { message?: string }
      if (body.message) {
        message = body.message
      }
    } catch {
      // keep status text
    }
    throw new ApiError(message, response.status)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export const api = {
  health() {
    return request<HealthInfo>('/health', { method: 'GET' })
  },

  listPublicDictionaries() {
    return request<DictionarySummary[]>('/public/dictionaries', { method: 'GET' })
  },

  listPublicFonts() {
    return request<SharedFont[]>('/public/fonts', { method: 'GET' })
  },

  publicSearch(query: string, dictionaryId?: number) {
    const params = new URLSearchParams({ q: query })
    if (dictionaryId) {
      params.set('dict', String(dictionaryId))
    }
    return request<SearchResult[]>(`/public/search?${params.toString()}`, { method: 'GET' })
  },

  publicSuggest(query: string, dictionaryId?: number) {
    const params = new URLSearchParams({ q: query })
    if (dictionaryId) {
      params.set('dict', String(dictionaryId))
    }
    return request<SearchSuggestion[]>(`/public/suggest?${params.toString()}`, { method: 'GET' })
  },

  register(username: string, password: string) {
    return request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
  },

  login(username: string, password: string) {
    return request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
  },

  logout() {
    return request<void>('/auth/logout', {
      method: 'POST',
    })
  },

  me(token: string) {
    return request<UserSummary>('/me', { method: 'GET' }, token)
  },

  getPreferences(token: string) {
    return request<UserPreferences>('/preferences', { method: 'GET' }, token)
  },

  getSystemSettings(token: string) {
    return request<SystemSettings>('/settings/system', { method: 'GET' }, token)
  },

  updateSystemSettings(token: string, settings: SystemSettings) {
    return request<SystemSettings>('/settings/system', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }, token)
  },


  getMCPToken(token: string) {
    return request<MCPTokenStatus>('/mcp/token', { method: 'GET' }, token)
  },

  setMCPToken(token: string, mcpToken: string) {
    return request<MCPTokenStatus>('/mcp/token', {
      method: 'PUT',
      body: JSON.stringify({ token: mcpToken }),
    }, token)
  },

  generateMCPToken(token: string) {
    return request<MCPTokenStatus>('/mcp/token/generate', { method: 'POST' }, token)
  },

  deleteMCPToken(token: string) {
    return request<MCPTokenStatus>('/mcp/token', { method: 'DELETE' }, token)
  },

  updatePreferences(token: string, preferences: Pick<UserPreferences, 'language' | 'theme' | 'font_mode' | 'display_name' | 'custom_font_name' | 'recent_search_limit'>) {
    return request<UserPreferences>('/preferences', {
      method: 'PUT',
      body: JSON.stringify(preferences),
    }, token)
  },

  uploadFont(token: string, fontFile: File) {
    const formData = new FormData()
    formData.append('font', fontFile)
    return request<UserPreferences>('/preferences/font', {
      method: 'POST',
      body: formData,
    }, token)
  },

  deleteFont(token: string, name: string) {
    return request<UserPreferences>(`/preferences/font/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }, token)
  },

  uploadAvatar(token: string, avatarFile: File) {
    const formData = new FormData()
    formData.append('avatar', avatarFile)
    return request<UserPreferences>('/preferences/avatar', {
      method: 'POST',
      body: formData,
    }, token)
  },

  listDictionaries(token: string) {
    return request<DictionarySummary[]>('/dictionaries', { method: 'GET' }, token)
  },

  uploadDictionary(token: string, mdxFile: File, mddFiles: File[]) {
    const formData = new FormData()
    formData.append('mdx', mdxFile)
    for (const file of mddFiles) {
      formData.append('mdd', file)
    }
    return request<DictionarySummary>('/dictionaries/upload', {
      method: 'POST',
      body: formData,
    }, token)
  },

  toggleDictionary(token: string, id: number, enabled: boolean) {
    return request<DictionarySummary>(`/dictionaries/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    }, token)
  },

  setDictionaryPublic(token: string, id: number, isPublic: boolean) {
    return request<DictionarySummary>(`/dictionaries/${id}/public`, {
      method: 'PATCH',
      body: JSON.stringify({ public: isPublic }),
    }, token)
  },

  refreshDictionary(token: string, id: number) {
    return request<MaintenanceReport>(`/dictionaries/${id}/refresh`, {
      method: 'POST',
    }, token)
  },

  refreshLibrary(token: string) {
    return request<MaintenanceReport>('/dictionaries/refresh', {
      method: 'POST',
    }, token)
  },

  deleteDictionary(token: string, id: number) {
    return request<void>(`/dictionaries/${id}`, { method: 'DELETE' }, token)
  },

  search(token: string, query: string, dictionaryId?: number) {
    const params = new URLSearchParams({ q: query })
    if (dictionaryId) {
      params.set('dict', String(dictionaryId))
    }
    return request<SearchResult[]>(`/search?${params.toString()}`, { method: 'GET' }, token)
  },

  suggest(token: string, query: string, dictionaryId?: number) {
    const params = new URLSearchParams({ q: query })
    if (dictionaryId) {
      params.set('dict', String(dictionaryId))
    }
    return request<SearchSuggestion[]>(`/suggest?${params.toString()}`, { method: 'GET' }, token)
  },
}
