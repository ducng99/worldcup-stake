import { Show } from 'solid-js'
import type { Match } from '../types'
import { getFlagClass } from '../flags'

interface Props {
  match: Match
  teamOwners: Record<string, string>
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString('en-NZ', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Pacific/Auckland',
    timeZoneName: 'short',
  })
}

function formatStage(stage: string) {
  return stage.replace(/_/g, ' ').toLowerCase().replace(/\b\w/g, (c) => c.toUpperCase())
}

export default function MatchCard(props: Props) {
  const homeOwner = () => props.teamOwners[props.match.homeTeamCode]
  const awayOwner = () => props.teamOwners[props.match.awayTeamCode]
  const homeRank = () => props.match.homeTeamRank
  const awayRank = () => props.match.awayTeamRank
  const homeFlag = () => getFlagClass(props.match.homeTeamCode)
  const awayFlag = () => getFlagClass(props.match.awayTeamCode)

  const isFinished = () => props.match.status === 'FINISHED'
  const isLive = () => props.match.status === 'LIVE'
  const homeScore = () => props.match.homeScore
  const awayScore = () => props.match.awayScore
  const hasScore = () =>
    (isFinished() || isLive()) &&
    homeScore() !== null &&
    awayScore() !== null

  const rowClass = () => {
    if (isFinished()) return 'match-row finished'
    if (isLive()) return 'match-row live'
    return 'match-row upcoming'
  }

  return (
    <div class={rowClass()}>
      <div class={`row-meta${isLive() ? ' row-meta-live' : ''}`}>
        <span class="row-stage">{formatStage(props.match.stage)}</span>
        <span class="row-date">{formatDate(props.match.matchDate)}</span>
      </div>
      <div class="row-fixture">
        <div class="row-team home">
          <div class="row-team-info">
            <span class="row-team-name">{props.match.homeTeam}</span>
            <div class="row-team-badges">
              <Show when={homeRank()}>
                <span class="team-rank-badge">#{homeRank()}</span>
              </Show>
              <Show when={homeOwner()}>
                <span class="owner-badge">{homeOwner()}</span>
              </Show>
            </div>
          </div>
          <Show when={homeFlag()} fallback={<div class="row-flag-placeholder" />}>
            <span class={`${homeFlag()} row-flag`} />
          </Show>
        </div>

        <div class="row-score-block">
          <Show when={hasScore()} fallback={<span class="row-vs">vs</span>}>
            <span class="row-score">
              <span class="row-score-num">{homeScore()}</span>
              <span class="row-score-sep">–</span>
              <span class="row-score-num">{awayScore()}</span>
            </span>
          </Show>
        </div>

        <div class="row-team away">
          <Show when={awayFlag()} fallback={<div class="row-flag-placeholder" />}>
            <span class={`${awayFlag()} row-flag`} />
          </Show>
          <div class="row-team-info away-info">
            <span class="row-team-name">{props.match.awayTeam}</span>
            <div class="row-team-badges">
              <Show when={awayRank()}>
                <span class="team-rank-badge">#{awayRank()}</span>
              </Show>
              <Show when={awayOwner()}>
                <span class="owner-badge">{awayOwner()}</span>
              </Show>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
