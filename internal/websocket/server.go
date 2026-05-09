package websocket

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatServer struct {
	Port string
	Hub  *Hub
}

func NewChatServer(port string) *ChatServer {
	return &ChatServer{
		Port: port,
		Hub:  NewHub(),
	}
}

func (s *ChatServer) Start() {
	go s.Hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.serveWs)

	fmt.Printf("WebSocket Chat Server listening on ws://localhost:%s\n", s.Port)
	if err := http.ListenAndServe(":"+s.Port, mux); err != nil {
		log.Fatalf("WebSocket server error: %v", err)
	}
}

func (s *ChatServer) serveWs(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")
	room := r.URL.Query().Get("room")

	if userID == "" || username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if room == "" {
		room = "#general"
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		Hub:      s.Hub,
		Conn:     conn,
		Send:     make(chan ChatMessage, 256),
		UserID:   userID,
		Username: username,
		Room:     room,
	}

	client.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}
