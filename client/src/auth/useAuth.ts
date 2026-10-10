import { useContext } from 'react'
import { AuthContext } from './context'

// useAuth gives any component the current user and the auth actions.
export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used inside <AuthProvider>')
  }
  return ctx
}