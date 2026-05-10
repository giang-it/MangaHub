package websocket

import (
	"fmt"
	"sync"
)

type ChatMessage struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Message    string `json:"message"`
	Room       string `json:"room"`
	Timestamp  int64  `json:"timestamp"`
	IsPrivate  bool   `json:"is_private"`
	TargetUser string `json:"target_user,omitempty"`
}

type Hub struct {
	sync.RWMutex
	Rooms      map[string]map[*Client]bool
	Clients    map[string]*Client
	History    map[string][]ChatMessage
	Broadcast  chan ChatMessage
	Register   chan *Client
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Rooms:      make(map[string]map[*Client]bool),
		Clients:    make(map[string]*Client),
		History:    make(map[string][]ChatMessage),
		Broadcast:  make(chan ChatMessage),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Lock()
			h.Clients[client.Username] = client
			if h.Rooms[client.Room] == nil {
				h.Rooms[client.Room] = make(map[*Client]bool)
			}
			h.Rooms[client.Room][client] = true
			h.Unlock()
			h.broadcastSysMsg(client.Room, fmt.Sprintf("%s joined the chat", client.Username))

		case client := <-h.Unregister:
			h.Lock()
			if _, ok := h.Rooms[client.Room][client]; ok {
				delete(h.Rooms[client.Room], client)
				delete(h.Clients, client.Username)
				close(client.Send)
				if len(h.Rooms[client.Room]) == 0 {
					delete(h.Rooms, client.Room)
				} else {
					// We unlock before broadcasting to avoid deadlock
					h.Unlock()
					h.broadcastSysMsg(client.Room, fmt.Sprintf("%s left the chat", client.Username))
					h.Lock()
				}
			}
			h.Unlock()

		case message := <-h.Broadcast:
			h.RLock()
			if message.IsPrivate {
				var target, sender *Client
				if t, ok := h.Clients[message.TargetUser]; ok { target = t }
				if s, ok := h.Clients[message.Username]; ok { sender = s }
				h.RUnlock()
				
				if target != nil {
					h.sendToClient(target, message)
				}
				if sender != nil && message.Username != message.TargetUser {
					h.sendToClient(sender, message)
				}
				continue
			}

			// Save to history
			h.RUnlock()
			h.Lock()
			if len(h.History[message.Room]) >= 50 {
				h.History[message.Room] = h.History[message.Room][1:]
			}
			h.History[message.Room] = append(h.History[message.Room], message)
			roomClients := make([]*Client, 0, len(h.Rooms[message.Room]))
			for client := range h.Rooms[message.Room] {
				roomClients = append(roomClients, client)
			}
			h.Unlock()

			for _, client := range roomClients {
				h.sendToClient(client, message)
			}
		}
	}
}

func (h *Hub) sendToClient(client *Client, message ChatMessage) {
	select {
	case client.Send <- message:
	default:
		h.Lock()
		close(client.Send)
		delete(h.Rooms[client.Room], client)
		delete(h.Clients, client.Username)
		h.Unlock()
	}
}

func (h *Hub) broadcastSysMsg(room, text string) {
	msg := ChatMessage{
		Username:  "SYSTEM",
		Message:   text,
		Room:      room,
		Timestamp: 0,
	}
	h.RLock()
	roomClients := make([]*Client, 0, len(h.Rooms[room]))
	for client := range h.Rooms[room] {
		roomClients = append(roomClients, client)
	}
	h.RUnlock()
	for _, client := range roomClients {
		client.Send <- msg
	}
}
