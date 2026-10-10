import { useState } from 'react'
import { useAuth } from './auth/useAuth'
import { LoginForm } from './components/LoginForm'
import { MovieList } from './components/MovieList'
import { RegisterForm } from './components/RegisterForm'

export default function App() {
  const { status, user, logout } = useAuth()
  const [mode, setMode] = useState<'login' | 'register'>('login')

  if (status === 'loading') return <p>Loading…</p>

  if (!user) {
    return (
      <main>
        <h1>MagicStream</h1>
        {mode === 'login' ? <LoginForm /> : <RegisterForm />}
        <button type="button" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
          {mode === 'login' ? 'Need an account? Register' : 'Have an account? Log in'}
        </button>
      </main>
    )
  }

  return (
    <main>
      <header>
        <h1>MagicStream</h1>
        <p>
          Signed in as {user.first_name} ({user.role})
        </p>
        <button type="button" onClick={() => void logout()}>
          Log out
        </button>
      </header>
      <MovieList />
    </main>
  )
}