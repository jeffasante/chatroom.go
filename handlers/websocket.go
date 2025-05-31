package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/jeffasante/chatroom.go/middleware"
	"github.com/jeffasante/chatroom.go/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	chatrooms  map[uint]map[*Client]bool // chatroom_id -> clients
	db         *gorm.DB
}

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan *Message
	user       *models.User
	chatroomID uint
}

type Message struct {
	Type       string    `json:"type"`
	Content    string    `json:"content"`
	UserID     uint      `json:"user_id"`
	Username   string    `json:"username"`
	ChatroomID uint      `json:"chatroom_id"`
	Timestamp  time.Time `json:"timestamp"`
	MessageID  uint      `json:"message_id,omitempty"`
}

func NewHub(db *gorm.DB) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		chatrooms:  make(map[uint]map[*Client]bool),
		db:         db,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			
			// Add to chatroom
			if h.chatrooms[client.chatroomID] == nil {
				h.chatrooms[client.chatroomID] = make(map[*Client]bool)
			}
			h.chatrooms[client.chatroomID][client] = true
			
			log.Printf("User %s joined chatroom %d", client.user.Username, client.chatroomID)
			
			// Send user joined message to chatroom
			joinMessage := &Message{
				Type:       "user_joined",
				Content:    client.user.Username + " joined the chat",
				Username:   "System",
				ChatroomID: client.chatroomID,
				Timestamp:  time.Now(),
			}
			h.broadcastToChatroom(client.chatroomID, joinMessage)

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				
				// Remove from chatroom
				if clients, exists := h.chatrooms[client.chatroomID]; exists {
					delete(clients, client)
					if len(clients) == 0 {
						delete(h.chatrooms, client.chatroomID)
					}
				}
				
				close(client.send)
				
				log.Printf("User %s left chatroom %d", client.user.Username, client.chatroomID)
				
				// Send user left message to chatroom
				leftMessage := &Message{
					Type:       "user_left",
					Content:    client.user.Username + " left the chat",
					Username:   "System",
					ChatroomID: client.chatroomID,
					Timestamp:  time.Now(),
				}
				h.broadcastToChatroom(client.chatroomID, leftMessage)
			}

		case message := <-h.broadcast:
			// Save message to database
			if message.Type == "message" {
				dbMessage := models.Message{
					Content:    message.Content,
					UserID:     message.UserID,
					ChatroomID: message.ChatroomID,
					CreatedAt:  message.Timestamp,
				}
				
				if err := h.db.Create(&dbMessage).Error; err != nil {
					log.Printf("Failed to save message: %v", err)
				} else {
					message.MessageID = dbMessage.ID
				}
			}
			
			// Broadcast to chatroom
			h.broadcastToChatroom(message.ChatroomID, message)
		}
	}
}

func (h *Hub) broadcastToChatroom(chatroomID uint, message *Message) {
	if clients, exists := h.chatrooms[chatroomID]; exists {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(h.clients, client)
				delete(clients, client)
			}
		}
	}
}

func (h *Handler) HandleWebSocket(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get chatroom code from URL
		code := c.Param("code")
		
		// Verify user authentication
		token, err := c.Cookie("token")
		if err != nil || token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		
		user := middleware.GetUserByToken(h.db, token)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}
		
		// Get chatroom
		var chatroom models.Chatroom
		if err := h.db.Where("code = ?", code).First(&chatroom).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
			return
		}
		
		// Verify user is a member
		if !chatroom.IsMember(h.db, user.ID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
			return
		}
		
		// Upgrade to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		
		// Create client
		client := &Client{
			hub:        hub,
			conn:       conn,
			send:       make(chan *Message, 256),
			user:       user,
			chatroomID: chatroom.ID,
		}
		
		// Register client
		hub.register <- client
		
		// Start goroutines
		go client.writePump()
		go client.readPump()
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		
		var incomingMessage struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		
		if err := json.Unmarshal(messageBytes, &incomingMessage); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}
		
		// Create message to broadcast
		message := &Message{
			Type:       incomingMessage.Type,
			Content:    incomingMessage.Content,
			UserID:     c.user.ID,
			Username:   c.user.Username,
			ChatroomID: c.chatroomID,
			Timestamp:  time.Now(),
		}
		
		c.hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			messageBytes, err := json.Marshal(message)
			if err != nil {
				log.Printf("JSON marshal error: %v", err)
				continue
			}
			
			if err := c.conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
			
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// API endpoint to get recent messages
func (h *Handler) GetMessages(c *gin.Context) {
	token, err := c.Cookie("token")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user := middleware.GetUserByToken(h.db, token)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}
	
	code := c.Param("code")
	var chatroom models.Chatroom
	if err := h.db.Where("code = ?", code).First(&chatroom).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}
	
	if !chatroom.IsMember(h.db, user.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	
	// Get query parameters
	limitStr := c.DefaultQuery("limit", "50")
	afterIDStr := c.Query("after_id")
	
	limit, _ := strconv.Atoi(limitStr)
	if limit > 100 {
		limit = 100
	}
	
	var messages []models.Message
	query := h.db.Where("chatroom_id = ?", chatroom.ID).
		Preload("User").
		Order("created_at ASC").
		Limit(limit)
	
	if afterIDStr != "" {
		afterID, _ := strconv.ParseUint(afterIDStr, 10, 32)
		query = query.Where("id > ?", afterID)
	}
	
	if err := query.Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch messages"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"count":    len(messages),
	})
}