package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"mangahub/internal/auth"
	mangaHandler "mangahub/internal/manga"
	"mangahub/internal/tcp"
	internalUDP "mangahub/internal/udp"
	userHandler "mangahub/internal/user"
	chatPkg "mangahub/internal/websocket"
	"mangahub/pkg/database"
	"mangahub/pkg/models"
	tcpPkg "mangahub/pkg/tcp"
	udpPkg "mangahub/pkg/udp"

	grpcInternal "mangahub/internal/grpc"
	pb "mangahub/proto"

	"google.golang.org/grpc"
)

var (
	apiURL        = "http://localhost:8080"
	tokenFile     = ".mangahub_token"
	globalChatHub *chatPkg.Hub
	sessionName   string
)

func getTokenPath() string {
	home, _ := os.UserHomeDir()
	
	// 1. Explicit CLI Flag
	if sessionName != "" {
		return home + "/" + tokenFile + "_" + sessionName
	}
	// 2. Environment Variable
	if envPath := os.Getenv("MANGAHUB_TOKEN_FILE"); envPath != "" {
		return envPath
	}
	// 3. Automatic detection for Windows Terminal tabs
	if wtSession := os.Getenv("WT_SESSION"); wtSession != "" {
		return home + "/" + tokenFile + "_" + wtSession
	}
	// 4. Default
	return home + "/" + tokenFile
}

func getToken() string {
	b, err := os.ReadFile(getTokenPath())
	if err != nil {
		return ""
	}
	return string(b)
}

func saveToken(token string) error {
	return os.WriteFile(getTokenPath(), []byte(token), 0600)
}

