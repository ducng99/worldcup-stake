package handlers

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"worldcup-stake/models"
)

func TestComputeLeaderboardScoresDrawsAndSameOwnerMatches(t *testing.T) {
	database := newTestLeaderboardDB(t)

	entries, err := ComputeLeaderboard(database)
	if err != nil {
		t.Fatalf("compute leaderboard: %v", err)
	}

	pointsByName := map[string]float64{}
	for _, entry := range entries {
		pointsByName[entry.Name] = entry.Points
	}

	assertPoints(t, pointsByName, "Ava", 1.5)
	assertPoints(t, pointsByName, "Ben", 0.5)
	assertPoints(t, pointsByName, "Cam", 1.0)

	teamsByEntry := map[string][]models.TeamInfo{}
	for _, entry := range entries {
		teamsByEntry[entry.Name] = entry.Teams
	}
	if got := teamsByEntry["Cam"]; len(got) != 2 || got[0].Code != "DEL" || got[1].Code != "GAM" {
		t.Fatalf("Cam teams = %v, want [DEL GAM]", got)
	}
}

func assertPoints(t *testing.T, pointsByName map[string]float64, name string, want float64) {
	t.Helper()
	if got := pointsByName[name]; got != want {
		t.Fatalf("%s points = %v, want %v", name, got, want)
	}
}

func newTestLeaderboardDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	_, err = database.Exec(`
		CREATE TABLE users (
			id   INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE TABLE teams (
			id   INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			code TEXT NOT NULL
		);
		CREATE TABLE user_teams (
			user_id INTEGER REFERENCES users(id),
			team_id INTEGER REFERENCES teams(id),
			PRIMARY KEY (user_id, team_id)
		);
		CREATE TABLE matches (
			id           TEXT PRIMARY KEY,
			home_team_id INTEGER REFERENCES teams(id),
			away_team_id INTEGER REFERENCES teams(id),
			home_score   INTEGER,
			away_score   INTEGER,
			status       TEXT,
			match_date   TEXT,
			stage        TEXT
		);

		INSERT INTO users (id, name) VALUES (1, 'Ava'), (2, 'Ben'), (3, 'Cam');
		INSERT INTO teams (id, name, code) VALUES
			(1, 'Alpha', 'ALP'),
			(2, 'Beta', 'BET'),
			(3, 'Gamma', 'GAM'),
			(4, 'Delta', 'DEL');
		INSERT INTO user_teams (user_id, team_id) VALUES
			(1, 1),
			(2, 2),
			(3, 3),
			(3, 4);

		INSERT INTO matches (id, home_team_id, away_team_id, home_score, away_score, status, match_date, stage) VALUES
			('draw', 1, 2, 1, 1, 'FINISHED', '2026-06-12T00:00:00Z', 'Group'),
			('same-owner', 3, 4, 2, 0, 'FINISHED', '2026-06-13T00:00:00Z', 'Group'),
			('win', 1, 3, 2, 0, 'FINISHED', '2026-06-14T00:00:00Z', 'Group');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return database
}
