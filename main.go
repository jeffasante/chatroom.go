package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/jeffasante/chatroom.go/handlers"
	"github.com/jeffasante/chatroom.go/middleware"
	"github.com/jeffasante/chatroom.go/models"
)

var db *gorm.DB

func main() {
	// Initialize the database connection
	var err error
	db, err = gorm.Open(sqlite.Open("chatroom.db"), &gorm.Config{})

	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto migrate the schema
	db.AutoMigrate(&models.User{}, &models.Chatroom{}, &models.Message{}, &models.Membership{})

	// Initialize handlers with database
	h := handlers.New(db)

	// Initialize WebSocket hub
	hub := handlers.NewHub(db)
	go hub.Run()


	// Create a new Gin router
	router := gin.Default()

	// Load HTML templates
	router.LoadHTMLGlob("templates/*")

	// Serve static files
	router.Static("/static", "./static")

	// Auth routes
	router.GET("/", redirectToDashboard)
	router.GET("/login", h.ShowLogin)
	router.POST("/login", h.HandleLogin)
	router.GET("/signup", h.ShowSignup)
	router.POST("/signup", h.HandleSignup)
	router.POST("/logout", h.HandleLogout)

	// Authorized routes (require authentication)
	authorized := router.Group("/")
	authorized.Use(middleware.AuthMiddleware(db))
	{
		authorized.GET("/dashboard", h.ShowDashboard)
		authorized.POST("/create-room", h.CreateRoom)
		authorized.POST("/join-room", h.JoinRoom)
		authorized.GET("/room/:code", h.ShowRoom)
		
		// WebSocket and API routes
		authorized.GET("/ws/:code", h.HandleWebSocket(hub))
		authorized.GET("/api/messages/:code", h.GetMessages)

		// Room management routes
		authorized.POST("/api/room/:code/update", h.UpdateRoom)
		authorized.DELETE("/api/room/:code", h.DeleteRoom)
		authorized.GET("/api/room/:code/members", h.GetRoomMembers)
	}

	log.Println("Server started on :8080")
	router.Run(":8080")

}

func redirectToDashboard(c *gin.Context) {
	token, err := c.Cookie("token")
	if err != nil || token == "" {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	
	// Check if session exists in our shared session storage
	if _, exists := middleware.Sessions[token]; !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	
	c.Redirect(http.StatusSeeOther, "/dashboard")
}