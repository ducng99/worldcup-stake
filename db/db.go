package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Init() (*sql.DB, error) {
	if err := os.MkdirAll(DataDir(), 0o755); err != nil {
		return nil, err
	}

	database, err := sql.Open("sqlite", Path())
	if err != nil {
		return nil, err
	}
	if err := migrate(database); err != nil {
		return nil, err
	}
	return database, nil
}

func Path() string {
	return filepath.Join(DataDir(), "stake.db")
}

func DataDir() string {
	path := os.Getenv("DATA_DIR")
	if path == "" {
		return "data"
	}
	return path
}

func migrate(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id   INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS teams (
			id   INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			code TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS user_teams (
			user_id INTEGER REFERENCES users(id),
			team_id INTEGER REFERENCES teams(id),
			PRIMARY KEY (user_id, team_id)
		);
		CREATE TABLE IF NOT EXISTS matches (
			id           TEXT PRIMARY KEY,
			home_team_id INTEGER REFERENCES teams(id),
			away_team_id INTEGER REFERENCES teams(id),
			home_score   INTEGER,
			away_score   INTEGER,
			status       TEXT,
			match_date   TEXT,
			stage        TEXT
		);
		CREATE TABLE IF NOT EXISTS match_sources (
			match_id        TEXT NOT NULL REFERENCES matches(id),
			source          TEXT NOT NULL,
			source_match_id TEXT NOT NULL,
			PRIMARY KEY (match_id, source),
			UNIQUE(source, source_match_id)
		);
		CREATE TABLE IF NOT EXISTS push_subscriptions (
			id                 INTEGER PRIMARY KEY,
			user_id            INTEGER REFERENCES users(id),
			endpoint           TEXT NOT NULL UNIQUE,
			p256dh             TEXT NOT NULL,
			auth               TEXT NOT NULL,
			notify_leaderboard BOOLEAN NOT NULL DEFAULT 1,
			notify_match_start BOOLEAN NOT NULL DEFAULT 1,
			created_at         TEXT NOT NULL,
			updated_at         TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS notification_deliveries (
			subscription_id INTEGER REFERENCES push_subscriptions(id),
			event_key       TEXT NOT NULL,
			sent_at         TEXT NOT NULL,
			PRIMARY KEY (subscription_id, event_key)
		);
		CREATE TABLE IF NOT EXISTS leaderboard_state (
			user_id    INTEGER PRIMARY KEY REFERENCES users(id),
			rank       INTEGER NOT NULL,
			points     REAL NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS team_rankings (
			country_code     TEXT PRIMARY KEY,
			team_name        TEXT NOT NULL,
			rank             INTEGER NOT NULL,
			prev_rank        INTEGER NOT NULL,
			total_points     REAL NOT NULL,
			prev_points      REAL NOT NULL,
			ranking_movement INTEGER NOT NULL,
			rated_matches    INTEGER NOT NULL,
			updated_at       TEXT NOT NULL
		);
		UPDATE matches
		SET status = CASE UPPER(TRIM(status))
			WHEN 'SCHEDULED' THEN 'UPCOMING'
			WHEN 'TIMED' THEN 'UPCOMING'
			WHEN 'IN_PLAY' THEN 'LIVE'
			WHEN 'PAUSED' THEN 'LIVE'
			WHEN 'AWARDED' THEN 'FINISHED'
			WHEN 'CANCELLED' THEN 'FINISHED'
			WHEN 'POSTPONED' THEN 'FINISHED'
			WHEN 'SUSPENDED' THEN 'FINISHED'
			ELSE UPPER(TRIM(status))
		END
		WHERE status IS NOT NULL
			AND UPPER(TRIM(status)) NOT IN ('UPCOMING', 'LIVE', 'FINISHED');
	`)
	return err
}
