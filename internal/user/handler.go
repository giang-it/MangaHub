package user

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mangahub/pkg/models"

	"github.com/gin-gonic/gin"
)

// Handler holds the database connection for user library operations
type Handler struct {
	DB      *sql.DB
	TCPChan chan<- models.ProgressUpdate
}

// NewHandler creates a new user handler
func NewHandler(db *sql.DB) *Handler {
	return &Handler{DB: db}
}

// validStatuses for library entries
var validStatuses = map[string]bool{
	"reading":      true,
	"completed":    true,
	"plan-to-read": true,
	"on-hold":      true,
	"dropped":      true,
}

// AddToLibrary handles POST /users/library
func (h *Handler) AddToLibrary(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var req models.LibraryAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
		return
	}

	// Validate status
	if !validStatuses[strings.ToLower(req.Status)] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be: reading, completed, plan-to-read, on-hold, or dropped"})
		return
	}

	// Check if manga exists
	var mangaTitle string
	err := h.DB.QueryRow("SELECT title FROM manga WHERE id = ?", req.MangaID).Scan(&mangaTitle)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found: '" + req.MangaID + "'"})
		return
	}

	// Check if already in library
	var exists int
	h.DB.QueryRow("SELECT COUNT(*) FROM user_progress WHERE user_id = ? AND manga_id = ?", userID, req.MangaID).Scan(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Manga already in your library. Use PUT /users/progress to update."})
		return
	}

	// Insert into user_progress
	_, err = h.DB.Exec(
		"INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) VALUES (?, ?, ?, ?, ?)",
		userID, req.MangaID, req.CurrentChapter, req.Status, time.Now().UTC(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add manga to library"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Manga added to library",
		"manga_id": req.MangaID,
		"title":    mangaTitle,
		"status":   req.Status,
	})
}

// GetLibrary handles GET /users/library?status=&sort_by=&page=&limit=
func (h *Handler) GetLibrary(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	statusFilter := c.Query("status")
	page := 1
	limit := 50

	query := `SELECT up.manga_id, m.title, m.author, up.current_chapter, COALESCE(up.current_volume, 0), m.total_chapters, up.status, COALESCE(up.rating, 0), COALESCE(up.notes, ''), CAST(up.updated_at AS TEXT)
			  FROM user_progress up
			  JOIN manga m ON up.manga_id = m.id
			  WHERE up.user_id = ?`
	countQuery := `SELECT COUNT(*) FROM user_progress WHERE user_id = ?`
	args := []interface{}{userID}

	if statusFilter != "" {
		query += " AND up.status = ?"
		countQuery += " AND status = ?"
		args = append(args, statusFilter)
	}

	var totalCount int
	h.DB.QueryRow(countQuery, args...).Scan(&totalCount)

	query += " ORDER BY up.updated_at DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, (page-1)*limit)

	rows, err := h.DB.Query(query, queryArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}
	defer rows.Close()

	var entries []models.LibraryEntry
	for rows.Next() {
		var e models.LibraryEntry
		err := rows.Scan(&e.MangaID, &e.Title, &e.Author, &e.CurrentChapter, &e.CurrentVolume, &e.TotalChapters, &e.Status, &e.Rating, &e.Notes, &e.UpdatedAt)
		if err != nil {
			continue
		}
		entries = append(entries, e)
	}

	if entries == nil {
		entries = []models.LibraryEntry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"entries":     entries,
		"total_count": totalCount,
	})
}

