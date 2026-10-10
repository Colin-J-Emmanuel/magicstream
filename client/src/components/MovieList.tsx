import { useEffect, useState } from 'react'
import { api, errorMessage } from '../api/client'
import type { Movie } from '../api/types'

export function MovieList() {
  const [movies, setMovies] = useState<Movie[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    api<Movie[]>('/movies', { signal: controller.signal })
      .then(setMovies)
      .catch((err) => {
        if (err.name !== 'AbortError') setError(errorMessage(err))
      })
    return () => controller.abort()
  }, [])

  if (error) return <p role="alert">Failed to load movies: {error}</p>
  if (!movies) return <p>Loading movies…</p>

  return (
    <ul>
      {movies.map((m) => (
        <li key={m.imdb_id}>
          {m.title}: {m.ranking.ranking_name}
        </li>
      ))}
    </ul>
  )
}