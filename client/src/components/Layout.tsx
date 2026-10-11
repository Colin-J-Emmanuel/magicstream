import { Link, NavLink, Outlet, useNavigate } from 'react-router'
import { useAuth } from '../auth/useAuth'

export function Layout() {
  const { status, user, logout } = useAuth()
  const navigate = useNavigate()

  async function handleLogout() {
    await logout()
    navigate('/')
  }

  return (
    <>
      <header className="site-header">
        <Link to="/" className="wordmark">
          MagicStream
        </Link>
        <nav className="site-nav" aria-label="Main">
          <NavLink to="/" end>
            Movies
          </NavLink>
          {status === 'authenticated' && user && (
            <>
              <span className="user-name">Signed in as {user.first_name}</span>
              <button type="button" className="button-quiet" onClick={() => void handleLogout()}>
                Log out
              </button>
            </>
          )}
          {status === 'anonymous' && (
            <>
              <NavLink to="/login">Log in</NavLink>
              <NavLink to="/register">Create account</NavLink>
            </>
          )}
        </nav>
      </header>
      <main>
        <Outlet />
      </main>
    </>
  )
}