package handlers

import (
	"log"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/jeffasante/chatroom.go/middleware"
	"github.com/jeffasante/chatroom.go/models"
)

type Handler struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Handler {
	return &Handler{
		db: db,
	}
}

func (h *Handler) ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"error":   c.Query("error"),
		"success": c.Query("success"),
	})
}

func (h *Handler) ShowSignup(c *gin.Context) {
	c.HTML(http.StatusOK, "signup.html", gin.H{
		"error": c.Query("error"),
	})
}

func (h *Handler) HandleLogin(c *gin.Context) {
	emailOrUsername := c.PostForm("email") // This field can now accept email OR username
	password := c.PostForm("password")

	log.Printf("Login attempt for: %s", emailOrUsername)

	var user models.User
	// Try to find user by email OR username
	if err := h.db.Where("email = ? OR username = ?", emailOrUsername, emailOrUsername).First(&user).Error; err != nil {
		log.Printf("User not found: %s", emailOrUsername)
		c.Redirect(http.StatusFound, "/login?error=invalid")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("Invalid password for user: %s", emailOrUsername)
		c.Redirect(http.StatusFound, "/login?error=invalid")
		return
	}

	// Create session token and store in shared session map
	token := h.generateToken()
	middleware.SetSession(token, user.ID)
	
	log.Printf("Login successful for user %s (ID: %d), token: %s", user.Username, user.ID, token)
	log.Printf("Sessions map now contains %d sessions", len(middleware.Sessions))

	// Set cookie
	c.SetCookie("token", token, 3600*24, "/", "", false, true)
	c.Redirect(http.StatusFound, "/dashboard")
}

func (h *Handler) HandleSignup(c *gin.Context) {
	username := c.PostForm("username")
	email := c.PostForm("email")
	password := c.PostForm("password")

	// Validate input
	if username == "" || email == "" || password == "" {
		c.Redirect(http.StatusFound, "/signup?error=missing")
		return
	}

	// Check if user exists
	var existingUser models.User
	if err := h.db.Where("email = ? OR username = ?", email, username).First(&existingUser).Error; err == nil {
		c.Redirect(http.StatusFound, "/signup?error=exists")
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.Redirect(http.StatusFound, "/signup?error=server")
		return
	}

	// Create user
	user := models.User{
		Username: username,
		Email:    email,
		Password: string(hashedPassword),
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.Redirect(http.StatusFound, "/signup?error=server")
		return
	}

	c.Redirect(http.StatusFound, "/login?success=created")
}

func (h *Handler) HandleLogout(c *gin.Context) {
	token, _ := c.Cookie("token")
	middleware.DeleteSession(token)
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func (h *Handler) generateToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (h *Handler) GetCurrentUser(c *gin.Context) *models.User {
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(models.User); ok {
			return &u
		}
	}
	return nil
}