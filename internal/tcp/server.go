package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"mangahub/pkg/models"
	"net"
	"sync"
)

type ProgressSyncServer struct {
	Port          string
	connections   map[string][]net.Conn
	mu            sync.Mutex
	BroadcastChan chan models.ProgressUpdate
}

func NewProgressSyncServer(port string, broadcastChan chan models.ProgressUpdate) *ProgressSyncServer {
	return &ProgressSyncServer{
		Port:          port,
		connections:   make(map[string][]net.Conn),
		BroadcastChan: broadcastChan,
	}
}

func (s *ProgressSyncServer) Start() {
	listener, err := net.Listen("tcp", ":"+s.Port)
	if err != nil {
		log.Fatalf("TCP Server error: %v", err)
	}
	fmt.Printf("TCP Sync Server listening on tcp://localhost:%s\n", s.Port)

	go s.handleBroadcast()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *ProgressSyncServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Đọc UserID từ client khi mới kết nối
	reader := bufio.NewReader(conn)
	authLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	var authReq struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal([]byte(authLine), &authReq); err != nil {
		return
	}

	s.mu.Lock()
	s.connections[authReq.UserID] = append(s.connections[authReq.UserID], conn)
	s.mu.Unlock()

	// Giữ connection mở
	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			break
		}
	}

	// Xóa connection khi client ngắt kết nối
	s.mu.Lock()
	conns := s.connections[authReq.UserID]
	for i, c := range conns {
		if c == conn {
			s.connections[authReq.UserID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
}

func (s *ProgressSyncServer) handleBroadcast() {
	for update := range s.BroadcastChan {
		s.mu.Lock()
		conns, exists := s.connections[update.UserID]
		if exists {
			data, _ := json.Marshal(update)
			data = append(data, '\n')
			for _, conn := range conns {
				conn.Write(data)
			}
		}
		s.mu.Unlock()
	}
}
