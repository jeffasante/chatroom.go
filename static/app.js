// Modal functions
function showCreateModal() {
    document.getElementById('createModal').style.display = 'block';
    document.getElementById('joinModal').style.display = 'none';
}

function showJoinModal() {
    document.getElementById('joinModal').style.display = 'block';
    document.getElementById('createModal').style.display = 'none';
}

function closeModals() {
    document.getElementById('createModal').style.display = 'none';
    document.getElementById('joinModal').style.display = 'none';
}

// Close modal when clicking outside
window.onclick = function(event) {
    const createModal = document.getElementById('createModal');
    const joinModal = document.getElementById('joinModal');
    
    if (event.target === createModal) {
        createModal.style.display = 'none';
    }
    if (event.target === joinModal) {
        joinModal.style.display = 'none';
    }
}

// Create room form
document.addEventListener('DOMContentLoaded', function() {
    const createForm = document.getElementById('createForm');
    const joinForm = document.getElementById('joinForm');
    
    if (createForm) {
        createForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const name = document.getElementById('roomName').value;
            const password = document.getElementById('roomPassword').value;
            
            try {
                const response = await fetch('/create-room', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    },
                    body: `name=${encodeURIComponent(name)}&password=${encodeURIComponent(password)}`
                });
                
                const result = await response.json();
                
                if (result.success) {
                    alert(`Room created! Code: ${result.code}`);
                    location.reload();
                } else {
                    alert('Error: ' + result.error);
                }
            } catch (error) {
                alert('Network error occurred');
            }
        });
    }
    
    if (joinForm) {
        joinForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const code = document.getElementById('joinCode').value;
            const password = document.getElementById('joinPassword').value;
            
            try {
                const response = await fetch('/join-room', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    },
                    body: `code=${encodeURIComponent(code)}&password=${encodeURIComponent(password)}`
                });
                
                const result = await response.json();
                
                if (result.success) {
                    alert('Successfully joined room!');
                    location.reload();
                } else {
                    alert('Error: ' + result.error);
                }
            } catch (error) {
                alert('Network error occurred');
            }
        });
    }
});

// WebSocket Chat Implementation
let socket = null;
let roomCode = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;
let displayedMessages = new Set(); // Track displayed messages to prevent duplicates

