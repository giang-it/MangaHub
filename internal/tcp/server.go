package tcp

import (
	"bufio"
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
		// Xử lý mỗi kết nối mới bằng một Goroutine riêng
		go s.handleConnection(conn)
	}
}

func (s *ProgressSyncServer) handleConnection(conn net.Conn) {
	addr := conn.RemoteAddr().String()
	s.Mu.Lock()
	s.Clients[addr] = conn
	s.Mu.Unlock()

	fmt.Printf("[TCP] New client connected: %s\n", addr)

	// Giữ kết nối mở để lắng nghe hoặc đợi đóng
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		// Ở đây bạn có thể thêm logic nếu client muốn gửi tin ngược lại
	}

	// Xử lý khi ngắt kết nối
	s.Mu.Lock()
	delete(s.Clients, addr)
	s.Mu.Unlock()
	conn.Close()
	fmt.Printf("[TCP] Client disconnected: %s\n", addr)
}

func (s *ProgressSyncServer) handleBroadcast() {
	for update := range s.Broadcast {
		msg, _ := json.Marshal(update)
		s.Mu.Lock()
		for addr, conn := range s.Clients {
			_, err := fmt.Fprintf(conn, string(msg)+"\n")
			if err != nil {
				conn.Close()
				delete(s.Clients, addr)
			}
		}
		s.Mu.Unlock()
	}
}
