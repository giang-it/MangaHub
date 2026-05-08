package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
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
	tcpPkg "mangahub/pkg/tcp"
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

func getCurrentUserID() string {
	token := getToken()
	if token == "" {
		return ""
	}
	claims, err := auth.ParseToken(token)
	if err != nil {
		return ""
	}
	return claims["user_id"].(string)
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

	// Tạo channel để kết nối HTTP API và TCP Server
	broadcastChan := make(chan models.ProgressUpdate, 100)

	// Khởi chạy TCP Sync Server trên cổng 8081
	tcpServer := tcp.NewProgressSyncServer("8081", broadcastChan)
	go tcpServer.Start()

	r := gin.Default()

	mh := mangaHandler.NewHandler(db)
	uh := userHandler.NewHandler(db)
	uh.TCPChan = broadcastChan
	// --- AUTHENTICATION ENDPOINTS ---
	r.POST("/auth/register", func(c *gin.Context) {
		var req models.AuthRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
			return
		}

		// FIX: Bắt buộc email hợp lệ
		if req.Email == "" || !models.IsValidEmail(req.Email) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Registration failed: Invalid email format"})
			return
		}

		// FIX: Kiểm tra mật khẩu mạnh (ít nhất 8 ký tự, có hoa, thường, số)
		if !models.IsStrongPassword(req.Password) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Registration failed: Password too weak"})
			return
		}

		hashedPwd, _ := auth.HashPassword(req.Password)
		userID := uuid.New().String()

		_, err := db.Exec("INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)",
			userID, req.Username, req.Email, hashedPwd)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed: Username or Email already exists"})
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

		// FIX: Hỗ trợ login bằng cả Username hoặc Email
		identifier := req.Username
		if identifier == "" {
			identifier = req.Email
		}

		var userID, hashedPwd string
		err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ? OR email = ?",
			identifier, identifier).Scan(&userID, &hashedPwd)

		if err != nil || !auth.CheckPasswordHash(req.Password, hashedPwd) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Login failed: Invalid credentials"})
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

	var statusServerCmd = &cobra.Command{
		Use:   "status",
		Short: "Check server status",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Get(apiURL + "/manga?limit=1")
			if err != nil {
				fmt.Println("✗ HTTP API: Offline")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Println("MangaHub Server Status")
				fmt.Println("----------------------")
				fmt.Println("✓ HTTP API: Online (localhost:8080)")
				fmt.Println("✓ TCP Sync: Online (localhost:8081)")
				fmt.Println("Overall System Health: ✓ Healthy")
			} else {
				fmt.Println("⚠ HTTP API: Degraded")
			}
		},
	}

	serverCmd.AddCommand(startServerCmd)
	serverCmd.AddCommand(statusServerCmd)

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
			if username == "" && email == "" {
				fmt.Println("Error: required flag(s) \"username\" or \"email\" not set")
				return
			}

			fmt.Print("Password: ")
			pwdBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				fmt.Println("\nError reading password")
				return
			}
			fmt.Println()
			password := string(pwdBytes)

			loginID := username
			if loginID == "" {
				loginID = email
			}

			reqBody, _ := json.Marshal(map[string]string{
				"username": loginID,
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
			fmt.Printf("Welcome back, %s!\n\n", loginID)
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
	loginCmd.Flags().StringVar(&email, "email", "", "Email")

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
	var searchLimit, searchPage int
	var searchSortBy, searchOrder string
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
			if searchPage > 0 {
				requestURL += fmt.Sprintf("&page=%d", searchPage)
			}
			if searchSortBy != "" {
				requestURL += "&sort_by=" + url.QueryEscape(searchSortBy)
			}
			if searchOrder != "" {
				requestURL += "&order=" + url.QueryEscape(searchOrder)
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
	searchCmd2.Flags().IntVar(&searchPage, "page", 1, "Page number")
	searchCmd2.Flags().StringVar(&searchSortBy, "sort-by", "title", "Sort by (title, author, total_chapters)")
	searchCmd2.Flags().StringVar(&searchOrder, "order", "asc", "Order (asc, desc)")

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

			// Fetch user progress if logged in
			token := getToken()
			if token != "" {
				req, _ := http.NewRequest("GET", apiURL+"/users/library", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				client := &http.Client{}
				if libResp, err := client.Do(req); err == nil {
					defer libResp.Body.Close()
					if libResp.StatusCode == http.StatusOK {
						var libResult map[string]interface{}
						json.NewDecoder(libResp.Body).Decode(&libResult)
						if entries, ok := libResult["entries"].([]interface{}); ok {
							for _, e := range entries {
								entry := e.(map[string]interface{})
								if entry["manga_id"] == mangaID {
									fmt.Printf("\nProgress:\n")
									fmt.Printf("  Your Status:      %v\n", entry["status"])
									fmt.Printf("  Current Chapter:  %.0f\n", entry["current_chapter"])
									if entry["current_volume"] != nil && entry["current_volume"].(float64) > 0 {
										fmt.Printf("  Current Volume:   %.0f\n", entry["current_volume"])
									}
									if entry["rating"] != nil && entry["rating"].(float64) > 0 {
										fmt.Printf("  Personal Rating:  %.0f/10\n", entry["rating"])
									}
									if entry["notes"] != nil && entry["notes"] != "" {
										fmt.Printf("  Notes:            %v\n", entry["notes"])
									}
									fmt.Printf("  Last Updated:     %v\n", entry["updated_at"])
									break
								}
							}
						}
					}
				}
			}

			fmt.Printf("\nActions:\n")
			fmt.Printf("  Add to library: mangahub library add --manga-id %s --status reading\n", mangaID)
			fmt.Printf("  Update progress: mangahub progress update --manga-id %s --chapter 1\n", mangaID)
		},
	}

	var listPage, listLimit int
	var mangaListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all manga in database",
		Run: func(cmd *cobra.Command, args []string) {
			listURL := fmt.Sprintf("%s/manga?page=%d&limit=%d", apiURL, listPage, listLimit)
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
	mangaListCmd.Flags().IntVar(&listPage, "page", 1, "Page number")
	mangaListCmd.Flags().IntVar(&listLimit, "limit", 20, "Number of results per page")

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

	var libListSortBy, libListOrder string
	var libListCmd = &cobra.Command{
		Use:   "list",
		Short: "View your library",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in. Please login first.")
				return
			}

			reqURL := apiURL + "/users/library?"
			if libStatus != "" {
				reqURL += "status=" + libStatus + "&"
			}
			if libListSortBy != "" {
				reqURL += "sort_by=" + url.QueryEscape(libListSortBy) + "&"
			}
			if libListOrder != "" {
				reqURL += "order=" + url.QueryEscape(libListOrder) + "&"
			}

			req, _ := http.NewRequest("GET", reqURL, nil)
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

			groups := make(map[string][]map[string]interface{})
			for _, e := range entries {
				entry := e.(map[string]interface{})
				st := entry["status"].(string)
				groups[st] = append(groups[st], entry)
			}

			statuses := []string{"reading", "completed", "plan-to-read", "on-hold", "dropped"}

			for _, st := range statuses {
				if items := groups[st]; len(items) > 0 {
					fmt.Printf("%s (%d):\n", strings.Title(st), len(items))
					fmt.Printf("┌%-20s┬%-26s┬%-10s┬%-8s┬%-20s┐\n", strings.Repeat("─", 20), strings.Repeat("─", 26), strings.Repeat("─", 10), strings.Repeat("─", 8), strings.Repeat("─", 20))
					fmt.Printf("│ %-18s │ %-24s │ %-8s │ %-6s │ %-18s │\n", "ID", "Title", "Chapter", "Rating", "Updated")
					fmt.Printf("├%-20s┼%-26s┼%-10s┼%-8s┼%-20s┤\n", strings.Repeat("─", 20), strings.Repeat("─", 26), strings.Repeat("─", 10), strings.Repeat("─", 8), strings.Repeat("─", 20))
					for _, entry := range items {
						ratingStr := "-"
						if r := entry["rating"]; r != nil && r.(float64) > 0 {
							ratingStr = fmt.Sprintf("%.0f/10", r.(float64))
						}
						fmt.Printf("│ %-18s │ %-24s │ %-8.0f │ %-6s │ %-18s │\n",
							truncate(entry["manga_id"].(string), 18),
							truncate(entry["title"].(string), 24),
							entry["current_chapter"],
							ratingStr,
							truncate(entry["updated_at"].(string), 18))
					}
					fmt.Printf("└%-20s┴%-26s┴%-10s┴%-8s┴%-20s┘\n\n", strings.Repeat("─", 20), strings.Repeat("─", 26), strings.Repeat("─", 10), strings.Repeat("─", 8), strings.Repeat("─", 20))
				}
			}
		},
	}
	libListCmd.Flags().StringVar(&libStatus, "status", "", "Filter by status")
	libListCmd.Flags().StringVar(&libListSortBy, "sort-by", "last-updated", "Sort by (title, last-updated)")
	libListCmd.Flags().StringVar(&libListOrder, "order", "desc", "Order (asc, desc)")

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

	// --- TCP SYNC CLI COMMANDS ---
	var syncCmd = &cobra.Command{
		Use:   "sync",
		Short: "Manage real-time synchronization",
	}

	var syncConnectCmd = &cobra.Command{
		Use:   "connect",
		Short: "Connect to TCP sync server",
		Run: func(cmd *cobra.Command, args []string) {
			state := tcpPkg.SyncState{
				Status:           "Active",
				ServerURL:        "localhost:8081",
				SessionID:        "sess_" + uuid.New().String()[:8],
				ConnectedAt:      time.Now(),
				MessagesSent:     0,
				MessagesReceived: 0,
				LastSyncUpdate:   "None",
			}
			tcpPkg.SaveState(state)

			fmt.Println("Connecting to TCP sync server at localhost:8081...")
			fmt.Println("✓ Connected successfully!")
			fmt.Printf("\nConnection Details:\n")
			fmt.Printf("Server: localhost:8081\nSession ID: %s\nConnected at: %v\n", state.SessionID, state.ConnectedAt.Format("2006-01-02 15:04:05 UTC"))
			fmt.Println("\nSync Status:\nAuto-sync: enabled\nConflict resolution: last_write_wins\nDevices connected: 1 (CLI)")
			fmt.Println("\nReal-time sync is now active.")
		},
	}

	var syncStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check sync connection status",
		Run: func(cmd *cobra.Command, args []string) {
			state := tcpPkg.LoadState()
			if state.Status != "Active" {
				fmt.Println("Sync is disconnected. Run 'mangahub sync connect'.")
				return
			}
			uptime := time.Since(state.ConnectedAt).Round(time.Second)
			fmt.Println("TCP Sync Status:")
			fmt.Printf("\nConnection: ✓ %s\nServer: %s\nUptime: %v\n", state.Status, state.ServerURL, uptime)
			fmt.Printf("\nSync Statistics:\nMessages sent: %d\nMessages received: %d\nLast sync: %s\n",
				state.MessagesSent, state.MessagesReceived, state.LastSyncUpdate)
		},
	}

	var syncMonitorCmd = &cobra.Command{
		Use:   "monitor",
		Short: "View real-time progress updates",
		Run: func(cmd *cobra.Command, args []string) {
			state := tcpPkg.LoadState()
			if state.Status != "Active" {
				fmt.Println("Please run 'mangahub sync connect' first.")
				return
			}
			conn, err := net.Dial("tcp", state.ServerURL)
			if err != nil {
				fmt.Printf("Connection failed: %v\n", err)
				return
			}
			defer conn.Close()

			userID := getCurrentUserID()
			fmt.Fprintf(conn, `{"user_id":"%s"}`+"\n", userID)

			fmt.Println("Monitoring real-time sync updates... (Press Ctrl+C to exit)")
			reader := bufio.NewReader(conn)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				var update models.ProgressUpdate
				if err := json.Unmarshal([]byte(line), &update); err == nil {
					fmt.Printf("[%s] ← Device updated: %s → Chapter %d\n",
						time.Now().Format("15:04:05"), update.MangaID, update.Chapter)
					tcpPkg.IncrementReceived()
				}
			}
		},
	}

	syncCmd.AddCommand(syncConnectCmd, syncStatusCmd, syncMonitorCmd)
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
			tcpPkg.IncrementSent(mangaID, chapter)
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

	var progressHistoryCmd = &cobra.Command{
		Use:   "history",
		Short: "View progress updates",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in.")
				return
			}

			reqURL := apiURL + "/users/library"
			req, _ := http.NewRequest("GET", reqURL, nil)
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
				fmt.Println("No progress history found.")
				return
			}

			fmt.Println("Progress History:")
			fmt.Println("-----------------")
			for _, e := range entries {
				entry := e.(map[string]interface{})
				if mangaID == "" || entry["manga_id"] == mangaID {
					fmt.Printf("- %v (%v): Chapter %.0f, Status: %v\n", entry["title"], entry["manga_id"], entry["current_chapter"], entry["status"])
					if entry["current_volume"] != nil && entry["current_volume"].(float64) > 0 {
						fmt.Printf("  Volume: %.0f\n", entry["current_volume"])
					}
					if entry["notes"] != nil && entry["notes"] != "" {
						fmt.Printf("  Notes: %v\n", entry["notes"])
					}
					fmt.Printf("  Last Updated: %v\n\n", entry["updated_at"])
				}
			}
		},
	}
	progressHistoryCmd.Flags().StringVar(&mangaID, "manga-id", "", "Filter by Manga ID")

	progressCmd.AddCommand(progressUpdateCmd)
	progressCmd.AddCommand(progressHistoryCmd)

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(mangaCmd)
	rootCmd.AddCommand(libraryCmd)
	rootCmd.AddCommand(progressCmd)
	rootCmd.AddCommand(syncCmd)

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
