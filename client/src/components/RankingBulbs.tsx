import type { CSSProperties } from 'react'
import type { Ranking } from '../api/types'
import { litBulbs, rankingLabel } from '../lib/ranking'

export function RankingBulbs({ ranking, size = 'small' }: { ranking: Ranking; size?: 'small' | 'large' }) {
  const lit = litBulbs(ranking.ranking_value)
  const label = rankingLabel(ranking)

  return (
    <div className={size === 'large' ? 'bulbs bulbs-large' : 'bulbs'} role="img" aria-label={`Ranking: ${label}`}>
      <span className="bulb-row" aria-hidden="true">
        {Array.from({ length: 5 }, (_, i) => (
          <span key={i} className={i < lit ? 'bulb on' : 'bulb'} style={{ '--i': i } as CSSProperties} />
        ))}
      </span>
      <span className="bulb-label" aria-hidden="true">
        {label}
      </span>
    </div>
  )
}