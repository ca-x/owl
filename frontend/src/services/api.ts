import type { AuthResponse, DictionarySummary, SearchResult, UserSummary } from '../types'

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

  me(token: string) {
    return request<UserSummary>('/me', { method: 'GET' }, token)
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
}
