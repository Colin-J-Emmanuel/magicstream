import { useEffect, useState } from 'react'
import { api, ApiError } from './api/client'
import type { Movie } from './api/types'

export default function App() {
  const [movies, setMovies] = useState<Movie[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    api<Movie[]>('/movies', { signal: controller.signal })
      .then(setMovies)
      .catch((err) => {
        if (err.name === 'AbortError') return // component went away; nothing to update
        setError(err instanceof ApiError ? `${err.status}: ${err.message}` : 'could not reach the server')
      })
    return () => controller.abort() // cleanup: cancel the request if the component unmounts
  }, [])

  if (error) return <p>Failed to load movies: {error}</p>
  if (!movies) return <p>Loading…</p>

  return (
    <main>
      <h1>MagicStream</h1>
      <ul>
        {movies.map((m) => (
          <li key={m.imdb_id}>
            {m.title}: {m.ranking.ranking_name}
          </li>
        ))}
      </ul>
    </main>
  )
}