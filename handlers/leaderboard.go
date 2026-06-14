package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"worldcup-stake/models"
)

func GetLeaderboard(database *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		entries, err := ComputeLeaderboard(database)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, entries)
	}
}

func ComputeLeaderboard(database *sql.DB) ([]models.LeaderboardEntry, error) {
	rows, err := database.Query(`
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
		)
		SELECT
			u.id,
			u.name,
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
		ORDER BY points DESC, reached_date ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LeaderboardEntry
	for rows.Next() {
		var e models.LeaderboardEntry
		var reachedDate sql.NullString
		if err := rows.Scan(&e.UserID, &e.Name, &e.Points, &reachedDate); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	teamRows, err := database.Query(`
		SELECT ut.user_id, t.name, t.code
		FROM user_teams ut
		JOIN teams t ON t.id = ut.team_id
		ORDER BY ut.user_id, t.name
	`)
	if err != nil {
		return nil, err
	}
	defer teamRows.Close()

	userTeams := map[int][]models.TeamInfo{}
	for teamRows.Next() {
		var userID int
		var t models.TeamInfo
		if err := teamRows.Scan(&userID, &t.Name, &t.Code); err != nil {
			continue
		}
		userTeams[userID] = append(userTeams[userID], t)
	}

	for i := range entries {
		entries[i].Rank = i + 1
		entries[i].Teams = userTeams[entries[i].UserID]
		if entries[i].Teams == nil {
			entries[i].Teams = []models.TeamInfo{}
		}
	}

	if entries == nil {
		entries = []models.LeaderboardEntry{}
	}

	return entries, nil
}
