package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/gin-gonic/gin"
)

type PushService struct {
	db         *sql.DB
	publicKey  string
	privateKey string
	subject    string
}

type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

type pushTemplate struct {
	Title string
	Body  string
}

var matchStartTemplates = []pushTemplate{
	{Title: "Kickoff!", Body: "%s vs %s is underway. Time to sweat your stake."},
	{Title: "Ball's rolling", Body: "%s vs %s has kicked off. Your team is on the clock."},
	{Title: "Game on", Body: "%s vs %s is live. Settle in and watch the table shake."},
	{Title: "Whistle blown", Body: "%s vs %s just started. Every minute matters now."},
}

var leaderboardTemplates = []pushTemplate{
	{Title: "The table just moved", Body: "You're now %s after moving from %s."},
	{Title: "Leaderboard drama", Body: "The standings changed: %s now, %s before."},
	{Title: "Table update", Body: "You're sitting %s after being %s. Check the damage."},
	{Title: "Rank shuffle", Body: "You've landed at %s after starting at %s. The race is heating up."},
}

const pushRecordSize uint32 = 2048

type pushSubscriptionInput struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type pushPreferencesInput struct {
	UserID            int                   `json:"userId"`
	Subscription      pushSubscriptionInput `json:"subscription"`
	Endpoint          string                `json:"endpoint"`
	NotifyLeaderboard bool                  `json:"notifyLeaderboard"`
	NotifyMatchStart  bool                  `json:"notifyMatchStart"`
}

type unsubscribeInput struct {
	Endpoint string `json:"endpoint"`
}

type testNotificationInput struct {
	Subscription pushSubscriptionInput `json:"subscription"`
}

func NewPushService(database *sql.DB) *PushService {
	return &PushService{
		db:         database,
		publicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		privateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		subject:    os.Getenv("VAPID_SUBJECT"),
	}
}

func (p *PushService) PublicKey() string {
	return p.publicKey
}

func GetVAPIDPublicKey(push *PushService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if push.PublicKey() == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "VAPID_PUBLIC_KEY not set"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"publicKey": push.PublicKey()})
	}
}

func SubscribePush(database *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input pushPreferencesInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if input.Subscription.Endpoint == "" || input.Subscription.Keys.P256dh == "" || input.Subscription.Keys.Auth == "" || input.UserID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "userId and subscription endpoint/keys are required"})
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		_, err := database.Exec(`
			INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, notify_leaderboard, notify_match_start, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(endpoint) DO UPDATE SET
				user_id = excluded.user_id,
				p256dh = excluded.p256dh,
				auth = excluded.auth,
				notify_leaderboard = excluded.notify_leaderboard,
				notify_match_start = excluded.notify_match_start,
				updated_at = excluded.updated_at
		`, input.UserID, input.Subscription.Endpoint, input.Subscription.Keys.P256dh, input.Subscription.Keys.Auth, input.NotifyLeaderboard, input.NotifyMatchStart, now, now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "subscribed"})
	}
}

func UpdatePushPreferences(database *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input pushPreferencesInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		endpoint := input.Endpoint
		if endpoint == "" {
			endpoint = input.Subscription.Endpoint
		}
		if endpoint == "" || input.UserID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint and userId are required"})
			return
		}

		result, err := database.Exec(`
			UPDATE push_subscriptions
			SET user_id = ?, notify_leaderboard = ?, notify_match_start = ?, updated_at = ?
			WHERE endpoint = ?
		`, input.UserID, input.NotifyLeaderboard, input.NotifyMatchStart, time.Now().UTC().Format(time.RFC3339), endpoint)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

func UnsubscribePush(database *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input unsubscribeInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if input.Endpoint == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint is required"})
			return
		}

		_, err := database.Exec("DELETE FROM push_subscriptions WHERE endpoint = ?", input.Endpoint)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "unsubscribed"})
	}
}

