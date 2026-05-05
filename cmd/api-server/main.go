package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"mangahub/internal/auth"
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

	r := gin.Default()

	// --- CÁC ENDPOINT AUTHENTICATION ---
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

	// Middleware xác thực JWT
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

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)

	if len(os.Args) == 1 {
		// Run server by default if no arguments are provided, like a traditional api-server
		runServer()
		return
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
