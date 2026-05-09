package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// NotificationServer is the UDP server that receives client registrations
// and broadcasts chapter release notifications.
type NotificationServer struct {
	Port    string
	clients map[string]*net.UDPAddr // key: "host:port"
	mu      sync.Mutex
	conn    *net.UDPConn
}

// Notification is the message broadcasted to registered UDP clients.
type Notification struct {
	Type      string `json:"type"`
	MangaID   string `json:"manga_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// RegistrationMessage is the packet sent by clients to register.
type RegistrationMessage struct {
	Action string `json:"action"` // "register" or "unregister"
	UserID string `json:"user_id"`
}

// RegistrationResponse is the confirmation sent back to the client.
type RegistrationResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// NewNotificationServer creates a new UDP notification server.
func NewNotificationServer(port string) *NotificationServer {
	return &NotificationServer{
		Port:    port,
		clients: make(map[string]*net.UDPAddr),
	}
}

// Start begins listening on the UDP port for client registrations and
// processes incoming packets in a loop.
func (s *NotificationServer) Start() {
	addr, err := net.ResolveUDPAddr("udp", ":"+s.Port)
	if err != nil {
		log.Fatalf("UDP Server resolve error: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("UDP Server listen error: %v", err)
	}
	s.conn = conn

	fmt.Printf("UDP Notification Server listening on udp://localhost:%s\n", s.Port)

	buf := make([]byte, 4096)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("UDP read error: %v", err)
			continue
		}
		go s.handlePacket(buf[:n], clientAddr)
	}
}

// broadcastTrigger extends RegistrationMessage for broadcast-trigger packets.
type broadcastTrigger struct {
	Action  string `json:"action"`
	MangaID string `json:"manga_id"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// handlePacket processes registration, unregistration, and broadcast-trigger packets.
func (s *NotificationServer) handlePacket(data []byte, clientAddr *net.UDPAddr) {
	// First, peek at the action field using a generic map.
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("UDP: invalid packet from %s: %v", clientAddr, err)
		return
	}

	action := raw["action"]
	key := clientAddr.String()

	switch action {
	case "register":
		userID := raw["user_id"]
		s.mu.Lock()
		s.clients[key] = clientAddr
		count := len(s.clients)
		s.mu.Unlock()

		log.Printf("UDP: client registered: %s (user: %s), total clients: %d", key, userID, count)

		resp := RegistrationResponse{
			Status:  "registered",
			Message: fmt.Sprintf("Successfully registered for notifications. Total subscribers: %d", count),
		}
		s.sendTo(resp, clientAddr)

	case "unregister":
		userID := raw["user_id"]
		s.mu.Lock()
		delete(s.clients, key)
		count := len(s.clients)
		s.mu.Unlock()

		log.Printf("UDP: client unregistered: %s (user: %s), total clients: %d", key, userID, count)

		resp := RegistrationResponse{
			Status:  "unregistered",
			Message: "Successfully unsubscribed from notifications.",
		}
		s.sendTo(resp, clientAddr)

	case "broadcast":
		// A trusted caller (CLI notify test) requests a broadcast.
		notif := Notification{
			Type:    raw["type"],
			MangaID: raw["manga_id"],
			Message: raw["message"],
		}
		if notif.Type == "" {
			notif.Type = "chapter_release"
		}
		log.Printf("UDP: broadcast trigger received from %s for manga '%s'", key, notif.MangaID)
		// Run in goroutine so we can send an ack back first.
		go s.Broadcast(notif)

		// Acknowledge the trigger to the sender.
		resp := RegistrationResponse{
			Status:  "broadcast_triggered",
			Message: fmt.Sprintf("Broadcast initiated for manga '%s'", notif.MangaID),
		}
		s.sendTo(resp, clientAddr)

	case "ping":
		// Health-check probe from server status command.
		resp := RegistrationResponse{Status: "pong", Message: "UDP Notification Server is running"}
		s.sendTo(resp, clientAddr)

	default:
		log.Printf("UDP: unknown action '%s' from %s", action, key)
	}
}

// Broadcast sends a notification to all currently registered clients.
func (s *NotificationServer) Broadcast(notif Notification) {
	notif.Timestamp = time.Now().Unix()
	data, err := json.Marshal(notif)
	if err != nil {
		log.Printf("UDP broadcast marshal error: %v", err)
		return
	}

	s.mu.Lock()
	clients := make([]*net.UDPAddr, 0, len(s.clients))
	for _, addr := range s.clients {
		clients = append(clients, addr)
	}
	s.mu.Unlock()

	if len(clients) == 0 {
		log.Println("UDP broadcast: no clients registered, skipping")
		return
	}

	successCount := 0
	for _, addr := range clients {
		if _, err := s.conn.WriteToUDP(data, addr); err != nil {
			log.Printf("UDP broadcast error to %s: %v", addr, err)
			// Remove unreachable client
			s.mu.Lock()
			delete(s.clients, addr.String())
			s.mu.Unlock()
		} else {
			successCount++
		}
	}
	log.Printf("UDP broadcast sent to %d/%d clients: [%s] %s", successCount, len(clients), notif.MangaID, notif.Message)
}

// ClientCount returns the number of currently registered clients.
func (s *NotificationServer) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

// sendTo sends a JSON response back to a specific UDP client.
func (s *NotificationServer) sendTo(v interface{}, addr *net.UDPAddr) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	if _, err := s.conn.WriteToUDP(data, addr); err != nil {
		log.Printf("UDP sendTo error: %v", err)
	}
}
