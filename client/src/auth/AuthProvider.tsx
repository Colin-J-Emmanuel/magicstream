import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, setSessionExpiredHandler } from '../api/client'
import type { User } from '../api/types'
import { AuthContext, type AuthState, type RegisterInput } from './context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [status, setStatus] = useState<AuthState['status']>('loading')

  useEffect(() => {
    // If a refresh ever fails, the session is over: show the signed-out view.
    setSessionExpiredHandler(() => {
      setUser(null)
      setStatus('anonymous')
    })

    // The cookies are http-only, so ask the server who we are.
    const controller = new AbortController()
    api<User>('/me', { signal: controller.signal })
      .then((u) => {
        setUser(u)
        setStatus('authenticated')
      })
      .catch((err) => {
        if (err.name === 'AbortError') return
        setUser(null)
        setStatus('anonymous')
      })

    return () => {
      controller.abort()
      setSessionExpiredHandler(null)
    }
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const u = await api<User>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
    setUser(u)
    setStatus('authenticated')
  }, [])

  const register = useCallback(
    async (input: RegisterInput) => {
      await api<User>('/auth/register', { method: 'POST', body: JSON.stringify(input) })
      await login(input.email, input.password) // registration doesn't start a session; login does
    },
    [login],
  )

  const logout = useCallback(async () => {
    try {
      await api<void>('/auth/logout', { method: 'POST' })
    } finally {
      // Clear local state even if the request failed: the user asked to be signed out.
      setUser(null)
      setStatus('anonymous')
    }
  }, [])

  const value = useMemo(
    () => ({ user, status, login, register, logout }),
    [user, status, login, register, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}