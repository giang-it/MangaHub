package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"mangahub/pkg/models"
	"os"
)

// SeedData loads manga data from JSON file into the database
func SeedData(db *sql.DB) error {
	// 1. Kiểm tra xem đã có dữ liệu chưa để tránh trùng lặp
	var count int
	db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	if count > 0 {
		return nil // Đã có dữ liệu, không cần seed nữa
	}

	// 2. Đọc file JSON
	content, err := os.ReadFile("data/manga_seed.json")
	if err != nil {
		log.Printf("Warning: Could not read seed file: %v", err)
		return err
	}

	var mangas []models.Manga
	if err := json.Unmarshal(content, &mangas); err != nil {
		return err
	}

	// 3. Insert vào Database using transaction for better performance
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO manga (id, title, author, genres, status, total_chapters, description, cover_url) 
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	inserted := 0
	for _, m := range mangas {
		genresJSON, _ := json.Marshal(m.Genres)
		_, err := stmt.Exec(m.ID, m.Title, m.Author, string(genresJSON), m.Status, m.TotalChapters, m.Description, m.CoverURL)
		if err != nil {
			log.Printf("Lỗi insert %s: %v", m.Title, err)
		} else {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Printf("✓ Đã nạp %d/%d manga vào database!\n", inserted, len(mangas))
	return nil
}
