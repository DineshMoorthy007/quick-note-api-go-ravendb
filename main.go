package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"quicknote/api/db"
	"quicknote/api/handlers"
)

func main() {
	store := db.InitDB("http://localhost:8080", "quicknote_db")
	if store == nil {
		log.Fatal("failed to initialize RavenDB document store")
	}
	defer store.Close()

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:5173"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead},
		AllowHeaders: []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		MaxAge:       12 * time.Hour,
	}))

	router.POST("/api/auth/login", func(c *gin.Context) {
		handlers.Login(c, store)
	})

	router.POST("/api/auth/register", func(c *gin.Context) {
		handlers.Register(c, store)
	})

	router.GET("/api/notes", func(c *gin.Context) {
		handlers.GetNotesByUser(c, store)
	})

	router.POST("/api/notes", func(c *gin.Context) {
		handlers.CreateNote(c, store)
	})

	router.PUT("/api/notes/:id", func(c *gin.Context) {
		handlers.UpdateNote(c, store)
	})

	router.PUT("/api/notes/:id/pin", func(c *gin.Context) {
		handlers.TogglePinNote(c, store)
	})

	router.DELETE("/api/notes/:id", func(c *gin.Context) {
		handlers.DeleteNote(c, store)
	})

	if err := router.Run(":5000"); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
