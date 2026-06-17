package main

import (
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"worldcup-stake/db"
	"worldcup-stake/frontend"
	"worldcup-stake/handlers"
)

func main() {
	database, err := db.Init()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if err := db.Seed(database); err != nil {
		log.Fatal(err)
	}

	pushService := handlers.NewPushService(database)
	providers := []handlers.MatchProvider{handlers.NewFIFAProvider()}
	if apiKey := os.Getenv("FOOTBALL_DATA_API_KEY"); apiKey != "" {
		providers = append(providers, handlers.NewFootballDataProvider(apiKey))
	}
	syncer := handlers.NewSyncer(database, providers, pushService)

	go func() {
		syncer.Sync()
		if err := syncer.RefreshTeamRankings(); err != nil {
			log.Printf("Rankings: failed to refresh team rankings: %v", err)
		}
		for {
			interval := 1 * time.Hour
			if syncer.HasLiveMatches() {
				interval = 1 * time.Minute
			} else if next := syncer.TimeUntilNextMatch(); next < interval {
				interval = next
			}
			time.Sleep(interval)
			syncer.Sync()
		}
	}()

	sub, err := fs.Sub(frontend.Static, "dist")
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/users", handlers.GetUsers(database))
		api.GET("/matches", handlers.GetMatches(database))
		api.GET("/leaderboard", handlers.GetLeaderboard(database))
		api.GET("/push/vapid-public-key", handlers.GetVAPIDPublicKey(pushService))
		api.POST("/push/subscriptions", handlers.SubscribePush(database))
		api.PATCH("/push/subscriptions/preferences", handlers.UpdatePushPreferences(database))
		api.DELETE("/push/subscriptions", handlers.UnsubscribePush(database))
		api.POST("/push/test", handlers.SendTestNotification(pushService))
		api.POST("/sync", func(c *gin.Context) {
			go syncer.Sync()
			c.JSON(http.StatusOK, gin.H{"status": "sync triggered"})
		})
	}

	// Serve static assets directly; all other routes get index.html for SPA routing
	r.GET("/assets/*filepath", func(c *gin.Context) {
		c.FileFromFS("assets/"+c.Param("filepath"), http.FS(sub))
	})
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = path[1:]
		}
		if data, err := fs.ReadFile(sub, path); err == nil {
			ext := filepath.Ext(path)
			if mime := mime.TypeByExtension(ext); mime != "" {
				c.Data(http.StatusOK, mime, data)
			} else {
				c.Data(http.StatusOK, "application/octet-stream", data)
			}
			return
		}
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
