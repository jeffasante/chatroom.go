package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/jeffasante/chatroom.go/models"
)

// Global session storage - shared between middleware and handlers
var Sessions = make(map[string]uint)

func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil || token == "" {
			log.Printf("No token found in cookies")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		log.Printf("Checking token: %s", token)
		log.Printf("Sessions map contains %d sessions", len(Sessions))

		userID, exists := Sessions[token]
		if !exists {
			log.Printf("Token not found in sessions map")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		log.Printf("Found user ID %d for token", userID)

		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			log.Printf("User %d not found in database", userID)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		log.Printf("User %s authenticated successfully", user.Username)
		c.Set("user", user)
		c.Next()
	}
}

// Helper function to sync sessions between middleware and handlers
func SetSession(token string, userID uint) {
	Sessions[token] = userID
	log.Printf("Session set: token=%s, userID=%d", token, userID)
}

func DeleteSession(token string) {
	delete(Sessions, token)
	log.Printf("Session deleted: token=%s", token)
}

func GetUserByToken(db *gorm.DB, token string) *models.User {
	userID, exists := Sessions[token]
	if !exists {
		return nil
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		return nil
	}

	return &user
}