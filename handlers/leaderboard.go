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
		WITH player_wins AS (
			SELECT
				u.id,
				m.match_date,
				ROW_NUMBER() OVER (PARTITION BY u.id ORDER BY m.match_date) AS win_num
			FROM users u
			JOIN user_teams ut ON ut.user_id = u.id
			JOIN matches m ON (m.home_team_id = ut.team_id OR m.away_team_id = ut.team_id)
			WHERE m.status = 'FINISHED' AND (
				(m.home_team_id = ut.team_id AND m.home_score > m.away_score) OR
				(m.away_team_id = ut.team_id AND m.away_score > m.home_score)
			)
		)
		SELECT
			u.id,
			u.name,
			COALESCE(wins.total_points, 0) AS points,
			(
				SELECT pw.match_date
				FROM player_wins pw
				WHERE pw.id = u.id AND pw.win_num = COALESCE(wins.total_points, 0)
			) AS reached_date
		FROM users u
		LEFT JOIN (
			SELECT id, COUNT(*) AS total_points
			FROM player_wins
			GROUP BY id
		) wins ON wins.id = u.id
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
		SELECT ut.user_id, t.name
		FROM user_teams ut
		JOIN teams t ON t.id = ut.team_id
		ORDER BY ut.user_id, t.name
	`)
	if err != nil {
		return nil, err
	}
	defer teamRows.Close()

	userTeams := map[int][]string{}
	for teamRows.Next() {
		var userID int
		var teamName string
		if err := teamRows.Scan(&userID, &teamName); err != nil {
			continue
		}
		userTeams[userID] = append(userTeams[userID], teamName)
	}

	for i := range entries {
		entries[i].Rank = i + 1
		entries[i].Teams = userTeams[entries[i].UserID]
		if entries[i].Teams == nil {
			entries[i].Teams = []string{}
		}
	}

	if entries == nil {
		entries = []models.LeaderboardEntry{}
	}

	return entries, nil
}
