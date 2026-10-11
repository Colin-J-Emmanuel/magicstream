import { Link, Navigate, useLocation } from 'react-router'
import { useAuth } from '../auth/useAuth'
import { RegisterForm } from '../components/RegisterForm'
import { returnPath } from '../lib/redirect'

export function RegisterPage() {
  const { status, user } = useAuth()
  const location = useLocation()

  if (status === 'loading') return null
  if (user) return <Navigate to={returnPath(location.state)} replace />

  return (
    <section className="auth">
      <h1 className="page-title">Create an account</h1>
      <p className="lede">Pick the genres you like. Your recommendations come from them.</p>
      <RegisterForm />
      <p className="switch">
        Already have an account?{' '}
        <Link to="/login" state={location.state}>
          Log in
        </Link>
      </p>
    </section>
  )
}