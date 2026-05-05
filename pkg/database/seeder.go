package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mangahub/pkg/models"
)

func SeedData(db *sql.DB) error {
	// 1. Kiểm tra xem đã có dữ liệu chưa để tránh trùng lặp
	var count int
	db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	if count > 0 {
		return nil // Đã có dữ liệu, không cần seed nữa
	}

	// 2. Đọc file JSON
	content, err := ioutil.ReadFile("data/manga_seed.json")
	if err != nil {
		return err
	}

	var mangas []models.Manga
	if err := json.Unmarshal(content, &mangas); err != nil {
		return err
	}

	// 3. Insert vào Database
	for _, m := range mangas {
		genresJSON, _ := json.Marshal(m.Genres) // Chuyển mảng thành string
		query := `INSERT INTO manga (id, title, author, genres, status, total_chapters, description) 
                  VALUES (?, ?, ?, ?, ?, ?, ?)`
		_, err := db.Exec(query, m.ID, m.Title, m.Author, string(genresJSON), m.Status, m.TotalChapters, m.Description)
		if err != nil {
			log.Printf("Lỗi insert %s: %v", m.Title, err)
		}
	}

	fmt.Println("✓ Đã nạp dữ liệu mẫu thành công!")
	return nil
}
