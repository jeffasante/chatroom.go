package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jeffasante/chatroom.go/models"
)

// Update room name
func (h *Handler) UpdateRoom(c *gin.Context) {
	user := h.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	code := c.Param("code")
	newName := c.PostForm("name")

	if newName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room name is required"})
		return
	}

	// Get chatroom
	var chatroom models.Chatroom
	if err := h.db.Where("code = ?", code).First(&chatroom).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	// Check if user is the owner
	if chatroom.OwnerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only room owner can update room"})
		return
	}

	// Update room name
	if err := h.db.Model(&chatroom).Update("name", newName).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Room name updated successfully",
		"name":    newName,
	})
}

// Delete room and all associated data
func (h *Handler) DeleteRoom(c *gin.Context) {
	user := h.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	code := c.Param("code")

	// Get chatroom
	var chatroom models.Chatroom
	if err := h.db.Where("code = ?", code).First(&chatroom).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	// Check if user is the owner
	if chatroom.OwnerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only room owner can delete room"})
		return
	}

	// Begin transaction to ensure all data is deleted together
	tx := h.db.Begin()

	// Delete all messages in the room
	if err := tx.Where("chatroom_id = ?", chatroom.ID).Delete(&models.Message{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete messages"})
		return
	}

	// Delete all memberships
	if err := tx.Where("chatroom_id = ?", chatroom.ID).Delete(&models.Membership{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete memberships"})
		return
	}

	// Delete the chatroom
	if err := tx.Delete(&chatroom).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete room"})
		return
	}

	// Commit transaction
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Room deleted successfully",
	})
}

// Get room members list
func (h *Handler) GetRoomMembers(c *gin.Context) {
	user := h.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	code := c.Param("code")

	// Get chatroom
	var chatroom models.Chatroom
	if err := h.db.Where("code = ?", code).First(&chatroom).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	// Check if user is a member
	if !chatroom.IsMember(h.db, user.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this room"})
		return
	}

	// Get all memberships with user data
	var memberships []models.Membership
	if err := h.db.Where("chatroom_id = ?", chatroom.ID).
		Preload("User").
		Order("joined_at ASC").
		Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get members"})
		return
	}

	// Format response
	type MemberInfo struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		IsOwner  bool   `json:"is_owner"`
		JoinedAt string `json:"joined_at"`
	}

	var members []MemberInfo
	for _, membership := range memberships {
		members = append(members, MemberInfo{
			ID:       membership.User.ID,
			Username: membership.User.Username,
			IsOwner:  membership.User.ID == chatroom.OwnerID,
			JoinedAt: membership.JoinedAt.Format("2006-01-02 15:04"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"members": members,
		"count":   len(members),
	})
}