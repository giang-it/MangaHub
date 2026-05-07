package tcp

import (
	"encoding/json"
	"fmt"
	"mangahub/pkg/models"
	"net"
	"sync"
)

type ProgressSyncServer struct {
	Port      string
	Clients   map[string]net.Conn
	Mu        sync.Mutex
	Broadcast chan models.ProgressUpdate
}

func NewProgressSyncServer(port string) *ProgressSyncServer {
	return &ProgressSyncServer{
		Port:      port,
		Clients:   make(map[string]net.Conn),
		Broadcast: make(chan models.ProgressUpdate),
	}
}

func (s *ProgressSyncServer) Start() {
	listener, err := net.Listen("tcp", ":"+s.Port)
	if err != nil {
		fmt.Printf("TCP Error: %v\n", err)
		return
	}

	fmt.Printf("TCP Sync Server is running on port %s...\n", s.Port)

	// Chạy Goroutine để xử lý việc gửi tin nhắn tới mọi người
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
	addr := conn.RemoteAddr().String()
	defer func() {
		s.Mu.Lock()
		delete(s.Clients, addr)
		s.Mu.Unlock()
		conn.Close()
		fmt.Printf("[TCP] Cleaned up connection for: %s\n", addr)
	}()

	s.Mu.Lock()
	s.Clients[addr] = conn
	s.Mu.Unlock()

	fmt.Printf("[TCP] New client connected: %s\n", addr)

	buffer := make([]byte, 1024)
	for {
		_, err := conn.Read(buffer)
		if err != nil {

			return
		}
	}
}

func (s *ProgressSyncServer) handleBroadcast() {
	for update := range s.Broadcast {
		msg, _ := json.Marshal(update)
		payload := append(msg, '\n')

		s.Mu.Lock()
		for addr, conn := range s.Clients {
			_, err := conn.Write(payload)
			if err != nil {
				fmt.Printf("[TCP] Failed to send to %s: %v\n", addr, err)
				conn.Close()
				delete(s.Clients, addr)
			}
		}
		s.Mu.Unlock()
	}
}
