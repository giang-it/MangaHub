package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"mangahub/internal/auth"
	mangaHandler "mangahub/internal/manga"
	"mangahub/internal/tcp"
	userHandler "mangahub/internal/user"
	"mangahub/pkg/database"
	"mangahub/pkg/models"
)

var (
	apiURL    = "http://localhost:8080"
	tokenFile = ".mangahub_token"
)

func getToken() string {
	home, _ := os.UserHomeDir()
	path := home + "/" + tokenFile
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func saveToken(token string) error {
	home, _ := os.UserHomeDir()
	path := home + "/" + tokenFile
	return os.WriteFile(path, []byte(token), 0600)
}

func clearToken() error {
	home, _ := os.UserHomeDir()
	path := home + "/" + tokenFile
	return os.Remove(path)
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("Password must be at least 8 characters with mixed case and numbers")
	}
	hasUpper, hasLower, hasNumber := false, false, false
	for _, c := range password {
		if unicode.IsUpper(c) {
			hasUpper = true
		} else if unicode.IsLower(c) {
			hasLower = true
		} else if unicode.IsDigit(c) {
			hasNumber = true
		}
	}
	if !hasUpper || !hasLower || !hasNumber {
		return fmt.Errorf("Password must be at least 8 characters with mixed case and numbers")
	}
	return nil
}

