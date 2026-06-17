package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Syncer struct {
	db                 *sql.DB
	providers          []MatchProvider
	push               *PushService
	rankingProvider    *fifaRankingProvider
	rankingUpdateDelay time.Duration
}

func NewSyncer(database *sql.DB, providers []MatchProvider, push *PushService) *Syncer {
	return &Syncer{
		db:                 database,
		providers:          providers,
		push:               push,
		rankingProvider:    newFIFARankingProvider(),
		rankingUpdateDelay: 2 * time.Minute,
	}
}

type MatchProvider interface {
	Name() string
	FetchMatches() ([]ProviderMatch, error)
}

type ProviderMatch struct {
	Source       string
	SourceID     string
	HomeTeamCode string
	AwayTeamCode string
	HomeScore    *int
	AwayScore    *int
	Status       string
	MatchDate    string
	Stage        string
}

type FootballDataProvider struct {
	apiKey string
	client *http.Client
}

func NewFootballDataProvider(apiKey string) *FootballDataProvider {
	return &FootballDataProvider{apiKey: apiKey, client: http.DefaultClient}
}

func (p *FootballDataProvider) Name() string {
	return "football-data"
}

func (p *FootballDataProvider) FetchMatches() ([]ProviderMatch, error) {
	req, err := http.NewRequest("GET", "https://api.football-data.org/v4/competitions/WC/matches", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Auth-Token", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result footballDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	matches := make([]ProviderMatch, 0, len(result.Matches))
	for _, m := range result.Matches {
		matches = append(matches, ProviderMatch{
			Source:       p.Name(),
			SourceID:     fmt.Sprintf("%d", m.ID),
			HomeTeamCode: m.HomeTeam.TLA,
			AwayTeamCode: m.AwayTeam.TLA,
			HomeScore:    m.Score.FullTime.Home,
			AwayScore:    m.Score.FullTime.Away,
			Status:       m.Status,
			MatchDate:    m.UtcDate,
			Stage:        m.Stage,
		})
	}
	return matches, nil
}

type FIFAProvider struct {
	client *http.Client
}

func NewFIFAProvider() *FIFAProvider {
	return &FIFAProvider{client: http.DefaultClient}
}

func (p *FIFAProvider) Name() string {
	return "fifa"
}

func (p *FIFAProvider) FetchMatches() ([]ProviderMatch, error) {
	req, err := http.NewRequest("GET", "https://api.fifa.com/api/v3/calendar/matches?language=en&count=500&idCompetition=17&idSeason=285023", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result fifaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	matches := make([]ProviderMatch, 0, len(result.Results))
	for _, m := range result.Results {
		matches = append(matches, ProviderMatch{
			Source:       p.Name(),
			SourceID:     m.ID,
			HomeTeamCode: m.Home.Abbreviation,
			AwayTeamCode: m.Away.Abbreviation,
			HomeScore:    m.HomeTeamScore,
			AwayScore:    m.AwayTeamScore,
			Status:       fifaMatchStatus(m.MatchStatus),
			MatchDate:    m.Date,
			Stage:        fifaStage(m),
		})
	}
	return matches, nil
}

type fifaRankingProvider struct {
	url    string
	client *http.Client
}

type teamRanking struct {
	CountryCode     string
	TeamName        string
	Rank            int
	PrevRank        int
	TotalPoints     float64
	PrevPoints      float64
	RankingMovement int
	RatedMatches    int
}

func newFIFARankingProvider() *fifaRankingProvider {
	return &fifaRankingProvider{
		url:    "https://api.fifa.com/api/v3/fifarankings/rankings/live?gender=1&sportType=0&language=en",
		client: http.DefaultClient,
	}
}

func (p *fifaRankingProvider) fetchRankings() ([]teamRanking, error) {
	req, err := http.NewRequest("GET", p.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result fifaRankingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	rankings := make([]teamRanking, 0, len(result.Results))
	for _, r := range result.Results {
		countryCode := normalizeTeamCode(r.CountryCode)
		if countryCode == "" || r.Rank == 0 {
			continue
		}
		rankings = append(rankings, teamRanking{
			CountryCode:     countryCode,
			TeamName:        localizedDescription(r.TeamName),
			Rank:            r.Rank,
			PrevRank:        r.PrevRank,
			TotalPoints:     r.TotalPoints,
			PrevPoints:      r.PrevPoints,
			RankingMovement: r.RankingMovement,
			RatedMatches:    r.RatedMatches,
		})
	}
	return rankings, nil
}

type teamInfo struct {
	ID   int64
	Name string
}

type footballDataResponse struct {
	Matches []footballDataMatch `json:"matches"`
}

type footballDataMatch struct {
	ID       int      `json:"id"`
	UtcDate  string   `json:"utcDate"`
	Status   string   `json:"status"`
	Stage    string   `json:"stage"`
	HomeTeam apiTeam  `json:"homeTeam"`
	AwayTeam apiTeam  `json:"awayTeam"`
	Score    apiScore `json:"score"`
}

type apiTeam struct {
	TLA string `json:"tla"`
}

type apiScore struct {
	FullTime apiScorePair `json:"fullTime"`
}

type apiScorePair struct {
	Home *int `json:"home"`
	Away *int `json:"away"`
}

type fifaResponse struct {
	Results []fifaMatch `json:"Results"`
}

type fifaRankingsResponse struct {
	Results []fifaRanking `json:"Results"`
}

type fifaRanking struct {
	TeamName        []localizedValue `json:"TeamName"`
	CountryCode     string           `json:"IdCountry"`
	Rank            int              `json:"Rank"`
	PrevRank        int              `json:"PrevRank"`
	TotalPoints     float64          `json:"TotalPoints"`
	PrevPoints      float64          `json:"PrevPoints"`
	RankingMovement int              `json:"RankingMovement"`
	RatedMatches    int              `json:"RatedMatches"`
}

type fifaMatch struct {
	ID            string           `json:"IdMatch"`
	Date          string           `json:"Date"`
	MatchStatus   int              `json:"MatchStatus"`
	Home          fifaTeam         `json:"Home"`
	Away          fifaTeam         `json:"Away"`
	HomeTeamScore *int             `json:"HomeTeamScore"`
	AwayTeamScore *int             `json:"AwayTeamScore"`
	StageName     []localizedValue `json:"StageName"`
	GroupName     []localizedValue `json:"GroupName"`
}

type fifaTeam struct {
	Abbreviation string `json:"Abbreviation"`
}

type localizedValue struct {
	Locale      string `json:"Locale"`
	Description string `json:"Description"`
}

func (s *Syncer) Sync() {
	log.Println("Syncing match data...")

	for _, provider := range s.providers {
		matches, err := provider.FetchMatches()
		if err != nil {
			log.Printf("Sync: provider %s failed: %v", provider.Name(), err)
			continue
		}
		if len(matches) == 0 {
			log.Printf("Sync: provider %s returned no matches", provider.Name())
			continue
		}
		if err := s.syncMatches(provider.Name(), matches); err != nil {
			log.Printf("Sync: provider %s sync failed: %v", provider.Name(), err)
			continue
		}
		return
	}

	log.Println("Sync: all providers failed")
}

func (s *Syncer) HasLiveMatches() bool {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM matches WHERE status = 'LIVE'").Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (s *Syncer) TimeUntilNextMatch() time.Duration {
	var matchDate string
	err := s.db.QueryRow("SELECT match_date FROM matches WHERE status = 'UPCOMING' ORDER BY match_date ASC LIMIT 1").Scan(&matchDate)
	if err != nil {
		return time.Hour
	}
	t, err := time.Parse(time.RFC3339, matchDate)
	if err != nil {
		return time.Hour
	}
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

func (s *Syncer) syncMatches(providerName string, matches []ProviderMatch) error {
	teamMap, err := s.buildTeamMap()
	if err != nil {
		return fmt.Errorf("build team map: %w", err)
	}

	updated := 0
	shouldRefreshRankings := false
	for _, m := range matches {
		source := m.Source
		if source == "" {
			source = providerName
		}
		homeCode := normalizeTeamCode(m.HomeTeamCode)
		awayCode := normalizeTeamCode(m.AwayTeamCode)
		if homeCode == "" || awayCode == "" || m.MatchDate == "" || m.SourceID == "" {
			continue
		}
		home, homeOK := teamMap[homeCode]
		away, awayOK := teamMap[awayCode]
		if !homeOK || !awayOK {
			log.Printf("Sync: unknown team code home=%q away=%q, skipping %s match %s", homeCode, awayCode, source, m.SourceID)
			continue
		}
		matchID := internalMatchID(m.MatchDate, homeCode, awayCode)
		previousStatus, err := s.previousMatchStatus(matchID)
		if err != nil {
			log.Printf("Sync: failed to fetch previous match %s status: %v", matchID, err)
			continue
		}
		nextStatus := nextMatchStatus(previousStatus, m.Status)

		_, err = s.db.Exec(`
			INSERT INTO matches (id, home_team_id, away_team_id, home_score, away_score, status, match_date, stage)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				home_score = COALESCE(excluded.home_score, matches.home_score),
				away_score = COALESCE(excluded.away_score, matches.away_score),
				status     = excluded.status,
				match_date = excluded.match_date,
				stage      = excluded.stage
		`, matchID, home.ID, away.ID,
			m.HomeScore, m.AwayScore,
			nextStatus, m.MatchDate, m.Stage)
		if err != nil {
			log.Printf("Sync: failed to upsert match %s: %v", matchID, err)
			continue
		}
		_, err = s.db.Exec(`
			INSERT INTO match_sources (match_id, source, source_match_id)
			VALUES (?, ?, ?)
			ON CONFLICT(match_id, source) DO UPDATE SET
				source_match_id = excluded.source_match_id
		`, matchID, source, m.SourceID)
		if err != nil {
			log.Printf("Sync: failed to upsert source %s match %s: %v", source, m.SourceID, err)
			continue
		}
		if s.push != nil && isUpcomingStatus(previousStatus) && isLiveStatus(nextStatus) {
			s.push.NotifyMatchStart(matchID, home.ID, away.ID, home.Name, away.Name)
		}
		if isLiveStatus(previousStatus) && isFinishedStatus(nextStatus) {
			shouldRefreshRankings = true
		}
		updated++
	}
	if shouldRefreshRankings {
		s.scheduleRankingUpdate()
	}
	if s.push != nil {
		if err := s.notifyLeaderboardChanges(); err != nil {
			log.Printf("Sync: failed to process leaderboard notifications: %v", err)
		}
	}

	log.Printf("Sync: %d/%d matches upserted from %s", updated, len(matches), providerName)
	return nil
}

func (s *Syncer) scheduleRankingUpdate() {
	if s.rankingProvider == nil {
		return
	}
	delay := s.rankingUpdateDelay
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		if err := s.refreshTeamRankings(); err != nil {
			log.Printf("Rankings: failed to refresh team rankings: %v", err)
		}
	}()
}

func (s *Syncer) refreshTeamRankings() error {
	if s.rankingProvider == nil {
		return nil
	}
	rankings, err := s.rankingProvider.fetchRankings()
	if err != nil {
		return err
	}
	if len(rankings) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	updated := 0
	for _, ranking := range rankings {
		_, err := tx.Exec(`
			INSERT INTO team_rankings (
				country_code, team_name, rank, prev_rank,
				total_points, prev_points, ranking_movement, rated_matches, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(country_code) DO UPDATE SET
				team_name = excluded.team_name,
				rank = excluded.rank,
				prev_rank = excluded.prev_rank,
				total_points = excluded.total_points,
				prev_points = excluded.prev_points,
				ranking_movement = excluded.ranking_movement,
				rated_matches = excluded.rated_matches,
				updated_at = excluded.updated_at
		`, ranking.CountryCode, ranking.TeamName, ranking.Rank, ranking.PrevRank,
			ranking.TotalPoints, ranking.PrevPoints, ranking.RankingMovement, ranking.RatedMatches, now)
		if err != nil {
			return err
		}
		updated++
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("Rankings: %d team rankings upserted", updated)
	return nil
}

func fifaMatchStatus(status int) string {
	switch status {
	case 0:
		return "FINISHED"
	case 3:
		return "LIVE"
	case 1:
		return "UPCOMING"
	default:
		return "UPCOMING"
	}
}

func fifaStage(m fifaMatch) string {
	if group := localizedDescription(m.GroupName); group != "" {
		return group
	}
	return localizedDescription(m.StageName)
}

func localizedDescription(values []localizedValue) string {
	for _, value := range values {
		if value.Locale == "en-GB" && value.Description != "" {
			return value.Description
		}
	}
	for _, value := range values {
		if value.Description != "" {
			return value.Description
		}
	}
	return ""
}

var nonMatchIDChars = regexp.MustCompile(`[^A-Z0-9]+`)

var teamCodeAliases = map[string]string{
	"URU": "URY",
}

func normalizeTeamCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if alias, ok := teamCodeAliases[code]; ok {
		return alias
	}
	return code
}

func internalMatchID(matchDate, homeCode, awayCode string) string {
	t, err := time.Parse(time.RFC3339, matchDate)
	if err != nil {
		return fmt.Sprintf("%s_%s_%s", cleanMatchIDPart(matchDate), cleanMatchIDPart(homeCode), cleanMatchIDPart(awayCode))
	}
	return fmt.Sprintf("%s_%s_%s", t.UTC().Format("200601021504"), cleanMatchIDPart(homeCode), cleanMatchIDPart(awayCode))
}

func cleanMatchIDPart(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = nonMatchIDChars.ReplaceAllString(value, "")
	return value
}

func (s *Syncer) buildTeamMap() (map[string]teamInfo, error) {
	rows, err := s.db.Query("SELECT id, code, name FROM teams")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[string]teamInfo{}
	for rows.Next() {
		var id int64
		var code, name string
		if err := rows.Scan(&id, &code, &name); err != nil {
			return nil, err
		}
		m[normalizeTeamCode(code)] = teamInfo{ID: id, Name: name}
	}
	return m, nil
}

func (s *Syncer) previousMatchStatus(matchID string) (string, error) {
	var status string
	err := s.db.QueryRow("SELECT status FROM matches WHERE id = ?", matchID).Scan(&status)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return status, err
}

func isLiveStatus(status string) bool {
	return normalizeMatchStatus(status) == "LIVE"
}

func isUpcomingStatus(status string) bool {
	return normalizeMatchStatus(status) == "UPCOMING"
}

func isFinishedStatus(status string) bool {
	return normalizeMatchStatus(status) == "FINISHED"
}

func normalizeStatus(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}

func normalizeMatchStatus(status string) string {
	switch normalizeStatus(status) {
	case "UPCOMING", "SCHEDULED", "TIMED":
		return "UPCOMING"
	case "LIVE", "IN_PLAY", "PAUSED":
		return "LIVE"
	case "FINISHED", "AWARDED", "CANCELLED", "POSTPONED", "SUSPENDED":
		return "FINISHED"
	default:
		return "UPCOMING"
	}
}

func nextMatchStatus(previousStatus, incomingStatus string) string {
	incoming := normalizeMatchStatus(incomingStatus)
	if normalizeStatus(previousStatus) == "" {
		return incoming
	}
	previous := normalizeMatchStatus(previousStatus)
	if isFinishedStatus(previous) {
		return previous
	}
	if isLiveStatus(previous) {
		if isFinishedStatus(incoming) {
			return incoming
		}
		return previous
	}
	if isUpcomingStatus(previous) {
		if isUpcomingStatus(incoming) || isLiveStatus(incoming) || isFinishedStatus(incoming) {
			return incoming
		}
	}
	return previous
}

func (s *Syncer) notifyLeaderboardChanges() error {
	entries, err := ComputeLeaderboard(s.db)
	if err != nil {
		return err
	}

	var existingCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM leaderboard_state").Scan(&existingCount); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	initializing := existingCount == 0

	for _, entry := range entries {
		var oldRank int
		var oldPoints float64
		err := s.db.QueryRow("SELECT rank, points FROM leaderboard_state WHERE user_id = ?", entry.UserID).Scan(&oldRank, &oldPoints)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if !initializing && err == nil && oldRank != entry.Rank {
			s.push.NotifyLeaderboardChange(entry.UserID, oldRank, entry.Rank, entry.Points)
		}
		_, err = s.db.Exec(`
			INSERT INTO leaderboard_state (user_id, rank, points, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(user_id) DO UPDATE SET
				rank = excluded.rank,
				points = excluded.points,
				updated_at = excluded.updated_at
		`, entry.UserID, entry.Rank, entry.Points, now)
		if err != nil {
			return err
		}
	}

	return nil
}
