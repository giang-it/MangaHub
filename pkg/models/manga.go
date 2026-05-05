package models

type Manga struct {
	ID            string   `json:"id" db:"id"`
	Title         string   `json:"title" db:"title"`
	Author        string   `json:"author" db:"author"`
	Genres        []string `json:"genres" db:"genres"`
	Status        string   `json:"status" db:"status"`
	TotalChapters int      `json:"total_chapters" db:"total_chapters"`
	Description   string   `json:"description" db:"description"`
}