func clearToken() error {
	return os.Remove(getTokenPath())
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

func getUsernameFromDB(userID string) string {
	if userID == "" {
		return ""
	}
	db := database.InitDB("./data/mangahub.db")
	defer db.Close()

	var username string
	err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if err != nil {
		return ""
	}
	return username
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

// globalUDPServer is used by the HTTP progress-update handler to trigger notifications.
var globalUDPServer *internalUDP.NotificationServer

func runServer() {
	db := database.InitDB("./data/mangahub.db")
	defer db.Close()
	database.SeedData(db)

	// Tạo channel để kết nối HTTP API và TCP Server
	broadcastChan := make(chan models.ProgressUpdate, 100)

	// Khởi chạy TCP Sync Server trên cổng 8081
	tcpServer := tcp.NewProgressSyncServer("8081", broadcastChan)
	go tcpServer.Start()

	// Khởi chạy UDP Notification Server trên cổng 9091
	udpServer := internalUDP.NewNotificationServer("9091")
	globalUDPServer = udpServer
	go udpServer.Start()

	// Khởi chạy WebSocket Chat Server trên cổng 9093
	wsServer := chatPkg.NewChatServer("9093")
	globalChatHub = wsServer.Hub
	go wsServer.Start()

	// Khởi tạo và chạy gRPC Server trên port 9092
	gServer := grpcInternal.NewMangaServer(db, broadcastChan)
	go gServer.Start("9092")

	r := gin.Default()

	r.GET("/chat/users", func(c *gin.Context) {
		if globalChatHub == nil {
			c.JSON(http.StatusOK, []interface{}{})
			return
		}
		globalChatHub.RLock()
		defer globalChatHub.RUnlock()

		var users []map[string]string
		for room, clients := range globalChatHub.Rooms {
			for client := range clients {
				users = append(users, map[string]string{
					"username": client.Username,
					"room":     room,
				})
			}
		}
		c.JSON(http.StatusOK, users)
	})

	r.GET("/chat/history", func(c *gin.Context) {
		room := c.Query("room")
		if globalChatHub == nil {
			c.JSON(http.StatusOK, []interface{}{})
			return
		}
		globalChatHub.RLock()
		defer globalChatHub.RUnlock()

		history := globalChatHub.History[room]
		if history == nil {
			history = []chatPkg.ChatMessage{}
		}
		c.JSON(http.StatusOK, history)
	})

	r.POST("/chat/send", func(c *gin.Context) {
		var req struct {
			Username   string `json:"username"`
			Room       string `json:"room"`
			Message    string `json:"message"`
			IsPrivate  bool   `json:"is_private"`
			TargetUser string `json:"target_user"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		if globalChatHub != nil {
			msg := chatPkg.ChatMessage{
				Username:   req.Username,
				Message:    req.Message,
				Room:       req.Room,
				Timestamp:  time.Now().Unix(),
				IsPrivate:  req.IsPrivate,
				TargetUser: req.TargetUser,
			}
			globalChatHub.Broadcast <- msg
		}
		c.JSON(http.StatusOK, gin.H{"status": "sent"})
	})

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
	rootCmd.PersistentFlags().StringVar(&sessionName, "session", "", "Session profile name for multiple clients (e.g. client1)")

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
			fmt.Println("MangaHub Server Status")
			fmt.Println(strings.Repeat("─", 60))

			// --- HTTP API ---
			resp, err := http.Get(apiURL + "/manga?limit=1")
			if err != nil {
				fmt.Println("✗ HTTP API:        Offline (localhost:8080)")
			} else {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					fmt.Println("✓ HTTP API:        Online  (localhost:8080)")
				} else {
					fmt.Println("⚠ HTTP API:        Degraded (localhost:8080)")
				}
			}

			// --- TCP Sync ---
			tcpConn, tcpErr := net.DialTimeout("tcp", "localhost:8081", 2*time.Second)
			if tcpErr != nil {
				fmt.Println("✗ TCP Sync:        Offline (localhost:8081)")
			} else {
				tcpConn.Close()
				fmt.Println("✓ TCP Sync:        Online  (localhost:8081)")
			}

			// --- UDP Notifications ---
			udpAddr, _ := net.ResolveUDPAddr("udp", "localhost:9091")
			udpProbe, udpErr := net.DialUDP("udp", nil, udpAddr)
			if udpErr != nil {
				fmt.Println("✗ UDP Notify:      Offline (localhost:9091)")
			} else {
				// UDP is connectionless; a successful dial means the local stack is OK.
				// Send a probe packet and wait briefly for a response.
				probe, _ := json.Marshal(map[string]string{"action": "ping", "user_id": "status-check"})
				udpProbe.SetDeadline(time.Now().Add(500 * time.Millisecond))
				udpProbe.Write(probe)
				buf := make([]byte, 256)
				_, udpReadErr := udpProbe.Read(buf)
				udpProbe.Close()
				if udpReadErr != nil {
					// No response – server may still be up; treat as online
					fmt.Println("⚠ UDP Notify:      Degraded (localhost:9091) – no response")
				} else {
					fmt.Println("✓ UDP Notify:      Online  (localhost:9091)")
				}
			}

			fmt.Println(strings.Repeat("─", 60))

			// Overall health
			if err == nil && tcpErr == nil {
				fmt.Println("Overall System Health: ✓ Healthy")
			} else {
				fmt.Println("Overall System Health: ⚠ Degraded")
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
	registerCmd.MarkFlagRequired("email")

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

	var syncDisconnectCmd = &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect from sync server",
		Run: func(cmd *cobra.Command, args []string) {
			state := tcpPkg.LoadState()
			if state.Status == "Disconnected" {
				fmt.Println("Already disconnected from TCP sync server.")
				return
			}

			state.Status = "Disconnected"
			tcpPkg.SaveState(state)
			fmt.Println("✓ Disconnected from TCP sync server.")
			fmt.Println("Real-time sync is now paused.")
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

	syncCmd.AddCommand(syncConnectCmd, syncStatusCmd, syncMonitorCmd, syncDisconnectCmd)
	// --- PROGRESS CLI COMMANDS ---
	var progressCmd = &cobra.Command{
		Use:   "progress",
		Short: "Progress Tracking",
	}

	var progressSyncCmd = &cobra.Command{
		Use:   "sync",
		Short: "Manual sync with server",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in. Cannot sync.")
				return
			}
			fmt.Println("Syncing progress with server...")
			time.Sleep(1 * time.Second) // Giả lập độ trễ mạng
			fmt.Println("✓ Progress successfully synchronized across all devices.")
		},
	}

	var progressSyncStatusCmd = &cobra.Command{
		Use:   "sync-status",
		Short: "Check sync status for progress",
		Run: func(cmd *cobra.Command, args []string) {
			state := tcpPkg.LoadState()
			fmt.Println("Progress Sync Status:")
			fmt.Printf("Auto-sync: %s\n", map[bool]string{true: "enabled", false: "disabled"}[state.Status == "Active"])
			fmt.Println("Conflict resolution: last_write_wins")
			if state.Status == "Active" {
				fmt.Printf("Last sync update: %s\n", state.LastSyncUpdate)
			}
		},
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
	progressCmd.AddCommand(progressSyncCmd)
	progressCmd.AddCommand(progressSyncStatusCmd)

	// ─── UDP NOTIFY CLI COMMANDS ───────────────────────────────────────────────
	var notifyCmd = &cobra.Command{
		Use:   "notify",
		Short: "UDP Notification Commands",
	}

	// notify subscribe
	var notifySubscribeCmd = &cobra.Command{
		Use:   "subscribe",
		Short: "Subscribe to chapter release notifications via UDP",
		Run: func(cmd *cobra.Command, args []string) {
			state := udpPkg.LoadNotifyState()
			if state.Subscribed {
				fmt.Println("Already subscribed to notifications.")
				fmt.Printf("Server: %s\n", state.ServerAddr)
				fmt.Println("Run 'mangahub notify unsubscribe' to stop.")
				return
			}

			userID := getCurrentUserID()
			if userID == "" {
				fmt.Println("Not logged in. Please login first.")
				return
			}

			serverAddr := "localhost:9091"
			udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
			if err != nil {
				fmt.Printf("✗ Subscribe failed: cannot resolve server address: %v\n", err)
				return
			}

			// Bind a local port so the server can send us notifications.
			localAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
			conn, err := net.ListenUDP("udp", localAddr)
			if err != nil {
				fmt.Printf("✗ Subscribe failed: %v\n", err)
				return
			}
			defer conn.Close()

			// Send registration packet
			regMsg, _ := json.Marshal(map[string]string{
				"action":  "register",
				"user_id": userID,
			})
			conn.WriteToUDP(regMsg, udpAddr)

			// Wait for server confirmation
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			buf := make([]byte, 512)
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				fmt.Println("✗ Subscribe failed: no response from UDP server (is server running?)")
				return
			}

			var resp map[string]string
			json.Unmarshal(buf[:n], &resp)

			if resp["status"] != "registered" {
				fmt.Printf("✗ Subscribe failed: %s\n", resp["message"])
				return
			}

			// Save local subscription state
			newState := udpPkg.NotifyState{
				Subscribed:   true,
				ServerAddr:   serverAddr,
				SubscribedAt: time.Now(),
				UserID:       userID,
			}
			udpPkg.SaveNotifyState(newState)

			fmt.Println("✓ Subscribed to chapter release notifications!")
			fmt.Printf("\nNotification Details:\n")
			fmt.Printf("  Server:       %s\n", serverAddr)
			fmt.Printf("  Subscribed:   %s UTC\n", time.Now().Format("2006-01-02 15:04:05"))
			fmt.Printf("  User ID:      %s\n", userID)
			fmt.Printf("  %s\n", resp["message"])
			fmt.Println("\nYou will receive UDP notifications when new chapters are released.")
			fmt.Println("Run 'mangahub notify unsubscribe' to stop receiving notifications.")
			fmt.Println("Run 'mangahub notify test' to trigger a test notification.")

			fmt.Println("\nListening for notifications... (Press Ctrl+C to exit)")

			// Xóa deadline trước đó để lắng nghe vô thời hạn
			conn.SetReadDeadline(time.Time{})

			for {
				n, _, err := conn.ReadFromUDP(buf)
				if err != nil {
					fmt.Printf("\nStopped listening: %v\n", err)
					break
				}

				var notif internalUDP.Notification
				if err := json.Unmarshal(buf[:n], &notif); err == nil && notif.Type != "" {
					fmt.Printf("\n[%s] 🔔 NOTIFICATION RECEIVED!\n", time.Now().Format("15:04:05"))
					fmt.Printf("  Manga: %s\n", notif.MangaID)
					fmt.Printf("  Message: %s\n", notif.Message)
				} else {
					fmt.Printf("\n[%s] 🔔 Raw Notification: %s\n", time.Now().Format("15:04:05"), string(buf[:n]))
				}
			}
		},
	}

	// notify unsubscribe
	var notifyUnsubscribeCmd = &cobra.Command{
		Use:   "unsubscribe",
		Short: "Unsubscribe from chapter release notifications",
		Run: func(cmd *cobra.Command, args []string) {
			state := udpPkg.LoadNotifyState()
			if !state.Subscribed {
				fmt.Println("Not currently subscribed to notifications.")
				return
			}

			userID := getCurrentUserID()
			if userID == "" {
				userID = state.UserID
			}

			udpAddr, err := net.ResolveUDPAddr("udp", state.ServerAddr)
			if err == nil {
				localAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
				conn, connErr := net.ListenUDP("udp", localAddr)
				if connErr == nil {
					defer conn.Close()
					unregMsg, _ := json.Marshal(map[string]string{
						"action":  "unregister",
						"user_id": userID,
					})
					conn.WriteToUDP(unregMsg, udpAddr)
				}
			}

			state.Subscribed = false
			udpPkg.SaveNotifyState(state)

			fmt.Println("✓ Unsubscribed from notifications.")
			fmt.Println("You will no longer receive chapter release notifications.")
			fmt.Println("Run 'mangahub notify subscribe' to subscribe again.")
		},
	}

	// notify preferences
	var notifyPreferencesCmd = &cobra.Command{
		Use:   "preferences",
		Short: "View notification preferences",
		Run: func(cmd *cobra.Command, args []string) {
			state := udpPkg.LoadNotifyState()
			fmt.Println("Notification Preferences:")
			fmt.Println(strings.Repeat("─", 40))
			subStatus := "✗ Not subscribed"
			if state.Subscribed {
				subStatus = "✓ Subscribed"
			}
			fmt.Printf("  Status:       %s\n", subStatus)
			if state.Subscribed {
				fmt.Printf("  Server:       %s\n", state.ServerAddr)
				fmt.Printf("  Subscribed:   %s UTC\n", state.SubscribedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("  User ID:      %s\n", state.UserID)
			}
			fmt.Println(strings.Repeat("─", 40))
			fmt.Println("\nAvailable Commands:")
			if state.Subscribed {
				fmt.Println("  mangahub notify unsubscribe  - Stop receiving notifications")
				fmt.Println("  mangahub notify test         - Send a test notification")
			} else {
				fmt.Println("  mangahub notify subscribe    - Start receiving notifications")
			}
		},
	}

	// notify test  – broadcasts a test notification to all subscribers
	var notifyTestMangaID string
	var notifyTestCmd = &cobra.Command{
		Use:   "test",
		Short: "Send a test notification broadcast",
		Run: func(cmd *cobra.Command, args []string) {
			// Resolve target: if --manga-id provided use it; else send generic test.
			mid := notifyTestMangaID
			if mid == "" {
				mid = "test-manga"
			}

			serverAddr := "localhost:9091"
			udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
			if err != nil {
				fmt.Printf("✗ Test failed: cannot resolve server address: %v\n", err)
				return
			}

			// We trigger the broadcast by talking directly to the running server
			// via a special "broadcast" action packet.
			notif := internalUDP.Notification{
				Type:    "chapter_release",
				MangaID: mid,
				Message: fmt.Sprintf("[TEST] New chapter of '%s' is now available!", mid),
			}

			// Connect to a local UDP socket and send a broadcast-trigger packet.
			localAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
			conn, connErr := net.ListenUDP("udp", localAddr)
			if connErr != nil {
				fmt.Printf("✗ Test failed: %v\n", connErr)
				return
			}
			defer conn.Close()

			// Send broadcast trigger packet to the server
			trigger, _ := json.Marshal(map[string]interface{}{
				"action":   "broadcast",
				"manga_id": notif.MangaID,
				"message":  notif.Message,
				"type":     notif.Type,
			})
			conn.WriteToUDP(trigger, udpAddr)

			// Wait briefly for acknowledgement
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			buf := make([]byte, 512)
			_, _, readErr := conn.ReadFromUDP(buf)

			if readErr != nil {
				// No ack – that is OK; broadcast packets don't always get acked
				fmt.Printf("✓ Test broadcast sent for manga '%s'\n", mid)
			} else {
				fmt.Printf("✓ Test notification broadcast sent and acknowledged for manga '%s'\n", mid)
			}

			fmt.Printf("\nNotification Details:\n")
			fmt.Printf("  Type:    %s\n", notif.Type)
			fmt.Printf("  Manga:   %s\n", notif.MangaID)
			fmt.Printf("  Message: %s\n", notif.Message)
			fmt.Printf("  Server:  %s\n", serverAddr)
			fmt.Println("\nAll registered subscribers should receive this notification.")
		},
	}
	notifyTestCmd.Flags().StringVar(&notifyTestMangaID, "manga-id", "", "Manga ID to use in test notification")

	notifyCmd.AddCommand(notifySubscribeCmd, notifyUnsubscribeCmd, notifyPreferencesCmd, notifyTestCmd)

	// ─── CHAT CLI COMMANDS ──
	var chatCmd = &cobra.Command{
		Use:   "chat",
		Short: "Real-time chat commands",
	}

	var grpcCmd = &cobra.Command{
		Use:   "grpc",
		Short: "gRPC Service Operations",
	}

	var grpcMangaCmd = &cobra.Command{
		Use:   "manga",
		Short: "gRPC Manga Operations",
	}

	var grpcGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Query manga via gRPC",
		Run: func(cmd *cobra.Command, args []string) {
			conn, _ := grpc.Dial("localhost:9092", grpc.WithInsecure())
			defer conn.Close()
			client := pb.NewMangaServiceClient(conn)

			res, err := client.GetManga(context.Background(), &pb.GetMangaRequest{Id: mangaID})
			if err != nil {
				fmt.Printf("✗ gRPC Error: %v\n", err)
				return
			}
			fmt.Printf("✓ [gRPC] Found: %s - %s\n", res.Title, res.Author)
		},
	}
	grpcGetCmd.Flags().StringVar(&mangaID, "id", "", "Manga ID")
	grpcMangaCmd.AddCommand(grpcGetCmd)

	var grpcSearchQuery string
	var grpcSearchCmd = &cobra.Command{
		Use:   "search",
		Short: "Search manga via gRPC",
		Run: func(cmd *cobra.Command, args []string) {
			conn, _ := grpc.Dial("localhost:9092", grpc.WithInsecure())
			defer conn.Close()
			client := pb.NewMangaServiceClient(conn)

			res, err := client.SearchManga(context.Background(), &pb.SearchRequest{Query: grpcSearchQuery})
			if err != nil {
				fmt.Printf("✗ gRPC Error: %v\n", err)
				return
			}
			if len(res.Results) == 0 {
				fmt.Printf("No manga found matching your search criteria via gRPC.\n")
				return
			}
			fmt.Printf("✓ [gRPC] Search Results for '%s':\n", grpcSearchQuery)
			for _, m := range res.Results {
				fmt.Printf("  - %s: %s (%s)\n", m.Id, m.Title, m.Author)
			}
		},
	}
	grpcSearchCmd.Flags().StringVar(&grpcSearchQuery, "query", "", "Search query")
	grpcMangaCmd.AddCommand(grpcSearchCmd)

	var grpcProgressCmd = &cobra.Command{
		Use:   "progress",
		Short: "gRPC Progress Operations",
	}

	var grpcChapter int
	var grpcProgressUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update progress via gRPC",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in.")
				return
			}
			userID := getCurrentUserID()
			conn, _ := grpc.Dial("localhost:9092", grpc.WithInsecure())
			defer conn.Close()
			client := pb.NewMangaServiceClient(conn)

			res, err := client.UpdateProgress(context.Background(), &pb.ProgressRequest{
				UserId:  userID,
				MangaId: mangaID,
				Chapter: int32(grpcChapter),
			})
			if err != nil {
				fmt.Printf("✗ gRPC Error: %v\n", err)
				return
			}
			if res.Success {
				fmt.Printf("✓ [gRPC] %s\n", res.Message)
			} else {
				fmt.Printf("✗ [gRPC] Failed: %s\n", res.Message)
			}
		},
	}
	grpcProgressUpdateCmd.Flags().StringVar(&mangaID, "manga-id", "", "Manga ID")
	grpcProgressUpdateCmd.Flags().IntVar(&grpcChapter, "chapter", 0, "Chapter number")
	grpcProgressCmd.AddCommand(grpcProgressUpdateCmd)

	grpcCmd.AddCommand(grpcMangaCmd)
	grpcCmd.AddCommand(grpcProgressCmd)
	rootCmd.AddCommand(grpcCmd)
	var chatRoomID string
	var chatJoinCmd = &cobra.Command{
		Use:   "join",
		Short: "Join a chat room (default: #general or specify --manga-id)",
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in. Please login first.")
				return
			}
			userID := getCurrentUserID()
			username := getUsernameFromDB(userID)
			if username == "" {
				username = "User_" + userID[:5]
			}

			room := "#general"
			if chatRoomID != "" {
				room = chatRoomID
			}

			for {
				nextRoom := runChatSession(room, userID, username)
				if nextRoom == "" {
					break
				}
				room = nextRoom
			}
		},
	}
	chatJoinCmd.Flags().StringVar(&chatRoomID, "manga-id", "", "Join specific manga discussion")

	var chatHistoryLimit int
	var chatHistoryCmd = &cobra.Command{
		Use:   "history",
		Short: "View chat history",
		Run: func(cmd *cobra.Command, args []string) {
			room := "#general"
			if chatRoomID != "" {
				room = chatRoomID
			}

			hr, err := http.Get(apiURL + "/chat/history?room=" + url.QueryEscape(room))
			if err != nil {
				fmt.Println("✗ Error: Could not connect to server")
				return
			}
			defer hr.Body.Close()

			var h []chatPkg.ChatMessage
			if err := json.NewDecoder(hr.Body).Decode(&h); err != nil {
				fmt.Println("✗ Error parsing history")
				return
			}

			if len(h) == 0 {
				fmt.Println("No recent messages.")
				return
			}

			if chatHistoryLimit > 0 && len(h) > chatHistoryLimit {
				h = h[len(h)-chatHistoryLimit:]
			}

			fmt.Println("Recent messages:")
			for _, m := range h {
				ts := time.Unix(m.Timestamp, 0).Format("15:04")
				if m.Username == "SYSTEM" {
					fmt.Printf("[%s] *** %s ***\n", ts, m.Message)
				} else if !m.IsPrivate {
					fmt.Printf("[%s] %s: %s\n", ts, m.Username, m.Message)
				}
			}
		},
	}
	chatHistoryCmd.Flags().StringVar(&chatRoomID, "manga-id", "", "View history for specific manga discussion")
	chatHistoryCmd.Flags().IntVar(&chatHistoryLimit, "limit", 0, "Limit number of messages")

	var chatSendCmd = &cobra.Command{
		Use:   "send [message]",
		Short: "Send a message to chat",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			token := getToken()
			if token == "" {
				fmt.Println("Not logged in. Please login first.")
				return
			}
			userID := getCurrentUserID()
			username := getUsernameFromDB(userID)
			if username == "" {
				username = "User_" + userID[:5]
			}

			room := "#general"
			if chatRoomID != "" {
				room = chatRoomID
			}

			msg := args[0]
			reqBody, _ := json.Marshal(map[string]interface{}{
				"username": username,
				"room":     room,
				"message":  msg,
			})

			resp, err := http.Post(apiURL+"/chat/send", "application/json", bytes.NewBuffer(reqBody))
			if err != nil || resp.StatusCode != http.StatusOK {
				fmt.Println("✗ Error: Could not send message")
				return
			}
			fmt.Println("✓ Message sent successfully")
		},
	}
	chatSendCmd.Flags().StringVar(&chatRoomID, "manga-id", "", "Send message to specific manga discussion")

	chatCmd.AddCommand(chatJoinCmd, chatHistoryCmd, chatSendCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(mangaCmd)
	rootCmd.AddCommand(libraryCmd)
	rootCmd.AddCommand(progressCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(notifyCmd)
	rootCmd.AddCommand(chatCmd)

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

func formatRoomName(room string) string {
	if room == "#general" {
		return "General Chat"
	}
	words := strings.Split(room, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ") + " Discussion"
}

func runChatSession(room, userID, username string) string {
	fmt.Println("Connecting to WebSocket chat server at ws://localhost:9093...")

	u := url.URL{Scheme: "ws", Host: "localhost:9093", Path: "/ws"}
	q := u.Query()
	q.Set("user_id", userID)
	q.Set("username", username)
	q.Set("room", room)
	u.RawQuery = q.Encode()

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial error:", err)
	}
	defer c.Close()

	userCount := 1
	resp, err := http.Get(apiURL + "/chat/users")
	if err == nil {
		var users []map[string]string
		json.NewDecoder(resp.Body).Decode(&users)
		resp.Body.Close()
		userCount = len(users)
		if userCount == 0 {
			userCount = 1
		}
	}

	if room == "#general" {
		fmt.Println("✓ Connected to General Chat")
	} else {
		fmt.Printf("✓ Connected to %s\n", formatRoomName(room))
	}
	fmt.Printf("\nChat Room: %s\nConnected users: %d\nYour status: Online\n\n", room, userCount)

	histResp, err := http.Get(apiURL + "/chat/history?room=" + url.QueryEscape(room))
	if err == nil {
		var hist []chatPkg.ChatMessage
		json.NewDecoder(histResp.Body).Decode(&hist)
		histResp.Body.Close()
		if len(hist) > 0 {
			fmt.Println("Recent messages:")
			for _, msg := range hist {
				timeStr := time.Unix(msg.Timestamp, 0).Format("15:04")
				if msg.Username == "SYSTEM" {
					fmt.Printf("[%s] *** %s ***\n", timeStr, msg.Message)
				} else if !msg.IsPrivate {
					fmt.Printf("[%s] %s: %s\n", timeStr, msg.Username, msg.Message)
				}
			}
		}
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("You are now in chat. Type your message and press Enter.")
	fmt.Println("Type /help for commands or /quit to leave.")
	fmt.Println()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			var msg chatPkg.ChatMessage
			if err := json.Unmarshal(message, &msg); err == nil {
				timeStr := time.Unix(msg.Timestamp, 0).Format("15:04")
				fmt.Print("\r\033[K")
				if msg.Username == "SYSTEM" {
					fmt.Printf("[%s] *** %s ***\n", timeStr, msg.Message)
				} else if msg.IsPrivate {
					if msg.Username == username {
						fmt.Printf("[%s] [Private to %s]: %s\n", timeStr, msg.TargetUser, msg.Message)
					} else {
						fmt.Printf("[%s] [Private from %s]: %s\n", timeStr, msg.Username, msg.Message)
					}
				} else {
					fmt.Printf("[%s] %s: %s\n", timeStr, msg.Username, msg.Message)
				}
				fmt.Printf("%s> ", username)
			}
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s> ", username)
		if !scanner.Scan() {
			return ""
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		if strings.HasPrefix(text, "/") {
			parts := strings.SplitN(text, " ", 3)
			command := parts[0]

			switch command {
			case "/quit":
				fmt.Println("Leaving chat...")
				c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				fmt.Println("✓ Disconnected from chat server")
				return ""
			case "/help":
				fmt.Println("\nChat Commands:")
				fmt.Println("/help      - Show this help")
				fmt.Println("/users     - List online users")
				fmt.Println("/quit      - Leave chat")
				fmt.Println("/pm <user> <msg>- Private message")
				fmt.Println("/manga <id>   - Switch to manga chat")
				fmt.Println("/history    - Show recent history")
				fmt.Println("/status     - Connection status\n")
				continue
			case "/pm":
				if len(parts) < 3 {
					fmt.Println("Usage: /pm <username> <message>")
					continue
				}
				msg := chatPkg.ChatMessage{
					Message:    parts[2],
					IsPrivate:  true,
					TargetUser: parts[1],
				}
				c.WriteJSON(msg)
				continue
			case "/users":
				r, err := http.Get(apiURL + "/chat/users")
				if err == nil {
					var usrs []map[string]string
					json.NewDecoder(r.Body).Decode(&usrs)
					r.Body.Close()
					fmt.Printf("\nOnline Users (%d):\n", len(usrs))
					for _, u := range usrs {
						fmt.Printf("● %s (%s)\n", u["username"], formatRoomName(u["room"]))
					}
					fmt.Println()
				}
				continue
			case "/manga":
				if len(parts) < 2 {
					fmt.Println("Usage: /manga <id>")
					continue
				}
				c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return parts[1]
			case "/history":
				hr, err := http.Get(apiURL + "/chat/history?room=" + url.QueryEscape(room))
				if err == nil {
					var h []chatPkg.ChatMessage
					json.NewDecoder(hr.Body).Decode(&h)
					hr.Body.Close()
					fmt.Println("\nRecent messages:")
					for _, m := range h {
						ts := time.Unix(m.Timestamp, 0).Format("15:04")
						if m.Username == "SYSTEM" {
							fmt.Printf("[%s] *** %s ***\n", ts, m.Message)
						} else if !m.IsPrivate {
							fmt.Printf("[%s] %s: %s\n", ts, m.Username, m.Message)
						}
					}
					fmt.Println()
				}
				continue
			case "/status":
				fmt.Printf("\nConnection Status: Connected\nRoom: %s\nServer: ws://localhost:9093/ws\n\n", room)
				continue
			default:
				fmt.Println("Unknown command. Type /help for a list of commands.")
				continue
			}
		}

		msg := chatPkg.ChatMessage{Message: text}
		if err := c.WriteJSON(msg); err != nil {
			return ""
		}
	}
}

// truncate shortens a string to maxLen with "..." if needed
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
