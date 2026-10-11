import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { api, errorMessage } from '../api/client'
import type { Movie } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { MovieCard } from '../components/MovieCard'

export function RecommendationsPage() {
  const { user } = useAuth()
  const [movies, setMovies] = useState<Movie[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    api<Movie[]>('/recommendations?limit=12', { signal: controller.signal })
      .then(setMovies)
      .catch((err) => {
        if (err.name !== 'AbortError') setError(errorMessage(err))
      })
    return () => controller.abort()
  }, [])

  const genres = user?.favorite_genres.map((g) => g.genre_name).join(', ')

  return (
    <>
      <h1 className="page-title">For you</h1>
      <p className="lede">
        Films ranked Okay or better in {genres ? `your genres: ${genres}` : 'your favorite genres'}, best first.
      </p>
      {error && (
        <p className="form-error" role="alert">
          Couldn't load recommendations: {error}
        </p>
      )}
      {!error && !movies && <p className="status">Loading recommendations…</p>}
      {movies && movies.length === 0 && (
        <>
          <p className="lede">Nothing in your genres is ranked Okay or better yet. New reviews will appear here as they're written.</p>
          <Link to="/">Browse all movies</Link>
        </>
      )}
      {movies && movies.length > 0 && (
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