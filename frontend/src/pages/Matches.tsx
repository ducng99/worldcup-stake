import { createResource, createSignal, For, onCleanup, Show } from 'solid-js'
import type { MatchesResponse } from '../types'
import MatchCard from '../components/MatchCard'

async function fetchMatches(): Promise<MatchesResponse> {
  const res = await fetch('/api/matches')
  if (!res.ok) throw new Error('Failed to fetch matches')
  return res.json()
}

export default function Matches() {
  const [data, { refetch }] = createResource(fetchMatches)
  const [filterPlayer, setFilterPlayer] = createSignal('')

  const refreshTimer = window.setInterval(() => {
    refetch()
  }, 30_000)

  onCleanup(() => window.clearInterval(refreshTimer))

  const players = () => {
    const owners = data()?.teamOwners
    if (!owners) return []
    return [...new Set(Object.values(owners))].sort()
  }

  const filterMatches = (matches: ReturnType<typeof upcoming>) => {
    const player = filterPlayer()
    if (!player) return matches
    const owners = data()!.teamOwners
    return matches.filter(
      (m) => owners[m.homeTeamCode] === player || owners[m.awayTeamCode] === player
    )
  }

  const upcoming = () =>
    data()?.matches.filter(
      (m) => m.status !== 'FINISHED' && m.status !== 'AWARDED' && m.status !== 'CANCELLED'
    ) ?? []

  const finished = () =>
    [...(data()?.matches.filter(
      (m) => m.status === 'FINISHED' || m.status === 'AWARDED'
    ) ?? [])].reverse()

  const visibleUpcoming = () => filterMatches(upcoming())
  const visibleFinished = () => filterMatches(finished())

  return (
    <div>
      <Show when={data.loading}>
        <p class="loading">Loading matches…</p>
      </Show>
      <Show when={data.error}>
        <p class="error">Failed to load matches. Is the backend running?</p>
      </Show>
      <Show when={data()}>
        <div class="filter-bar">
          <select
            class="player-filter"
            value={filterPlayer()}
            onChange={(e) => setFilterPlayer(e.currentTarget.value)}
          >
            <option value="">All players</option>
            <For each={players()}>
              {(player) => <option value={player}>{player}</option>}
            </For>
          </select>
          <Show when={filterPlayer()}>
            <button class="filter-clear" onClick={() => setFilterPlayer('')}>✕</button>
          </Show>
        </div>
        <section>
          <div class="section-header">
            <h2 class="section-title">Upcoming</h2>
            <div class="section-line" />
            <span class="section-count">{visibleUpcoming().length}</span>
          </div>
          <Show when={visibleUpcoming().length === 0} fallback={
            <div class="match-list">
              <For each={visibleUpcoming()}>
                {(match) => <MatchCard match={match} teamOwners={data()!.teamOwners} />}
              </For>
            </div>
          }>
            <p class="empty">No upcoming matches.</p>
          </Show>
        </section>
        <section>
          <div class="section-header">
            <h2 class="section-title">Results</h2>
            <div class="section-line" />
            <span class="section-count">{visibleFinished().length}</span>
          </div>
          <Show when={visibleFinished().length === 0} fallback={
            <div class="match-list">
              <For each={visibleFinished()}>
                {(match) => <MatchCard match={match} teamOwners={data()!.teamOwners} />}
              </For>
            </div>
          }>
            <p class="empty">No results yet.</p>
          </Show>
        </section>
      </Show>
    </div>
  )
}
