package models

import (
	"time"
	"gorm.io/gorm"
)

type User struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	Username string `json:"username" gorm:"unique;not null"`
	Email    string `json:"email" gorm:"unique;not null"`
	Password string `json:"-" gorm:"not null"`
	CreatedAt time.Time
}

type Chatroom struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Code        string `json:"code" gorm:"unique;not null;size:8"`
	Name        string `json:"name" gorm:"not null"`
	Password    string `json:"-" gorm:"not null"`
	OwnerID     uint   `json:"owner_id"`
	CreatedAt   time.Time
	
	// Relationships
	Owner       User         `gorm:"foreignKey:OwnerID"`
	Messages    []Message    `gorm:"foreignKey:ChatroomID"`
	Memberships []Membership `gorm:"foreignKey:ChatroomID"`
}

type Message struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Content    string    `json:"content" gorm:"not null;type:text"`
	UserID     uint      `json:"user_id"`
	ChatroomID uint      `json:"chatroom_id"`
	CreatedAt  time.Time `json:"created_at"`
	
	// Relationships
	User     User     `gorm:"foreignKey:UserID"`
	Chatroom Chatroom `gorm:"foreignKey:ChatroomID"`
}

type Membership struct {
	ID         uint `gorm:"primaryKey"`
	UserID     uint `gorm:"not null"`
	ChatroomID uint `gorm:"not null"`
	JoinedAt   time.Time
	
	// Relationships
	User     User     `gorm:"foreignKey:UserID"`
	Chatroom Chatroom `gorm:"foreignKey:ChatroomID"`
}

// Helper methods - all now properly accept database parameter
func (u *User) GetChatrooms(db *gorm.DB) ([]Chatroom, error) {
	var memberships []Membership
	err := db.Where("user_id = ?", u.ID).Find(&memberships).Error
	if err != nil {
		return nil, err
	}
	
	var chatroomIDs []uint
	for _, membership := range memberships {
		chatroomIDs = append(chatroomIDs, membership.ChatroomID)
	}
	
	if len(chatroomIDs) == 0 {
		return []Chatroom{}, nil
	}
	
	var chatrooms []Chatroom
	err = db.Where("id IN ?", chatroomIDs).Find(&chatrooms).Error
	return chatrooms, err
}

func (c *Chatroom) IsMember(db *gorm.DB, userID uint) bool {
	var count int64
	db.Model(&Membership{}).Where("user_id = ? AND chatroom_id = ?", userID, c.ID).Count(&count)
	return count > 0
}

func (c *Chatroom) GetMessages(db *gorm.DB, limit int) ([]Message, error) {
	var messages []Message
	err := db.Where("chatroom_id = ?", c.ID).
		Preload("User").
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}