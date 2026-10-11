import { Link, Navigate, useLocation } from 'react-router'
import { useAuth } from '../auth/useAuth'
import { LoginForm } from '../components/LoginForm'
import { returnPath } from '../lib/redirect'

export function LoginPage() {
  const { status, user } = useAuth()
  const location = useLocation()

  if (status === 'loading') return null
  if (user) return <Navigate to={returnPath(location.state)} replace />

  return (
    <section className="auth">
      <h1 className="page-title">Log in</h1>
      <LoginForm />
      <p className="switch">
        New here?{' '}
        <Link to="/register" state={location.state}>
          Create an account
        </Link>
      </p>
    </section>
  )
}