import { useMemo, useState } from 'react'

interface AuthPanelProps {
  loading: boolean
  error: string
  onLogin: (username: string, password: string) => Promise<void>
  onRegister: (username: string, password: string) => Promise<void>
}

export function AuthPanel({ loading, error, onLogin, onRegister }: AuthPanelProps) {
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const title = useMemo(() => (mode === 'login' ? 'Welcome back' : 'Create your Owl account'), [mode])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (mode === 'login') {
      await onLogin(username, password)
      return
    }
    await onRegister(username, password)
  }

  return (
    <div className="auth-shell">
      <div className="auth-copy card">
        <div className="eyebrow">Owl Dictionary</div>
        <h1>Personal MDX / MDD dictionary search, in the browser.</h1>
        <p>
          Upload your own dictionaries, keep them isolated per account, and search HTML entries with images,
          audio, and other bundled resources.
        </p>
        <ul className="feature-list">
          <li>Fast fuzzy search across enabled dictionaries</li>
          <li>Private dictionary library per user</li>
          <li>Modern responsive UI with dark mode</li>
        </ul>
      </div>

      <form className="auth-card card" onSubmit={handleSubmit}>
        <div className="auth-tabs">
          <button className={mode === 'login' ? 'active' : ''} type="button" onClick={() => setMode('login')}>
            Login
          </button>
          <button className={mode === 'register' ? 'active' : ''} type="button" onClick={() => setMode('register')}>
            Register
          </button>
        </div>

        <div>
          <h2>{title}</h2>
          <p className="muted">Use a username and password. JWT auth is handled by the backend.</p>
        </div>

        <label className="field">
          <span>Username</span>
          <input value={username} onChange={(event) => setUsername(event.target.value)} placeholder="owl-user" required />
        </label>

        <label className="field">
          <span>Password</span>
          <input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            placeholder="••••••••"
            minLength={6}
            required
          />
        </label>

        {error ? <div className="error-banner">{error}</div> : null}

        <button className="primary-button" type="submit" disabled={loading}>
          {loading ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
        </button>
      </form>
    </div>
  )
}
