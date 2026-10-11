import type { Ranking } from '../api/types'

export const NOT_RANKED_VALUE = 999

// litBulbs maps a ranking value (1 = Excellent … 5 = Terrible) to lit bulbs (5 … 1).
// Unranked movies light none.
export function litBulbs(rankingValue: number): number {
  return rankingValue >= 1 && rankingValue <= 5 ? 6 - rankingValue : 0
}

export function rankingLabel(ranking: Ranking): string {
  return ranking.ranking_value === NOT_RANKED_VALUE ? 'Awaiting review' : ranking.ranking_name
}