func runServer() {
	db := database.InitDB("./data/mangahub.db")
	defer db.Close()
	database.SeedData(db)

	tcpServer := tcp.NewProgressSyncServer("8081")
	go tcpServer.Start()

	r := gin.Default()

	// Initialize handlers
	mh := mangaHandler.NewHandler(db)
	uh := userHandler.NewHandler(db)

	uh.TCPChan = tcpServer.Broadcast
	// --- AUTHENTICATION ENDPOINTS ---
	r.POST("/auth/register", func(c *gin.Context) {
		var req models.AuthRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
			return
		}

		hashedPwd, _ := auth.HashPassword(req.Password)
		userID := uuid.New().String()

		_, err := db.Exec("INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)",
			userID, req.Username, hashedPwd)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Username đã tồn tại"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Đăng ký thành công", "user_id": userID})
	})

	r.POST("/auth/login", func(c *gin.Context) {
		var req models.AuthRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu thông tin"})
			return
		}

		var userID, hashedPwd string
		err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?",
			req.Username).Scan(&userID, &hashedPwd)

		if err != nil || !auth.CheckPasswordHash(req.Password, hashedPwd) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sai tài khoản hoặc mật khẩu"})
			return
		}

		token, _ := auth.GenerateToken(userID)
		c.JSON(http.StatusOK, models.AuthResponse{Token: token})
	})

	// JWT Middleware
	authMiddleware := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			tokenString := c.GetHeader("Authorization")
			if tokenString == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
				c.Abort()
				return
			}
			if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
				tokenString = tokenString[7:]
			}

			claims, err := auth.ParseToken(tokenString)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				c.Abort()
				return
			}

			c.Set("user_id", claims["user_id"])
			c.Next()
		}
	}

	// --- PROTECTED AUTH ROUTES ---
	authGroup := r.Group("/auth")
	authGroup.Use(authMiddleware())
	{
		authGroup.GET("/status", func(c *gin.Context) {
			userID := c.MustGet("user_id").(string)

			var username string
			var createdAt string
			err := db.QueryRow("SELECT username, CAST(created_at AS TEXT) FROM users WHERE id = ?", userID).Scan(&username, &createdAt)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"user_id":    userID,
				"username":   username,
				"created_at": createdAt,
			})
		})

		authGroup.POST("/change-password", func(c *gin.Context) {
			userID := c.MustGet("user_id").(string)

			var req struct {
				OldPassword string `json:"old_password"`
				NewPassword string `json:"new_password"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
				return
			}

			var hashedPwd string
			err := db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&hashedPwd)
			if err != nil || !auth.CheckPasswordHash(req.OldPassword, hashedPwd) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Mật khẩu hiện tại không đúng"})
				return
			}

			newHashedPwd, _ := auth.HashPassword(req.NewPassword)
			_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", newHashedPwd, userID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi khi cập nhật mật khẩu"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Đổi mật khẩu thành công"})
		})
	}

	// --- MANGA CRUD ENDPOINTS ---
	r.GET("/manga", mh.SearchManga)
	r.GET("/manga/:id", mh.GetManga)

	mangaAdmin := r.Group("/manga")
	mangaAdmin.Use(authMiddleware())
	{
		mangaAdmin.POST("", mh.CreateManga)
		mangaAdmin.PUT("/:id", mh.UpdateManga)
		mangaAdmin.DELETE("/:id", mh.DeleteManga)
	}

	// --- USER LIBRARY ENDPOINTS ---
	userGroup := r.Group("/users")
	userGroup.Use(authMiddleware())
	{
		userGroup.POST("/library", uh.AddToLibrary)
		userGroup.GET("/library", uh.GetLibrary)
		userGroup.PUT("/library", uh.UpdateLibraryEntry)
		userGroup.DELETE("/library/:manga_id", uh.RemoveFromLibrary)
		userGroup.PUT("/progress", uh.UpdateProgress)
	}

	fmt.Println("Server is running on port 8080...")
	r.Run(":8080")
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mangahub",
		Short: "MangaHub Application (Server & CLI)",
	}

	var serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Server Operations",
	}

	var startServerCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the MangaHub Server",
		Run: func(cmd *cobra.Command, args []string) {
			runServer()
		},
	}
	serverCmd.AddCommand(startServerCmd)

	var authCmd = &cobra.Command{
		Use:   "auth",
		Short: "Authentication Commands",
	}

	var username, email string
	var registerCmd = &cobra.Command{
		Use:   "register",
		Short: "Register New Account",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("Password: ")
			pwdBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				fmt.Println("\nError reading password")
				return
			}
			fmt.Print("\nConfirm password: ")
			pwdConfirmBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				fmt.Println("\nError reading password")
				return
			}
			fmt.Println()

			password := string(pwdBytes)
			if password != string(pwdConfirmBytes) {
				fmt.Println("✗ Registration failed: Passwords do not match")
				return
			}

			if err := validatePassword(password); err != nil {
				fmt.Println("✗ Registration failed: Password too weak")
				fmt.Println(err.Error())
				return
			}

			reqBody, _ := json.Marshal(map[string]string{
				"username": username,
				"email":    email,
				"password": password,
			})

			resp, err := http.Post(apiURL+"/auth/register", "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				fmt.Printf("✗ Registration failed: Server connection error\n")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode != http.StatusCreated {
				errMsg := result["error"]
				fmt.Printf("✗ Registration failed: %v\n", errMsg)
				return
			}

			fmt.Println("\n✓ Account created successfully!")
			fmt.Printf("User ID: %v\n", result["user_id"])
			fmt.Printf("Username: %v\n", username)
			fmt.Printf("Email: %v\n", email)
			fmt.Printf("Created: %v UTC\n\n", time.Now().Format("2006-01-02 15:04:05"))
			fmt.Println("Please login to start using MangaHub:")
			fmt.Printf("mangahub auth login --username %s\n", username)
		},
	}
	registerCmd.Flags().StringVar(&username, "username", "", "Username")
	registerCmd.Flags().StringVar(&email, "email", "", "Email")
	registerCmd.MarkFlagRequired("username")

	var loginCmd = &cobra.Command{
		Use:   "login",
		Short: "Login to your account",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("Password: ")
			pwdBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				fmt.Println("\nError reading password")
				return
			}
			fmt.Println()
			password := string(pwdBytes)

			reqBody, _ := json.Marshal(map[string]string{
				"username": username,
				"password": password,
			})

			resp, err := http.Post(apiURL+"/auth/login", "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				fmt.Printf("✗ Login failed: Server connection error\n")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("✗ Login failed: %v\n", result["error"])
				return
			}

			token := result["token"].(string)
			saveToken(token)

			fmt.Println("\n✓ Login successful!")
			fmt.Printf("Welcome back, %s!\n\n", username)
			fmt.Println("Session Details:")
			fmt.Printf("Token expires: %v UTC (24 hours)\n", time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05"))
			fmt.Println("Permissions: read, write, sync")
			fmt.Println("\nAuto-sync: enabled")
			fmt.Println("Notifications: enabled")
			fmt.Println("\nReady to use MangaHub! Try:")
			fmt.Println("mangahub manga search \"your favorite manga\"")
		},
	}
	loginCmd.Flags().StringVar(&username, "username", "", "Username")
	loginCmd.MarkFlagRequired("username")

	var logoutCmd = &cobra.Command{
		Use:   "logout",
		Short: "Logout of your account",
		Run: func(cmd *cobra.Command, args []string) {
			clearToken()
			fmt.Println("Logged out successfully.")
		},
	}

	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check Authentication Status",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in.")
				return
			}

			req, _ := http.NewRequest("GET", apiURL+"/auth/status", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("Error: Server connection error\n")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)
				if result["error"] != nil {
					fmt.Printf("Session expired or invalid: %v. Please login again.\n", result["error"])
				} else {
					fmt.Printf("Session expired or invalid (%s). Please login again.\n", resp.Status)
				}
				clearToken()
				return
			}

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			fmt.Println("Authentication Status: Active")
			fmt.Printf("User ID: %v\n", result["user_id"])
			fmt.Printf("Username: %v\n", result["username"])
			fmt.Printf("Created At: %v\n", result["created_at"])
		},
	}

	var changePasswordCmd = &cobra.Command{
		Use:   "change-password",
		Short: "Change Password",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in.")
				return
			}

			fmt.Print("Current Password: ")
			oldPwdBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return
			}
			fmt.Print("\nNew Password: ")
			newPwdBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return
			}
			fmt.Print("\nConfirm New Password: ")
			confirmPwdBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return
			}
			fmt.Println()

			if string(newPwdBytes) != string(confirmPwdBytes) {
				fmt.Println("✗ Change password failed: New passwords do not match")
				return
			}

			if err := validatePassword(string(newPwdBytes)); err != nil {
				fmt.Println("✗ Change password failed: Password too weak")
				fmt.Println(err.Error())
				return
			}

			reqBody, _ := json.Marshal(map[string]string{
				"old_password": string(oldPwdBytes),
				"new_password": string(newPwdBytes),
			})

			req, _ := http.NewRequest("POST", apiURL+"/auth/change-password", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("✗ Change password failed: Server connection error\n")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)
				if result["error"] != nil {
					fmt.Printf("✗ Change password failed: %v\n", result["error"])
				} else {
					fmt.Printf("✗ Change password failed: %s\n", resp.Status)
				}
				return
			}

			fmt.Println("✓ Password changed successfully!")
		},
	}

	authCmd.AddCommand(registerCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(statusCmd)
	authCmd.AddCommand(changePasswordCmd)

	// --- MANGA CLI COMMANDS ---
	var mangaCmd = &cobra.Command{
		Use:   "manga",
		Short: "Manga Management Commands",
	}

	var genre, mangaStatus string
	var searchLimit int
	var searchCmd2 = &cobra.Command{
		Use:   "search [query]",
		Short: "Search for manga",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			query := strings.Join(args, " ")
			fmt.Printf("Searching for \"%s\"...\n\n", query)

			requestURL := fmt.Sprintf("%s/manga?q=%s", apiURL, url.QueryEscape(query))
			if genre != "" {
				requestURL += "&genre=" + url.QueryEscape(genre)
			}
			if mangaStatus != "" {
				requestURL += "&status=" + url.QueryEscape(strings.ToLower(mangaStatus))
			}
			if searchLimit > 0 {
				requestURL += fmt.Sprintf("&limit=%d", searchLimit)
			}

			resp, err := http.Get(requestURL)
			if err != nil {
				fmt.Println("✗ Search failed: Server connection error")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			results, ok := result["results"].([]interface{})
			if !ok || len(results) == 0 {
				fmt.Println("No manga found matching your search criteria.")
				fmt.Println("\nSuggestions:")
				fmt.Println("- Check spelling and try again")
				fmt.Println("- Use broader search terms")
				fmt.Println("- Browse by genre: mangahub manga search \"\" --genre action")
				return
			}

			fmt.Printf("Found %d results:\n\n", len(results))
			fmt.Printf("%-22s %-28s %-18s %-12s %s\n", "ID", "Title", "Author", "Status", "Chapters")
			fmt.Println(strings.Repeat("-", 90))
			for _, r := range results {
				m := r.(map[string]interface{})
				chap := int(m["total_chapters"].(float64))
				fmt.Printf("%-22s %-28s %-18s %-12s %d\n",
					m["id"], truncate(m["title"].(string), 26),
					truncate(m["author"].(string), 16), m["status"], chap)
			}
			fmt.Println("\nUse 'mangahub manga info <id>' to view details")
			fmt.Println("Use 'mangahub library add --manga-id <id>' to add to your library")
		},
	}
	searchCmd2.Flags().StringVar(&genre, "genre", "", "Filter by genre")
	searchCmd2.Flags().StringVar(&mangaStatus, "status", "", "Filter by status")
	searchCmd2.Flags().IntVar(&searchLimit, "limit", 20, "Max results")

	var mangaInfoCmd = &cobra.Command{
		Use:   "info [manga-id]",
		Short: "View manga details",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mangaID := args[0]
			resp, err := http.Get(apiURL + "/manga/" + mangaID)
			if err != nil {
				fmt.Println("✗ Error: Server connection error")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("✗ Manga not found: '%s'\n\nTry searching instead:\nmangahub manga search \"manga title\"\n", mangaID)
				return
			}

			var m map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&m)

			fmt.Println(strings.Repeat("─", 60))
			fmt.Printf("  %s\n", strings.ToUpper(m["title"].(string)))
			fmt.Println(strings.Repeat("─", 60))
			fmt.Printf("\nBasic Information:\n")
			fmt.Printf("  ID:       %s\n", m["id"])
			fmt.Printf("  Title:    %s\n", m["title"])
			fmt.Printf("  Author:   %s\n", m["author"])
			if genres, ok := m["genres"].([]interface{}); ok {
				gs := make([]string, len(genres))
				for i, g := range genres {
					gs[i] = g.(string)
				}
				fmt.Printf("  Genres:   %s\n", strings.Join(gs, ", "))
			}
			fmt.Printf("  Status:   %s\n", m["status"])
			fmt.Printf("  Chapters: %.0f\n", m["total_chapters"])
			fmt.Printf("\nDescription:\n  %s\n", m["description"])
			fmt.Printf("\nActions:\n")
			fmt.Printf("  Add to library: mangahub library add --manga-id %s --status reading\n", mangaID)
			fmt.Printf("  Update progress: mangahub progress update --manga-id %s --chapter 1\n", mangaID)
		},
	}

	var mangaListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all manga in database",
		Run: func(cmd *cobra.Command, args []string) {
			listURL := apiURL + "/manga?limit=50"
			if genre != "" {
				listURL += "&genre=" + url.QueryEscape(genre)
			}
			resp, err := http.Get(listURL)
			if err != nil {
				fmt.Println("✗ Error: Server connection error")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			results, ok := result["results"].([]interface{})
			if !ok || len(results) == 0 {
				fmt.Println("No manga found.")
				return
			}

			fmt.Printf("Manga Database (%d entries)\n\n", int(result["total_count"].(float64)))
			fmt.Printf("%-22s %-28s %-18s %-12s %s\n", "ID", "Title", "Author", "Status", "Chapters")
			fmt.Println(strings.Repeat("-", 90))
			for _, r := range results {
				m := r.(map[string]interface{})
				fmt.Printf("%-22s %-28s %-18s %-12s %.0f\n",
					m["id"], truncate(m["title"].(string), 26),
					truncate(m["author"].(string), 16), m["status"], m["total_chapters"])
			}
		},
	}
	mangaListCmd.Flags().StringVar(&genre, "genre", "", "Filter by genre")

	mangaCmd.AddCommand(searchCmd2)
	mangaCmd.AddCommand(mangaInfoCmd)
	mangaCmd.AddCommand(mangaListCmd)

	// --- LIBRARY CLI COMMANDS ---
	var libraryCmd = &cobra.Command{
		Use:   "library",
		Short: "Library Operations",
	}

	var mangaID, libStatus string
	var libRating int
	var libAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add manga to library",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in. Please login first.")
				return
			}

			reqBody, _ := json.Marshal(map[string]interface{}{
				"manga_id": mangaID,
				"status":   libStatus,
			})

			req, _ := http.NewRequest("POST", apiURL+"/users/library", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("✗ Error: Server connection error")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode != http.StatusCreated {
				fmt.Printf("✗ Failed to add: %v\n", result["error"])
				return
			}

			fmt.Println("✓ Manga added to library!")
			fmt.Printf("  Manga: %v\n", result["title"])
			fmt.Printf("  Status: %v\n", result["status"])
		},
	}
	libAddCmd.Flags().StringVar(&mangaID, "manga-id", "", "Manga ID")
	libAddCmd.Flags().StringVar(&libStatus, "status", "reading", "Status (reading, completed, plan-to-read, on-hold, dropped)")
	libAddCmd.Flags().IntVar(&libRating, "rating", 0, "Rating (1-10)")
	libAddCmd.MarkFlagRequired("manga-id")

	var libListCmd = &cobra.Command{
		Use:   "list",
		Short: "View your library",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in. Please login first.")
				return
			}

			url := apiURL + "/users/library"
			if libStatus != "" {
				url += "?status=" + libStatus
			}

			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("✗ Error: Server connection error")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			entries, ok := result["entries"].([]interface{})
			if !ok || len(entries) == 0 {
				fmt.Println("Your library is empty.\n\nGet started:")
				fmt.Println("  mangahub manga search \"your favorite series\"")
				fmt.Println("  mangahub library add --manga-id <id> --status reading")
				return
			}

			fmt.Printf("Your Manga Library (%d entries)\n\n", int(result["total_count"].(float64)))
			fmt.Printf("%-20s %-26s %-12s %-14s %s\n", "ID", "Title", "Chapter", "Status", "Updated")
			fmt.Println(strings.Repeat("-", 85))
			for _, e := range entries {
				entry := e.(map[string]interface{})
				cur := int(entry["current_chapter"].(float64))
				total := int(entry["total_chapters"].(float64))
				fmt.Printf("%-20s %-26s %4d/%-6d %-14s %s\n",
					entry["manga_id"], truncate(entry["title"].(string), 24),
					cur, total, entry["status"], entry["updated_at"])
			}
		},
	}
	libListCmd.Flags().StringVar(&libStatus, "status", "", "Filter by status")

	var libRemoveCmd = &cobra.Command{
		Use:   "remove",
		Short: "Remove manga from library",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in.")
				return
			}

			req, _ := http.NewRequest("DELETE", apiURL+"/users/library/"+mangaID, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("✗ Error: Server connection error")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("✗ Remove failed: %v\n", result["error"])
				return
			}
			fmt.Println("✓ Manga removed from library")
		},
	}
	libRemoveCmd.Flags().StringVar(&mangaID, "manga-id", "", "Manga ID")
	libRemoveCmd.MarkFlagRequired("manga-id")

	// --- LIBRARY UPDATE CLI COMMAND ---
	var libUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update library entry (status and/or rating)",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in. Please login first.")
				return
			}

			reqData := map[string]interface{}{
				"manga_id": mangaID,
			}
			if libStatus != "" {
				reqData["status"] = libStatus
			}
			if libRating > 0 {
				reqData["rating"] = libRating
			}

			reqBody, _ := json.Marshal(reqData)

			req, _ := http.NewRequest("PUT", apiURL+"/users/library", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("✗ Error: Server connection error")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("✗ Update failed: %v\n", result["error"])
				return
			}

			fmt.Println("✓ Library entry updated!")
			if result["status"] != nil {
				fmt.Printf("  Status: %v\n", result["status"])
			}
			if result["rating"] != nil {
				fmt.Printf("  Rating: %v/10\n", result["rating"])
			}
		},
	}
	libUpdateCmd.Flags().StringVar(&mangaID, "manga-id", "", "Manga ID")
	libUpdateCmd.Flags().StringVar(&libStatus, "status", "", "New status (reading, completed, plan-to-read, on-hold, dropped)")
	libUpdateCmd.Flags().IntVar(&libRating, "rating", 0, "Rating (1-10)")
	libUpdateCmd.MarkFlagRequired("manga-id")

	libraryCmd.AddCommand(libAddCmd)
	libraryCmd.AddCommand(libListCmd)
	libraryCmd.AddCommand(libRemoveCmd)
	libraryCmd.AddCommand(libUpdateCmd)

	// --- PROGRESS CLI COMMANDS ---
	var progressCmd = &cobra.Command{
		Use:   "progress",
		Short: "Progress Tracking",
	}

	var chapter, volume int
	var notes string
	var forceProgress bool
	var progressUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update reading progress",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in.")
				return
			}

			reqData := map[string]interface{}{
				"manga_id": mangaID,
				"chapter":  chapter,
			}
			if volume > 0 {
				reqData["volume"] = volume
			}
			if notes != "" {
				reqData["notes"] = notes
			}
			if forceProgress {
				reqData["force"] = true
			}

			reqBody, _ := json.Marshal(reqData)

			req, _ := http.NewRequest("PUT", apiURL+"/users/progress", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("✗ Error: Server connection error")
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("✗ Progress update failed: %v\n", result["error"])
				return
			}

			fmt.Println("✓ Progress updated successfully!")
			fmt.Printf("  Manga: %v\n", result["manga"])
			fmt.Printf("  Previous: Chapter %v\n", result["previous_chapter"])
			fmt.Printf("  Current:  Chapter %v\n", result["current_chapter"])
			if result["volume"] != nil {
				fmt.Printf("  Volume:   %v\n", result["volume"])
			}
			fmt.Printf("  Status:   %v\n", result["status"])
			if result["notes"] != nil {
				fmt.Printf("  Notes:    %v\n", result["notes"])
			}
		},
	}
	progressUpdateCmd.Flags().StringVar(&mangaID, "manga-id", "", "Manga ID")
	progressUpdateCmd.Flags().IntVar(&chapter, "chapter", 0, "Chapter number")
	progressUpdateCmd.Flags().IntVar(&volume, "volume", 0, "Volume number")
	progressUpdateCmd.Flags().StringVar(&notes, "notes", "", "Reading notes")
	progressUpdateCmd.Flags().BoolVar(&forceProgress, "force", false, "Force backward progress")
	progressUpdateCmd.MarkFlagRequired("manga-id")
	progressUpdateCmd.MarkFlagRequired("chapter")

	progressCmd.AddCommand(progressUpdateCmd)

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(mangaCmd)
	rootCmd.AddCommand(libraryCmd)
	rootCmd.AddCommand(progressCmd)

	if len(os.Args) == 1 {
		// Run server by default if no arguments are provided
		runServer()
		return
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// truncate shortens a string to maxLen with "..." if needed
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