// UpdateProgress handles PUT /users/progress
func (h *Handler) UpdateProgress(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var req models.ProgressUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
		return
	}

	// Check if manga is in user's library
	var currentChapter int
	var currentStatus string
	err := h.DB.QueryRow("SELECT current_chapter, status FROM user_progress WHERE user_id = ? AND manga_id = ?",
		userID, req.MangaID).Scan(&currentChapter, &currentStatus)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga '" + req.MangaID + "' not found in your library. Add it first."})
		return
	}

	// Validate chapter against manga total
	var totalChapters int
	var mangaTitle string
	h.DB.QueryRow("SELECT title, total_chapters FROM manga WHERE id = ?", req.MangaID).Scan(&mangaTitle, &totalChapters)

	if totalChapters > 0 && req.Chapter > totalChapters {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Chapter %d exceeds manga's total chapters (%d)", req.Chapter, totalChapters),
			"max":   totalChapters,
		})
		return
	}

	if req.Chapter < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chapter must be non-negative"})
		return
	}

	// Check for backward progress - require --force flag
	if req.Chapter < currentChapter && !req.Force {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Chapter %d is behind your current progress (Chapter %d). Use --force to set backwards progress.", req.Chapter, currentChapter),
		})
		return
	}

	// Auto-update status to completed if chapter equals total
	newStatus := currentStatus
	if totalChapters > 0 && req.Chapter >= totalChapters {
		newStatus = "completed"
	}

	// Build dynamic update query for progress
	updateQuery := "UPDATE user_progress SET current_chapter = ?, status = ?, updated_at = ?"
	updateArgs := []interface{}{req.Chapter, newStatus, time.Now().UTC()}

	if req.Volume > 0 {
		updateQuery += ", current_volume = ?"
		updateArgs = append(updateArgs, req.Volume)
	}
	if req.Notes != "" {
		updateQuery += ", notes = ?"
		updateArgs = append(updateArgs, req.Notes)
	}

	updateQuery += " WHERE user_id = ? AND manga_id = ?"
	updateArgs = append(updateArgs, userID, req.MangaID)

	_, err = h.DB.Exec(updateQuery, updateArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
		return
	}

	// Send TCP broadcast for real-time sync
	if h.TCPChan != nil {
		h.TCPChan <- models.ProgressUpdate{
			UserID:    userID,
			MangaID:   req.MangaID,
			Chapter:   req.Chapter,
			Timestamp: time.Now().Unix(),
		}
		fmt.Printf("[API-TCP] Dispatched update for User %s\n", userID)
	}

	response := gin.H{
		"message":          "Progress updated successfully",
		"manga":            mangaTitle,
		"previous_chapter": currentChapter,
		"current_chapter":  req.Chapter,
		"status":           newStatus,
	}
	if req.Volume > 0 {
		response["volume"] = req.Volume
	}
	if req.Notes != "" {
		response["notes"] = req.Notes
	}

	c.JSON(http.StatusOK, response)
}

// RemoveFromLibrary handles DELETE /users/library/:manga_id
func (h *Handler) RemoveFromLibrary(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	mangaID := c.Param("manga_id")

	result, err := h.DB.Exec("DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?", userID, mangaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove manga"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found in your library"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Manga removed from library"})
}

// UpdateLibraryEntry handles PUT /users/library - update status and rating of library entry
func (h *Handler) UpdateLibraryEntry(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var req models.LibraryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Check existence
	var exists int
	h.DB.QueryRow("SELECT COUNT(*) FROM user_progress WHERE user_id = ? AND manga_id = ?", userID, req.MangaID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found in your library"})
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}

	if req.Status != "" {
		if !validStatuses[strings.ToLower(req.Status)] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be: reading, completed, plan-to-read, on-hold, or dropped"})
			return
		}
		updates = append(updates, "status = ?")
		args = append(args, req.Status)
	}

	if req.Rating > 0 {
		if req.Rating < 1 || req.Rating > 10 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rating must be between 1 and 10"})
			return
		}
		updates = append(updates, "rating = ?")
		args = append(args, req.Rating)
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update. Provide --status or --rating."})
		return
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, userID, req.MangaID)

	query := "UPDATE user_progress SET " + strings.Join(updates, ", ") + " WHERE user_id = ? AND manga_id = ?"
	_, err := h.DB.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update library entry"})
		return
	}

	response := gin.H{"message": "Library entry updated"}
	if req.Status != "" {
		response["status"] = req.Status
	}
	if req.Rating > 0 {
		response["rating"] = req.Rating
	}

	c.JSON(http.StatusOK, response)
}
