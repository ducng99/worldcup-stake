package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCleanupDuplicateMatchesMergesOrphanIntoSourcedMatch(t *testing.T) {
	database := newTestDB(t)
	insertCleanupTeams(t, database)

	_, err := database.Exec(`
		INSERT INTO matches (id, home_team_id, away_team_id, status, match_date, stage)
		VALUES ('202606120200_KOR_CZE', 1, 2, 'UPCOMING', '2026-06-12T02:00:00Z', 'Group A');
		INSERT INTO match_sources (match_id, source, source_match_id)
		VALUES ('202606120200_KOR_CZE', 'fifa', '400021520');
		INSERT INTO matches (id, home_team_id, away_team_id, status, match_date, stage)
		VALUES ('202606120300_KOR_CZE', 1, 2, 'UPCOMING', '2026-06-12T03:00:00Z', 'Group A');
	`)
	if err != nil {
		t.Fatalf("insert duplicate matches: %v", err)
	}

	cleaned, err := cleanupDuplicateMatches(database)
	if err != nil {
		t.Fatalf("cleanupDuplicateMatches() error = %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}

	var matchCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM matches").Scan(&matchCount); err != nil {
		t.Fatalf("query match count: %v", err)
	}
	if matchCount != 1 {
		t.Fatalf("matchCount = %d, want 1", matchCount)
	}

	var matchID, matchDate string
	err = database.QueryRow(`
		SELECT m.id, m.match_date
		FROM matches m
		JOIN match_sources ms ON ms.match_id = m.id
		WHERE ms.source = 'fifa' AND ms.source_match_id = '400021520'
	`).Scan(&matchID, &matchDate)
	if err != nil {
		t.Fatalf("query source match: %v", err)
	}
	if matchID != "202606120200_KOR_CZE" {
		t.Fatalf("matchID = %q, want sourced match ID", matchID)
	}
	if matchDate != "2026-06-12T03:00:00Z" {
		t.Fatalf("matchDate = %q, want orphan match date", matchDate)
	}
}

func TestRefreshLeaderboardStateAfterDuplicateCleanup(t *testing.T) {
	database := newTestDB(t)
	insertCleanupTeams(t, database)

	_, err := database.Exec(`
		INSERT INTO users (id, name) VALUES (1, 'Ava'), (2, 'Ben');
		INSERT INTO user_teams (user_id, team_id) VALUES (1, 1), (2, 2);
		INSERT INTO matches (id, home_team_id, away_team_id, home_score, away_score, status, match_date, stage)
		VALUES ('202606120200_KOR_CZE', 1, 2, 2, 0, 'FINISHED', '2026-06-12T02:00:00Z', 'Group A');
		INSERT INTO match_sources (match_id, source, source_match_id)
		VALUES ('202606120200_KOR_CZE', 'fifa', '400021520');
		INSERT INTO matches (id, home_team_id, away_team_id, home_score, away_score, status, match_date, stage)
		VALUES ('202606120300_KOR_CZE', 1, 2, 2, 0, 'FINISHED', '2026-06-12T03:00:00Z', 'Group A');
		INSERT INTO leaderboard_state (user_id, rank, points, updated_at)
		VALUES (1, 1, 2.0, '2026-06-12T04:00:00Z'), (2, 2, 0.0, '2026-06-12T04:00:00Z');
	`)
	if err != nil {
		t.Fatalf("insert duplicate leaderboard data: %v", err)
	}

	cleaned, err := cleanupDuplicateMatches(database)
	if err != nil {
		t.Fatalf("cleanupDuplicateMatches() error = %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}
	if err := refreshLeaderboardState(database); err != nil {
		t.Fatalf("refreshLeaderboardState() error = %v", err)
	}

	pointsByUser := map[int]float64{}
	rows, err := database.Query("SELECT user_id, points FROM leaderboard_state")
	if err != nil {
		t.Fatalf("query leaderboard state: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var userID int
		var points float64
		if err := rows.Scan(&userID, &points); err != nil {
			t.Fatalf("scan leaderboard state: %v", err)
		}
		pointsByUser[userID] = points
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate leaderboard state: %v", err)
	}
	if pointsByUser[1] != 1.0 {
		t.Fatalf("Ava points = %v, want 1.0", pointsByUser[1])
	}
	if pointsByUser[2] != 0.0 {
		t.Fatalf("Ben points = %v, want 0.0", pointsByUser[2])
	}
}

func TestCleanupDuplicateMatchesSkipsAmbiguousOrphans(t *testing.T) {
	database := newTestDB(t)
	insertCleanupTeams(t, database)

	_, err := database.Exec(`
		INSERT INTO matches (id, home_team_id, away_team_id, status, match_date, stage)
		VALUES ('sourced-1', 1, 2, 'UPCOMING', '2026-06-12T02:00:00Z', 'Group A');
		INSERT INTO match_sources (match_id, source, source_match_id)
		VALUES ('sourced-1', 'fifa', '1');
		INSERT INTO matches (id, home_team_id, away_team_id, status, match_date, stage)
		VALUES ('sourced-2', 1, 2, 'UPCOMING', '2026-06-15T02:00:00Z', 'Group A');
		INSERT INTO match_sources (match_id, source, source_match_id)
		VALUES ('sourced-2', 'football-data', '2');
		INSERT INTO matches (id, home_team_id, away_team_id, status, match_date, stage)
		VALUES ('orphan', 1, 2, 'UPCOMING', '2026-06-12T03:00:00Z', 'Group A');
	`)
	if err != nil {
		t.Fatalf("insert ambiguous matches: %v", err)
	}

	cleaned, err := cleanupDuplicateMatches(database)
	if err != nil {
		t.Fatalf("cleanupDuplicateMatches() error = %v", err)
	}
	if cleaned != 0 {
		t.Fatalf("cleaned = %d, want 0", cleaned)
	}
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := migrate(database); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return database
}

func insertCleanupTeams(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.Exec(`
		INSERT INTO teams (id, name, code) VALUES (1, 'Korea Republic', 'KOR');
		INSERT INTO teams (id, name, code) VALUES (2, 'Czechia', 'CZE');
	`)
	if err != nil {
		t.Fatalf("insert teams: %v", err)
	}
}