func SendTestNotification(push *PushService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input testNotificationInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if input.Subscription.Endpoint == "" || input.Subscription.Keys.P256dh == "" || input.Subscription.Keys.Auth == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "subscription endpoint and keys are required"})
			return
		}

		if push.publicKey == "" || push.privateKey == "" || push.subject == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "VAPID keys not configured"})
			return
		}

		data, err := json.Marshal(pushPayload{
			Title: "Test notification",
			Body:  "If you see this, push notifications are working!",
			URL:   "/",
			Type:  "test",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal payload"})
			return
		}

		resp, err := webpush.SendNotification(data, &webpush.Subscription{
			Endpoint: input.Subscription.Endpoint,
			Keys: webpush.Keys{
				P256dh: input.Subscription.Keys.P256dh,
				Auth:   input.Subscription.Keys.Auth,
			},
		}, &webpush.Options{
			Subscriber:      push.subject,
			VAPIDPublicKey:  push.publicKey,
			VAPIDPrivateKey: push.privateKey,
			RecordSize:      pushRecordSize,
			TTL:             30,
		})
		if resp != nil {
			resp.Body.Close()
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send notification"})
			return
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("push endpoint returned status %d", resp.StatusCode)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "test notification sent"})
	}
}

func (p *PushService) NotifyMatchStart(matchID string, homeTeamID, awayTeamID int64, homeTeam, awayTeam string) {
	template := randomPushTemplate(matchStartTemplates)
	p.sendToSubscriptions(`
		SELECT ps.id, ps.endpoint, ps.p256dh, ps.auth
		FROM push_subscriptions ps
		JOIN user_teams ut ON ut.user_id = ps.user_id
		WHERE ps.notify_match_start = 1 AND ut.team_id IN (?, ?)
		GROUP BY ps.id
	`, []any{homeTeamID, awayTeamID}, "match-start:"+matchID, pushPayload{
		Title: template.Title,
		Body:  fmt.Sprintf(template.Body, homeTeam, awayTeam),
		URL:   "/",
		Type:  "match-start",
	})
}

func (p *PushService) NotifyLeaderboardChange(userID, oldRank, newRank, points int) {
	eventKey := "leaderboard-rank:" + itoa(userID) + ":" + itoa(oldRank) + ":" + itoa(newRank) + ":" + itoa(points)
	template := randomPushTemplate(leaderboardTemplates)
	p.sendToSubscriptions(`
		SELECT id, endpoint, p256dh, auth
		FROM push_subscriptions
		WHERE notify_leaderboard = 1 AND user_id = ?
	`, []any{userID}, eventKey, pushPayload{
		Title: template.Title,
		Body:  fmt.Sprintf(template.Body, ordinal(newRank), ordinal(oldRank)),
		URL:   "/leaderboard",
		Type:  "leaderboard",
	})
}

func randomPushTemplate(templates []pushTemplate) pushTemplate {
	return templates[rand.Intn(len(templates))]
}

func (p *PushService) sendToSubscriptions(query string, args []any, eventKey string, payload pushPayload) {
	if p.publicKey == "" || p.privateKey == "" || p.subject == "" {
		return
	}

	rows, err := p.db.Query(query, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	for rows.Next() {
		var id int64
		var endpoint, p256dh, auth string
		if err := rows.Scan(&id, &endpoint, &p256dh, &auth); err != nil {
			continue
		}
		if p.deliveryExists(id, eventKey) {
			continue
		}

		resp, err := webpush.SendNotification(data, &webpush.Subscription{
			Endpoint: endpoint,
			Keys: webpush.Keys{
				P256dh: p256dh,
				Auth:   auth,
			},
		}, &webpush.Options{
			Subscriber:      p.subject,
			VAPIDPublicKey:  p.publicKey,
			VAPIDPrivateKey: p.privateKey,
			RecordSize:      pushRecordSize,
			TTL:             30,
		})
		if resp != nil {
			resp.Body.Close()
		}
		if isExpiredPushResponse(resp) {
			p.deleteSubscription(id)
			continue
		}
		if resp != nil && (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) {
			continue
		}
		if err != nil {
			continue
		}
		p.recordDelivery(id, eventKey)
	}
}

func (p *PushService) deliveryExists(subscriptionID int64, eventKey string) bool {
	var exists int
	err := p.db.QueryRow("SELECT 1 FROM notification_deliveries WHERE subscription_id = ? AND event_key = ?", subscriptionID, eventKey).Scan(&exists)
	return err == nil
}

func (p *PushService) recordDelivery(subscriptionID int64, eventKey string) {
	_, _ = p.db.Exec("INSERT OR IGNORE INTO notification_deliveries (subscription_id, event_key, sent_at) VALUES (?, ?, ?)", subscriptionID, eventKey, time.Now().UTC().Format(time.RFC3339))
}

func (p *PushService) deleteSubscription(subscriptionID int64) {
	_, _ = p.db.Exec("DELETE FROM push_subscriptions WHERE id = ?", subscriptionID)
}

func isExpiredPushResponse(resp *http.Response) bool {
	return resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound)
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return itoa(n) + suffix
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
