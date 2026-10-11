import { Link, Navigate } from 'react-router'
import { useAuth } from '../auth/useAuth'
import { LoginForm } from '../components/LoginForm'

export function LoginPage() {
  const { status, user } = useAuth()

  if (status === 'loading') return null
  if (user) return <Navigate to="/" replace />

  return (
    <section className="auth">
      <h1 className="page-title">Log in</h1>
      <LoginForm />
      <p className="switch">
        New here? <Link to="/register">Create an account</Link>
      </p>
    </section>
  )
}