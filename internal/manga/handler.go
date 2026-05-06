package manga

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"mangahub/pkg/models"
)

// Handler holds the database connection for manga operations
type Handler struct {
	DB *sql.DB
}

// NewHandler creates a new manga handler
func NewHandler(db *sql.DB) *Handler {
	return &Handler{DB: db}
}

// SearchManga handles GET /manga?q=&genre=&status=&page=&limit=
func (h *Handler) SearchManga(c *gin.Context) {
	var params models.MangaSearchParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 20
	}
	offset := (params.Page - 1) * params.Limit

	query := "SELECT id, title, author, genres, status, total_chapters, description, COALESCE(cover_url,'') FROM manga WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM manga WHERE 1=1"
	var args []interface{}

	if params.Query != "" {
		query += " AND (title LIKE ? OR author LIKE ?)"
		countQuery += " AND (title LIKE ? OR author LIKE ?)"
		like := "%" + params.Query + "%"
		args = append(args, like, like)
	}
	if params.Genre != "" {
		query += " AND genres LIKE ?"
		countQuery += " AND genres LIKE ?"
		args = append(args, "%"+params.Genre+"%")
	}
	if params.Status != "" {
		query += " AND status = ?"
		countQuery += " AND status = ?"
		args = append(args, params.Status)
	}
	if params.Author != "" {
		query += " AND author LIKE ?"
		countQuery += " AND author LIKE ?"
		args = append(args, "%"+params.Author+"%")
	}

	var totalCount int
	h.DB.QueryRow(countQuery, args...).Scan(&totalCount)

	query += " ORDER BY title ASC LIMIT ? OFFSET ?"
	queryArgs := append(args, params.Limit, offset)

	rows, err := h.DB.Query(query, queryArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}
	defer rows.Close()

	var mangas []models.Manga
	for rows.Next() {
		var m models.Manga
		var genresStr string
		err := rows.Scan(&m.ID, &m.Title, &m.Author, &genresStr, &m.Status, &m.TotalChapters, &m.Description, &m.CoverURL)
		if err != nil {
			continue
		}
		json.Unmarshal([]byte(genresStr), &m.Genres)
		if m.Genres == nil {
			m.Genres = []string{}
		}
		mangas = append(mangas, m)
	}

	if mangas == nil {
		mangas = []models.Manga{}
	}

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Results:    mangas,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalCount: totalCount,
	})
}

// GetManga handles GET /manga/:id
func (h *Handler) GetManga(c *gin.Context) {
	mangaID := c.Param("id")

	var m models.Manga
	var genresStr string
	err := h.DB.QueryRow(
		"SELECT id, title, author, genres, status, total_chapters, description, COALESCE(cover_url,'') FROM manga WHERE id = ?",
		mangaID,
	).Scan(&m.ID, &m.Title, &m.Author, &genresStr, &m.Status, &m.TotalChapters, &m.Description, &m.CoverURL)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found: '" + mangaID + "'"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	json.Unmarshal([]byte(genresStr), &m.Genres)
	if m.Genres == nil {
		m.Genres = []string{}
	}

	c.JSON(http.StatusOK, m)
}

// CreateManga handles POST /manga (admin)
func (h *Handler) CreateManga(c *gin.Context) {
	var req models.MangaCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid manga data: " + err.Error()})
		return
	}

	// Validate status
	validStatuses := map[string]bool{"ongoing": true, "completed": true, "hiatus": true}
	if !validStatuses[strings.ToLower(req.Status)] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be: ongoing, completed, or hiatus"})
		return
	}

	genresJSON, _ := json.Marshal(req.Genres)
	_, err := h.DB.Exec(
		"INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		req.ID, req.Title, req.Author, string(genresJSON), req.Status, req.TotalChapters, req.Description, req.CoverURL,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY") {
			c.JSON(http.StatusConflict, gin.H{"error": "Manga with this ID already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create manga"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Manga created successfully", "id": req.ID})
}

// UpdateManga handles PUT /manga/:id
func (h *Handler) UpdateManga(c *gin.Context) {
	mangaID := c.Param("id")

	// Check if manga exists
	var exists int
	h.DB.QueryRow("SELECT COUNT(*) FROM manga WHERE id = ?", mangaID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found"})
		return
	}

	var req models.MangaUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}

	if req.Title != "" {
		updates = append(updates, "title = ?")
		args = append(args, req.Title)
	}
	if req.Author != "" {
		updates = append(updates, "author = ?")
		args = append(args, req.Author)
	}
	if req.Genres != nil {
		genresJSON, _ := json.Marshal(req.Genres)
		updates = append(updates, "genres = ?")
		args = append(args, string(genresJSON))
	}
	if req.Status != "" {
		updates = append(updates, "status = ?")
		args = append(args, req.Status)
	}
	if req.TotalChapters > 0 {
		updates = append(updates, "total_chapters = ?")
		args = append(args, req.TotalChapters)
	}
	if req.Description != "" {
		updates = append(updates, "description = ?")
		args = append(args, req.Description)
	}
	if req.CoverURL != "" {
		updates = append(updates, "cover_url = ?")
		args = append(args, req.CoverURL)
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	args = append(args, mangaID)
	query := "UPDATE manga SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	_, err := h.DB.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update manga"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Manga updated successfully"})
}

// DeleteManga handles DELETE /manga/:id
func (h *Handler) DeleteManga(c *gin.Context) {
	mangaID := c.Param("id")

	result, err := h.DB.Exec("DELETE FROM manga WHERE id = ?", mangaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete manga"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found"})
		return
	}

	// Also clean up user_progress for this manga
	h.DB.Exec("DELETE FROM user_progress WHERE manga_id = ?", mangaID)

	c.JSON(http.StatusOK, gin.H{"message": "Manga deleted successfully"})
}
