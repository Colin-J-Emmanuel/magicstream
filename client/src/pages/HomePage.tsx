import { useEffect, useState } from 'react'
import { api, errorMessage } from '../api/client'
import type { Movie } from '../api/types'
import { MovieCard } from '../components/MovieCard'

export function HomePage() {
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

  return (
    <>
      <h1 className="page-title">Now showing</h1>
      <p className="lede">Every film is reviewed by our editor, and the review is ranked on five lights. Pick one to watch the trailer.</p>
      {error && (
        <p className="form-error" role="alert">
          Couldn't load movies: {error}
        </p>
      )}
      {!error && !movies && <p className="status">Loading movies…</p>}
      {movies && (
        <ul className="movie-grid">
          {movies.map((m) => (
            <li key={m.imdb_id}>
              <MovieCard movie={m} />
            </li>
          ))}
        </ul>
      )}
    </>
  )
}