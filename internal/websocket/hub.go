package websocket

import (
	"fmt"
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
	Rooms      map[string]map[*Client]bool
	Clients    map[string]*Client
	Broadcast  chan ChatMessage
	Register   chan *Client
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Rooms:      make(map[string]map[*Client]bool),
		Clients:    make(map[string]*Client),
		Broadcast:  make(chan ChatMessage),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client.Username] = client
			if h.Rooms[client.Room] == nil {
				h.Rooms[client.Room] = make(map[*Client]bool)
			}
			h.Rooms[client.Room][client] = true
			h.broadcastSysMsg(client.Room, fmt.Sprintf("%s joined the chat", client.Username))

		case client := <-h.Unregister:
			if _, ok := h.Rooms[client.Room][client]; ok {
				delete(h.Rooms[client.Room], client)
				delete(h.Clients, client.Username)
				close(client.Send)
				if len(h.Rooms[client.Room]) == 0 {
					delete(h.Rooms, client.Room)
				} else {
					h.broadcastSysMsg(client.Room, fmt.Sprintf("%s left the chat", client.Username))
				}
			}

		case message := <-h.Broadcast:
			if message.IsPrivate {
				if target, ok := h.Clients[message.TargetUser]; ok {
					h.sendToClient(target, message)
				}
				if sender, ok := h.Clients[message.Username]; ok && message.Username != message.TargetUser {
					h.sendToClient(sender, message)
				}
				continue
			}

			roomClients := h.Rooms[message.Room]
			for client := range roomClients {
				h.sendToClient(client, message)
			}
		}
	}
}

func (h *Hub) sendToClient(client *Client, message ChatMessage) {
	select {
	case client.Send <- message:
	default:
		close(client.Send)
		delete(h.Rooms[client.Room], client)
		delete(h.Clients, client.Username)
	}
}

func (h *Hub) broadcastSysMsg(room, text string) {
	msg := ChatMessage{
		Username:  "SYSTEM",
		Message:   text,
		Room:      room,
		Timestamp: 0,
	}
	for client := range h.Rooms[room] {
		client.Send <- msg
	}
}
