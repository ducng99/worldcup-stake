package db

import (
	"database/sql"
	"log"
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
	cleaned, err := cleanupDuplicateMatches(database)
	if err != nil {
		return nil, err
	}
	if cleaned > 0 {
		log.Printf("DB: cleaned up %d duplicate match rows", cleaned)
		if err := refreshLeaderboardState(database); err != nil {
			return nil, err
		}
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

type duplicateMatch struct {
	orphanID    string
	canonicalID string
}

func cleanupDuplicateMatches(database *sql.DB) (int, error) {
	rows, err := database.Query(`
		SELECT o.id, MIN(c.id)
		FROM matches o
		JOIN matches c ON c.id <> o.id
			AND c.home_team_id = o.home_team_id
			AND c.away_team_id = o.away_team_id
			AND COALESCE(c.stage, '') = COALESCE(o.stage, '')
		JOIN match_sources cs ON cs.match_id = c.id
		LEFT JOIN match_sources os ON os.match_id = o.id
		WHERE os.match_id IS NULL
		GROUP BY o.id
		HAVING COUNT(DISTINCT c.id) = 1
		ORDER BY COALESCE(o.match_date, '')
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	duplicates := []duplicateMatch{}
	for rows.Next() {
		var duplicate duplicateMatch
		if err := rows.Scan(&duplicate.orphanID, &duplicate.canonicalID); err != nil {
			return 0, err
		}
		duplicates = append(duplicates, duplicate)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(duplicates) == 0 {
		return 0, nil
	}

	tx, err := database.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, duplicate := range duplicates {
		_, err := tx.Exec(`
			UPDATE matches
			SET home_score = (SELECT home_score FROM matches WHERE id = ?),
				away_score = (SELECT away_score FROM matches WHERE id = ?),
				status = (SELECT status FROM matches WHERE id = ?),
				match_date = (SELECT match_date FROM matches WHERE id = ?),
				stage = (SELECT stage FROM matches WHERE id = ?)
			WHERE id = ?
		`, duplicate.orphanID, duplicate.orphanID, duplicate.orphanID, duplicate.orphanID, duplicate.orphanID, duplicate.canonicalID)
		if err != nil {
			return 0, err
		}

		if _, err := tx.Exec("DELETE FROM matches WHERE id = ?", duplicate.orphanID); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(duplicates), nil
}

func refreshLeaderboardState(database *sql.DB) error {
	_, err := database.Exec(`
		WITH player_match_teams AS (
			SELECT
				u.id,
				m.id AS match_id,
				m.match_date,
				MAX(CASE WHEN m.home_team_id = ut.team_id THEN 1 ELSE 0 END) AS owns_home,
				MAX(CASE WHEN m.away_team_id = ut.team_id THEN 1 ELSE 0 END) AS owns_away,
				m.home_score,
				m.away_score
			FROM users u
			JOIN user_teams ut ON ut.user_id = u.id
			JOIN matches m ON (m.home_team_id = ut.team_id OR m.away_team_id = ut.team_id)
			WHERE m.status = 'FINISHED' AND m.home_score IS NOT NULL AND m.away_score IS NOT NULL
			GROUP BY u.id, m.id, m.match_date, m.home_score, m.away_score
		),
		player_match_points AS (
			SELECT
				id,
				match_id,
				match_date,
				CASE
					WHEN owns_home = 1 AND owns_away = 1 THEN 1.0
					WHEN home_score = away_score THEN 0.5
					WHEN owns_home = 1 AND home_score > away_score THEN 1.0
					WHEN owns_away = 1 AND away_score > home_score THEN 1.0
					ELSE 0.0
				END AS points
			FROM player_match_teams
		),
		player_progress AS (
			SELECT
				id,
				match_date,
				SUM(points) OVER (PARTITION BY id ORDER BY match_date, match_id ROWS UNBOUNDED PRECEDING) AS cumulative_points
			FROM player_match_points
			WHERE points > 0
		),
		leaderboard AS (
			SELECT
				u.id,
				COALESCE(scores.total_points, 0) AS points,
				(
					SELECT MIN(pp.match_date)
					FROM player_progress pp
					WHERE pp.id = u.id AND pp.cumulative_points >= COALESCE(scores.total_points, 0)
				) AS reached_date
			FROM users u
			LEFT JOIN (
				SELECT id, SUM(points) AS total_points
				FROM player_match_points
				GROUP BY id
			) scores ON scores.id = u.id
		),
		ranked AS (
			SELECT
				id,
				ROW_NUMBER() OVER (ORDER BY points DESC, reached_date ASC) AS rank,
				points
			FROM leaderboard
		)
		INSERT INTO leaderboard_state (user_id, rank, points, updated_at)
		SELECT id, rank, points, datetime('now')
		FROM ranked
		WHERE true
		ON CONFLICT(user_id) DO UPDATE SET
			rank = excluded.rank,
			points = excluded.points,
			updated_at = excluded.updated_at
	`)
	return err
}