function initializeChat(code) {
    roomCode = code;
    connectWebSocket();
    
    // Handle Enter key in message input
    const messageInput = document.getElementById('messageInput');
    if (messageInput) {
        messageInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                sendMessage();
            }
        });
    }
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/${roomCode}`;
    
    console.log('Attempting to connect to:', wsUrl);
    updateConnectionStatus('Connecting...', 'warning');
    
    socket = new WebSocket(wsUrl);
    
    socket.onopen = function(event) {
        console.log('Connected to chat');
        reconnectAttempts = 0;
        updateConnectionStatus('Connected', 'success');
    };
    
    socket.onmessage = function(event) {
        try {
            const message = JSON.parse(event.data);
            displayMessage(message);
        } catch (error) {
            console.error('Error parsing message:', error);
        }
    };
    
    socket.onclose = function(event) {
        console.log('Disconnected from chat', event);
        updateConnectionStatus('Disconnected', 'error');
        
        // Attempt to reconnect
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            console.log(`Attempting to reconnect... (${reconnectAttempts}/${maxReconnectAttempts})`);
            updateConnectionStatus(`Reconnecting... (${reconnectAttempts}/${maxReconnectAttempts})`, 'warning');
            setTimeout(connectWebSocket, 3000);
        } else {
            updateConnectionStatus('Connection failed', 'error');
        }
    };
    
    socket.onerror = function(error) {
        console.error('WebSocket error:', error);
        updateConnectionStatus('Connection error', 'error');
    };
}

function updateConnectionStatus(status, type) {
    // Update the status in the right panel
    const statusElement = document.getElementById('connectionStatus');
    if (statusElement) {
        statusElement.textContent = status;
        statusElement.className = `connection-status-${type}`;
    }
    
    console.log('Connection status:', status);
}

function sendMessage() {
    const input = document.getElementById('messageInput');
    const content = input.value.trim();
    
    if (content && socket && socket.readyState === WebSocket.OPEN) {
        const message = {
            type: 'message',
            content: content
        };
        
        socket.send(JSON.stringify(message));
        input.value = '';
        
        // Don't display the message here - let the WebSocket broadcast handle it
        // This prevents duplicate messages
    } else if (!socket || socket.readyState !== WebSocket.OPEN) {
        showConnectionStatus('Not connected', 'error');
    }
}

function displayMessage(message) {
    const messagesContainer = document.getElementById('messages');
    if (!messagesContainer) return;
    
    // Create a unique identifier for the message to prevent duplicates
    const messageId = message.message_id || `${message.user_id}-${message.timestamp}-${message.content}`;
    
    // Check if we've already displayed this message
    if (displayedMessages.has(messageId)) {
        console.log('Duplicate message detected, skipping:', messageId);
        return;
    }
    
    // Add to displayed messages set
    displayedMessages.add(messageId);
    
    const messageElement = document.createElement('div');
    messageElement.className = 'message';
    messageElement.setAttribute('data-message-id', messageId);
    
    // Format timestamp
    const timestamp = new Date(message.timestamp);
    const timeString = timestamp.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
    
    // Handle different message types
    if (message.type === 'user_joined' || message.type === 'user_left') {
        messageElement.classList.add('system-message');
        messageElement.innerHTML = `
            <div class="message-header">
                <span class="system-label">System</span>
                <span class="message-time">${timeString}</span>
            </div>
            <div class="message-content system-content">${escapeHtml(message.content)}</div>
        `;
    } else {
        messageElement.innerHTML = `
            <div class="message-header">
                ${escapeHtml(message.username)}
                <span class="message-time">${timeString}</span>
            </div>
            <div class="message-content">${escapeHtml(message.content)}</div>
        `;
    }
    
    messagesContainer.appendChild(messageElement);
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
    
    console.log('Displayed message:', messageId);
}

function showConnectionStatus(status, type) {
    // Remove existing status
    const existingStatus = document.querySelector('.connection-status');
    if (existingStatus) {
        existingStatus.remove();
    }
    
    // Create status element
    const statusElement = document.createElement('div');
    statusElement.className = `connection-status ${type}`;
    statusElement.textContent = status;
    
    // Add to page
    const chatContainer = document.querySelector('.chat-container');
    if (chatContainer) {
        chatContainer.insertBefore(statusElement, chatContainer.firstChild);
        
        // Auto-remove success messages
        if (type === 'success') {
            setTimeout(() => {
                if (statusElement.parentNode) {
                    statusElement.remove();
                }
            }, 3000);
        }
    }
    
    // Also update the right panel status
    updateConnectionStatus(status, type);
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Settings functions
function showSettings() {
    const settingsModal = document.getElementById('settingsModal');
    if (settingsModal) {
        settingsModal.style.display = 'block';
        loadRoomMembers(); // Load current members when opening settings
    }
}

async function updateRoom() {
    const newName = document.getElementById('roomNameEdit').value.trim();
    if (!newName) {
        alert('Room name cannot be empty');
        return;
    }
    
    const roomCode = getRoomCodeFromUrl();
    if (!roomCode) {
        alert('Could not determine room code');
        return;
    }
    
    try {
        const response = await fetch(`/api/room/${roomCode}/update`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: `name=${encodeURIComponent(newName)}`
        });
        
        const result = await response.json();
        
        if (result.success) {
            alert('Room name updated successfully!');
            // Update the room title on the page
            const roomTitle = document.querySelector('.room-title');
            if (roomTitle) {
                roomTitle.textContent = `${newName} [${roomCode}]`;
            }
            document.getElementById('settingsModal').style.display = 'none';
        } else {
            alert('Error: ' + result.error);
        }
    } catch (error) {
        alert('Network error occurred');
        console.error('Error updating room:', error);
    }
}

async function deleteRoom() {
    if (!confirm('Are you sure you want to delete this room? This action cannot be undone and will remove all messages and members.')) {
        return;
    }
    
    const roomCode = getRoomCodeFromUrl();
    if (!roomCode) {
        alert('Could not determine room code');
        return;
    }
    
    try {
        const response = await fetch(`/api/room/${roomCode}`, {
            method: 'DELETE'
        });
        
        const result = await response.json();
        
        if (result.success) {
            alert('Room deleted successfully');
            // Redirect to dashboard
            window.location.href = '/dashboard';
        } else {
            alert('Error: ' + result.error);
        }
    } catch (error) {
        alert('Network error occurred');
        console.error('Error deleting room:', error);
    }
}

async function loadRoomMembers() {
    const roomCode = getRoomCodeFromUrl();
    if (!roomCode) return;
    
    try {
        const response = await fetch(`/api/room/${roomCode}/members`);
        const result = await response.json();
        
        if (result.members) {
            updateMembersDisplay(result.members);
        }
    } catch (error) {
        console.error('Error loading members:', error);
    }
}

function updateMembersDisplay(members) {
    const membersContainer = document.querySelector('#settingsModal .members-list');
    if (!membersContainer) return;
    
    membersContainer.innerHTML = '';
    
    members.forEach((member, index) => {
        const memberDiv = document.createElement('div');
        memberDiv.style.marginBottom = '5px';
        
        const ownerBadge = member.is_owner ? ' (Owner)' : '';
        memberDiv.textContent = `${index + 1}: ${member.username}${ownerBadge}`;
        
        if (member.is_owner) {
            memberDiv.style.fontWeight = 'bold';
        }
        
        membersContainer.appendChild(memberDiv);
    });
}

function getRoomCodeFromUrl() {
    const pathParts = window.location.pathname.split('/');
    if (pathParts[1] === 'room' && pathParts[2]) {
        return pathParts[2];
    }
    return null;
}

// Auto-scroll to bottom on page load
document.addEventListener('DOMContentLoaded', function() {
    const messagesContainer = document.getElementById('messages');
    if (messagesContainer) {
        // Track existing messages on page load to prevent duplicates
        const existingMessages = messagesContainer.querySelectorAll('.message');
        existingMessages.forEach((msgElement, index) => {
            // Create a unique ID for existing messages based on their content and position
            const content = msgElement.querySelector('.message-content')?.textContent || '';
            const username = msgElement.querySelector('.message-header')?.textContent || '';
            const uniqueId = `existing-${index}-${content.substring(0, 20)}`;
            displayedMessages.add(uniqueId);
            msgElement.setAttribute('data-message-id', uniqueId);
        });
        
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
        
        // Get room code from URL and initialize chat
        const pathParts = window.location.pathname.split('/');
        if (pathParts[1] === 'room' && pathParts[2]) {
            initializeChat(pathParts[2]);
        }
    }
});

// Online users functionality (for future enhancement)
function updateOnlineUsers(users) {
    const onlineList = document.getElementById('onlineUsers');
    if (onlineList && users) {
        onlineList.innerHTML = users.map(user => 
            `<div class="online-user">${escapeHtml(user.username)}</div>`
        ).join('');
    }
}

// Typing indicator functionality (for future enhancement)
let typingTimer;
function handleTyping() {
    if (socket && socket.readyState === WebSocket.OPEN) {
        clearTimeout(typingTimer);
        
        // Send typing start
        socket.send(JSON.stringify({
            type: 'typing_start'
        }));
        
        // Clear typing after 2 seconds
        typingTimer = setTimeout(() => {
            socket.send(JSON.stringify({
                type: 'typing_stop'
            }));
        }, 2000);
    }
}