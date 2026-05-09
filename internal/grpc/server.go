package grpc

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	"mangahub/pkg/models"
	pb "mangahub/proto"

	"google.golang.org/grpc"
)

type MangaServer struct {
	pb.UnimplementedMangaServiceServer
	DB      *sql.DB
	TCPChan chan<- models.ProgressUpdate
}

func NewMangaServer(db *sql.DB, tcpChan chan<- models.ProgressUpdate) *MangaServer {
	return &MangaServer{DB: db, TCPChan: tcpChan}
}

func (s *MangaServer) Start(port string) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("gRPC: failed to listen: %v", err)
	}
	gs := grpc.NewServer()
	pb.RegisterMangaServiceServer(gs, s)
	fmt.Printf("gRPC Internal Service listening on grpc://localhost:%s\n", port)
	gs.Serve(lis)
}

func (s *MangaServer) GetManga(ctx context.Context, req *pb.GetMangaRequest) (*pb.MangaResponse, error) {
	var m pb.MangaResponse
	err := s.DB.QueryRow("SELECT id, title, author, status, total_chapters, description FROM manga WHERE id = ?", req.Id).
		Scan(&m.Id, &m.Title, &m.Author, &m.Status, &m.TotalChapters, &m.Description)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *MangaServer) UpdateProgress(ctx context.Context, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	query := `UPDATE user_progress SET current_chapter = ?, current_volume = ?, updated_at = ? WHERE user_id = ? AND manga_id = ?`
	_, err := s.DB.Exec(query, req.Chapter, req.Volume, time.Now().UTC(), req.UserId, req.MangaId)
	if err != nil {
		return &pb.ProgressResponse{Success: false, Message: err.Error()}, nil
	}

	// Đồng bộ sang TCP Sync Server qua channel
	if s.TCPChan != nil {
		s.TCPChan <- models.ProgressUpdate{
			UserID:    req.UserId,
			MangaID:   req.MangaId,
			Chapter:   int(req.Chapter),
			Timestamp: time.Now().Unix(),
		}
	}
	return &pb.ProgressResponse{Success: true, Message: "Updated successfully via gRPC"}, nil
}
