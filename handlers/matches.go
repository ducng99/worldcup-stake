package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"worldcup-stake/models"
)

type MatchesResponse struct {
	Matches    []models.Match    `json:"matches"`
	TeamOwners map[string]string `json:"teamOwners"`
}

func GetMatches(database *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := database.Query(`
			SELECT
				m.id, m.match_date, m.status, m.stage,
				ht.id, ht.name, ht.code,
				at.id, at.name, at.code,
				htr.rank, atr.rank,
				m.home_score, m.away_score
			FROM matches m
			JOIN teams ht ON ht.id = m.home_team_id
			JOIN teams at ON at.id = m.away_team_id
			LEFT JOIN team_rankings htr ON htr.country_code = ht.code
			LEFT JOIN team_rankings atr ON atr.country_code = at.code
			ORDER BY m.match_date ASC
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var matches []models.Match
		for rows.Next() {
			var m models.Match
			var homeRank, awayRank, homeScore, awayScore sql.NullInt64
			if err := rows.Scan(
				&m.ID, &m.MatchDate, &m.Status, &m.Stage,
				&m.HomeTeamID, &m.HomeTeam, &m.HomeTeamCode,
				&m.AwayTeamID, &m.AwayTeam, &m.AwayTeamCode,
				&homeRank, &awayRank,
				&homeScore, &awayScore,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if homeScore.Valid {
				v := int(homeScore.Int64)
				m.HomeScore = &v
			}
			if homeRank.Valid {
				v := int(homeRank.Int64)
				m.HomeTeamRank = &v
			}
			if awayScore.Valid {
				v := int(awayScore.Int64)
				m.AwayScore = &v
			}
			if awayRank.Valid {
				v := int(awayRank.Int64)
				m.AwayTeamRank = &v
			}
			matches = append(matches, m)
		}
		if matches == nil {
			matches = []models.Match{}
		}

		ownerRows, err := database.Query(`
			SELECT t.code, u.name
			FROM user_teams ut
			JOIN teams t ON t.id = ut.team_id
			JOIN users u ON u.id = ut.user_id
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer ownerRows.Close()

		teamOwners := map[string]string{}
		for ownerRows.Next() {
			var code, name string
			if err := ownerRows.Scan(&code, &name); err != nil {
				continue
			}
			teamOwners[code] = name
		}

		c.JSON(http.StatusOK, MatchesResponse{
			Matches:    matches,
			TeamOwners: teamOwners,
		})
	}
}
