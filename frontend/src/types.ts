export interface Match {
  id: string
  homeTeam: string
  homeTeamCode: string
  homeTeamId: number
  homeTeamRank: number | null
  awayTeam: string
  awayTeamCode: string
  awayTeamId: number
  awayTeamRank: number | null
  homeScore: number | null
  awayScore: number | null
  status: string
  matchDate: string
  stage: string
}

export interface MatchesResponse {
  matches: Match[]
  teamOwners: Record<string, string>
}

export interface TeamInfo {
  name: string
  code: string
}

export interface LeaderboardEntry {
  rank: number
  userId: number
  name: string
  points: number
  teams: TeamInfo[]
}

export interface User {
  id: number
  name: string
}
