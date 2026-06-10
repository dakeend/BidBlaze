import { useState } from 'react'
import { LogIn, User } from 'lucide-react'
import { login } from '../lib/auth'

type LoginPageProps = {
  onLogin: () => void
}

export function LoginPage({ onLogin }: LoginPageProps) {
  const [nickname, setNickname] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = nickname.trim()
    if (!trimmed) {
      setError('请输入昵称')
      return
    }
    setLoading(true)
    setError(null)
    try {
      await login(trimmed)
      onLogin()
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败，请重试')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">🔨 直播竞拍</h1>
        <p className="login-subtitle">输入昵称进入竞拍间</p>
        <form onSubmit={handleSubmit} className="login-form">
          <label className="login-field">
            <User size={18} aria-hidden="true" />
            <input
              type="text"
              placeholder="例如：买家张三"
              maxLength={32}
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              autoFocus
              disabled={loading}
            />
          </label>
          {error && <p className="login-error">{error}</p>}
          <button type="submit" className="login-button" disabled={loading}>
            <LogIn size={18} aria-hidden="true" />
            <span>{loading ? '登录中...' : '进入竞拍间'}</span>
          </button>
        </form>
      </div>
    </div>
  )
}
