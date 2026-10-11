import { useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api/client'
import type { Genre, Movie } from '../api/types'
import { useAuth } from '../auth/useAuth'

export function RegisterForm() {
  const { register } = useAuth()
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [genres, setGenres] = useState<Genre[]>([])
  const [selected, setSelected] = useState<number[]>([])
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // Offer the genres that actually exist in the catalog, rather than a hardcoded copy.
  useEffect(() => {
    const controller = new AbortController()
    api<Movie[]>('/movies', { signal: controller.signal })
      .then((movies) => {
        const byId = new Map<number, Genre>()
        for (const m of movies) for (const g of m.genre) byId.set(g.genre_id, g)
        setGenres([...byId.values()].sort((a, b) => a.genre_name.localeCompare(b.genre_name)))
      })
      .catch((err) => {
        if (err.name !== 'AbortError') setError(errorMessage(err))
      })
    return () => controller.abort()
  }, [])

  function toggleGenre(id: number) {
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (selected.length === 0) {
      setError('Pick at least one favorite genre.')
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      await register({
        first_name: firstName,
        last_name: lastName,
        email,
        password,
        favorite_genres: genres.filter((g) => selected.includes(g.genre_id)),
      })
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="form" onSubmit={handleSubmit}>
      <label className="field">
        First name
        <input required maxLength={50} autoComplete="given-name" value={firstName} onChange={(e) => setFirstName(e.target.value)} />
      </label>
      <label className="field">
        Last name
        <input required maxLength={50} autoComplete="family-name" value={lastName} onChange={(e) => setLastName(e.target.value)} />
      </label>
      <label className="field">
        Email
        <input type="email" required autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
      </label>
      <label className="field">
        Password, at least 8 characters
        <input type="password" required minLength={8} maxLength={72} autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />
      </label>
      <fieldset className="genres">
        <legend>Favorite genres</legend>
        {genres.map((g) => (
          <label key={g.genre_id} className="check">
            <input type="checkbox" checked={selected.includes(g.genre_id)} onChange={() => toggleGenre(g.genre_id)} />
            {g.genre_name}
          </label>
        ))}
      </fieldset>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <button type="submit" className="button" disabled={submitting}>
        {submitting ? 'Creating account…' : 'Create account'}
      </button>
    </form>
  )
}