import { A } from '@solidjs/router'

export default function Nav() {
  return (
    <nav>
      <A href="/" class="nav-logo" activeClass="">
        <span>⚽</span>
        <span>WC 2026 <span class="logo-accent">Sweepstakes</span></span>
        <span class="logo-badge">LIVE</span>
      </A>
      <A href="/" activeClass="active" end>Matches</A>
      <A href="/leaderboard" activeClass="active">Leaderboard</A>
      <A href="/notifications" activeClass="active">Notifications</A>
    </nav>
  )
}
