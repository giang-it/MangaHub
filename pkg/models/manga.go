package models

import "time"

// Manga represents a manga series in the database
type Manga struct {
	ID            string   `json:"id" db:"id"`
	Title         string   `json:"title" db:"title"`
	Author        string   `json:"author" db:"author"`
	Genres        []string `json:"genres" db:"genres"`
	Status        string   `json:"status" db:"status"`
	TotalChapters int      `json:"total_chapters" db:"total_chapters"`
	Description   string   `json:"description" db:"description"`
	CoverURL      string   `json:"cover_url,omitempty" db:"cover_url"`
}

// MangaCreateRequest is the request body for creating a new manga
type MangaCreateRequest struct {
	ID            string   `json:"id" binding:"required"`
	Title         string   `json:"title" binding:"required"`
	Author        string   `json:"author" binding:"required"`
	Genres        []string `json:"genres" binding:"required"`
	Status        string   `json:"status" binding:"required"`
	TotalChapters int      `json:"total_chapters"`
	Description   string   `json:"description"`
	CoverURL      string   `json:"cover_url"`
}

// MangaUpdateRequest is the request body for updating an existing manga
type MangaUpdateRequest struct {
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Genres        []string `json:"genres"`
	Status        string   `json:"status"`
	TotalChapters int      `json:"total_chapters"`
	Description   string   `json:"description"`
	CoverURL      string   `json:"cover_url"`
}

// MangaSearchParams holds query parameters for manga search
type MangaSearchParams struct {
	Query  string `form:"q"`
	Genre  string `form:"genre"`
	Status string `form:"status"`
	Author string `form:"author"`
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
}

// UserProgress represents a user's reading progress for a manga
type UserProgress struct {
	UserID         string    `json:"user_id" db:"user_id"`
	MangaID        string    `json:"manga_id" db:"manga_id"`
	CurrentChapter int       `json:"current_chapter" db:"current_chapter"`
	Status         string    `json:"status" db:"status"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// LibraryAddRequest is the request body for adding manga to library
type LibraryAddRequest struct {
	MangaID        string `json:"manga_id" binding:"required"`
	Status         string `json:"status" binding:"required"`
	CurrentChapter int    `json:"current_chapter"`
}

// LibraryUpdateRequest is the request body for updating library entry status
type LibraryUpdateRequest struct {
	MangaID string `json:"manga_id" binding:"required"`
	Status  string `json:"status"`
	Rating  int    `json:"rating"`
}

// ProgressUpdateRequest is the request body for updating reading progress
type ProgressUpdateRequest struct {
	MangaID string `json:"manga_id" binding:"required"`
	Chapter int    `json:"chapter" binding:"required"`
}

// LibraryEntry combines manga info with user progress for library display
type LibraryEntry struct {
	MangaID        string `json:"manga_id"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	CurrentChapter int    `json:"current_chapter"`
	TotalChapters  int    `json:"total_chapters"`
	Status         string `json:"status"`
	UpdatedAt      string `json:"updated_at"`
}

// PaginatedResponse wraps results with pagination metadata
type PaginatedResponse struct {
	Results    interface{} `json:"results"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalCount int         `json:"total_count"`
}
