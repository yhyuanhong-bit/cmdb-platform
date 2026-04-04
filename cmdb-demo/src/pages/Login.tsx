import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../stores/authStore'

export default function Login() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const success = await login(username, password)
    setLoading(false)

    if (success) {
      navigate('/locations')
    } else {
      setError('Login failed — check username/password, or verify the backend is running at ' + (import.meta.env.VITE_API_URL || 'http://localhost:8080'))
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface">
      <div className="w-full max-w-md p-8 bg-surface-container rounded-2xl shadow-lg">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold text-on-surface">Stitch CMDB</h1>
          <p className="text-sm text-on-surface-variant mt-2">Configuration Management Database</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="p-3 rounded-lg bg-error-container text-on-error-container text-sm">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-on-surface mb-1">
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg border border-outline-variant bg-surface
                         text-on-surface focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="Enter username"
              required
              autoFocus
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-on-surface mb-1">
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg border border-outline-variant bg-surface
                         text-on-surface focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="Enter password"
              required
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 rounded-lg bg-primary text-on-primary font-medium
                       hover:bg-primary/90 disabled:opacity-50 transition-colors"
          >
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>

        <p className="text-xs text-on-surface-variant text-center mt-6">
          Local: admin / admin123
        </p>
        <p className="text-xs text-on-surface-variant text-center mt-1">
          AD: username@domain (e.g. john@tw.company.com)
        </p>
      </div>
    </div>
  )
}
