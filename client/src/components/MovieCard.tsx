import { Link } from 'react-router'
import type { Movie } from '../api/types'
import { RankingBulbs } from './RankingBulbs'

export function MovieCard({ movie }: { movie: Movie }) {
  return (
    <Link to={`/movies/${movie.imdb_id}`} className="movie-card">
      <div>
        <h2 className="movie-card-title">{movie.title}</h2>
        <p className="movie-card-genres">{movie.genre.map((g) => g.genre_name).join(', ')}</p>
      </div>
      <RankingBulbs ranking={movie.ranking} />
    </Link>
  )
}