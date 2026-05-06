package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3" // Import driver SQLite
)

// InitDB khởi tạo kết nối và tạo các bảng cần thiết
func InitDB(dbPath string) *sql.DB {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Lỗi mở database:", err)
	}

	// Enable WAL mode for better concurrent read performance
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	// Câu lệnh tạo bảng (Schema)
	schema := `
    CREATE TABLE IF NOT EXISTS users (
        id TEXT PRIMARY KEY,
        username TEXT UNIQUE,
        password_hash TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS manga (
        id TEXT PRIMARY KEY,
        title TEXT NOT NULL,
        author TEXT,
        genres TEXT,
        status TEXT,
        total_chapters INTEGER DEFAULT 0,
        description TEXT,
        cover_url TEXT DEFAULT ''
    );

    CREATE TABLE IF NOT EXISTS user_progress (
        user_id TEXT,
        manga_id TEXT,
        current_chapter INTEGER DEFAULT 0,
        status TEXT,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (user_id, manga_id),
        FOREIGN KEY (user_id) REFERENCES users(id),
        FOREIGN KEY (manga_id) REFERENCES manga(id)
    );`

	_, err = db.Exec(schema)
	if err != nil {
		log.Fatal("Lỗi khởi tạo schema:", err)
	}

	// Add cover_url column if it doesn't exist (migration for existing DBs)
	db.Exec("ALTER TABLE manga ADD COLUMN cover_url TEXT DEFAULT ''")

	log.Println("Kết nối SQLite thành công và Schema đã sẵn sàng.")
	return db
}
