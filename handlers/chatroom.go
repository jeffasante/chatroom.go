package handlers

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/jeffasante/chatroom.go/models"
)

func (h *Handler) ShowDashboard(c *gin.Context) {
	user := h.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	chatrooms, err := user.GetChatrooms(h.db)  // Fixed: added h.db parameter
	if err != nil {
		chatrooms = []models.Chatroom{}
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"user":      user,
		"chatrooms": chatrooms,
	})
}

func (h *Handler) CreateRoom(c *gin.Context) {
	user := h.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	name := c.PostForm("name")
	password := c.PostForm("password")

	if name == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and password required"})
		return
	}

	// Generate unique room code
	code := h.generateRoomCode()
	for {
		var existing models.Chatroom
		if err := h.db.Where("code = ?", code).First(&existing).Error; err != nil {
			break // Code is unique
		}
		code = h.generateRoomCode()
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}

	// Create chatroom
	chatroom := models.Chatroom{
		Code:     code,
		Name:     name,
		Password: string(hashedPassword),
		OwnerID:  user.ID,
	}

	if err := h.db.Create(&chatroom).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create room"})
		return
	}

	// Add owner as member
	membership := models.Membership{
		UserID:     user.ID,
		ChatroomID: chatroom.ID,
		JoinedAt:   time.Now(),
	}
	h.db.Create(&membership)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"code":    code,
	})
}

func (h *Handler) JoinRoom(c *gin.Context) {
	user := h.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	code := c.PostForm("code")
	password := c.PostForm("password")

	var chatroom models.Chatroom
	if err := h.db.Where("code = ?", code).First(&chatroom).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(chatroom.Password), []byte(password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}

	// Check if already a member
	if chatroom.IsMember(h.db, user.ID) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "already a member",
			"code":    code,
		})
		return
	}

	// Add as member
	membership := models.Membership{
		UserID:     user.ID,
		ChatroomID: chatroom.ID,
		JoinedAt:   time.Now(),
	}

	if err := h.db.Create(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"code":    code,
	})
}

func (h *Handler) ShowRoom(c *gin.Context) {
	user := h.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	code := c.Param("code")
	var chatroom models.Chatroom
	if err := h.db.Where("code = ?", code).Preload("Owner").First(&chatroom).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error": "Room not found",
		})
		return
	}

	// Check if user is a member - Fixed: added h.db parameter
	if !chatroom.IsMember(h.db, user.ID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"error": "You are not a member of this room",
		})
		return
	}

	// Get recent messages - Fixed: added h.db parameter
	messages, err := chatroom.GetMessages(h.db, 50)
	if err != nil {
		messages = []models.Message{}
	}

	c.HTML(http.StatusOK, "room.html", gin.H{
		"user":     user,
		"chatroom": chatroom,
		"messages": messages,
	})
}

func (h *Handler) generateRoomCode() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	
	code := make([]byte, 8)
	for i := range code {
		code[i] = chars[rand.Intn(len(chars))]
	}
	return string(code)
}