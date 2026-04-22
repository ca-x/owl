import { useMemo, useState } from 'react'

import { useI18n } from '../i18n'

interface AuthPanelProps {
  loading: boolean
  error: string
  onLogin: (username: string, password: string) => Promise<void>
  onRegister: (username: string, password: string) => Promise<void>
}

export function AuthPanel({ loading, error, onLogin, onRegister }: AuthPanelProps) {
  const { t } = useI18n()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const title = useMemo(() => (mode === 'login' ? t.welcomeBack : t.createAccountTitle), [mode, t])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (mode === 'login') {
      await onLogin(username, password)
      return
    }
    await onRegister(username, password)
  }

  return (
    <form className="auth-card card auth-modal-card" onSubmit={handleSubmit}>
      <div className="auth-tabs">
        <button className={mode === 'login' ? 'active' : ''} type="button" onClick={() => setMode('login')}>
          {t.login}
        </button>
        <button className={mode === 'register' ? 'active' : ''} type="button" onClick={() => setMode('register')}>
          {t.register}
        </button>
      </div>

      <div>
        <h2>{title}</h2>
        <p className="muted">{t.authDescription}</p>
      </div>

      <label className="field">
        <span>{t.username}</span>
        <input value={username} onChange={(event) => setUsername(event.target.value)} placeholder={t.usernamePlaceholder} required />
      </label>

      <label className="field">
        <span>{t.password}</span>
        <input
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder={t.passwordPlaceholder}
          minLength={6}
          required
        />
      </label>

      {error ? <div className="error-banner">{error}</div> : null}

      <button className="primary-button" type="submit" disabled={loading}>
        {loading ? t.pleaseWait : mode === 'login' ? t.signIn : t.createAccount}
      </button>
    </form>
  )
}
