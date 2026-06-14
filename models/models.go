package models

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Match struct {
	ID           string `json:"id"`
	HomeTeamID   int    `json:"homeTeamId"`
	HomeTeam     string `json:"homeTeam"`
	HomeTeamCode string `json:"homeTeamCode"`
	AwayTeamID   int    `json:"awayTeamId"`
	AwayTeam     string `json:"awayTeam"`
	AwayTeamCode string `json:"awayTeamCode"`
	HomeScore    *int   `json:"homeScore"`
	AwayScore    *int   `json:"awayScore"`
	Status       string `json:"status"`
	MatchDate    string `json:"matchDate"`
	Stage        string `json:"stage"`
}

type TeamInfo struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type LeaderboardEntry struct {
	Rank   int        `json:"rank"`
	UserID int        `json:"userId"`
	Name   string     `json:"name"`
	Points float64    `json:"points"`
	Teams  []TeamInfo `json:"teams"`
}
