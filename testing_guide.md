# MangaHub Week 2 - Hướng Dẫn Test

> [!IMPORTANT]
> Mở **2 terminal**: một chạy server, một chạy CLI commands.

## Bước 0: Khởi động Server

```bash
# Terminal 1 - Chạy server
.\mangahub.exe server start
```

Kết quả mong đợi: `Server is running on port 8080...` và hiển thị 15 routes.

---

## Bước 1: Manga Search & Info (Không cần login)

### 1.1 Tìm kiếm manga
```bash
# Tìm theo tên (có khoảng trắng)
.\mangahub.exe manga search "Attack on Titan"

# Tìm theo từ khóa ngắn
.\mangahub.exe manga search "one"

# Tìm với filter genre
.\mangahub.exe manga search "action" --genre Shounen

# Tìm với filter status (viết thường)
.\mangahub.exe manga search "one" --status ongoing

# Giới hạn kết quả
.\mangahub.exe manga search "a" --limit 5
```

### 1.2 Xem chi tiết manga
```bash
.\mangahub.exe manga info one-piece
.\mangahub.exe manga info attack-on-titan
.\mangahub.exe manga info death-note

# Test manga không tồn tại
.\mangahub.exe manga info nonexistent
```

### 1.3 Liệt kê tất cả manga
```bash
.\mangahub.exe manga list
.\mangahub.exe manga list --genre Shoujo
.\mangahub.exe manga list --genre Seinen
```

---

## Bước 2: Đăng ký & Đăng nhập

### 2.1 Đăng ký
```bash
.\mangahub.exe auth register --username testuser --email test@example.com
# Nhập password: Test1234 (ẩn)
# Nhập lại password: Test1234
```

### 2.2 Đăng nhập
```bash
.\mangahub.exe auth login --username testuser
# Nhập password: Test1234
```

### 2.3 Kiểm tra trạng thái
```bash
.\mangahub.exe auth status
```

---

## Bước 3: Library Operations (Cần login)

### 3.1 Thêm manga vào library
```bash
# Thêm One Piece đang đọc
.\mangahub.exe library add --manga-id one-piece --status reading

# Thêm Death Note đã đọc xong
.\mangahub.exe library add --manga-id death-note --status completed

# Thêm manga dự định đọc
.\mangahub.exe library add --manga-id demon-slayer --status plan-to-read

# Test thêm manga không tồn tại
.\mangahub.exe library add --manga-id fake-manga --status reading

# Test thêm manga đã có trong library (duplicate)
.\mangahub.exe library add --manga-id one-piece --status reading
```

### 3.2 Xem library
```bash
# Xem toàn bộ library
.\mangahub.exe library list

# Lọc theo trạng thái
.\mangahub.exe library list --status reading
.\mangahub.exe library list --status completed
.\mangahub.exe library list --status plan-to-read
```

### 3.3 Xóa manga khỏi library
```bash
.\mangahub.exe library remove --manga-id demon-slayer

# Kiểm tra đã xóa
.\mangahub.exe library list
```

---

## Bước 4: Progress Tracking (Cần login)

### 4.1 Cập nhật tiến độ đọc
```bash
# Cập nhật One Piece lên chapter 500
.\mangahub.exe progress update --manga-id one-piece --chapter 500

# Cập nhật tiếp lên 1000
.\mangahub.exe progress update --manga-id one-piece --chapter 1000

# Test chapter vượt quá tổng (One Piece có 1100 chapters)
.\mangahub.exe progress update --manga-id one-piece --chapter 9999

# Test manga chưa có trong library
.\mangahub.exe progress update --manga-id naruto --chapter 50
```

### 4.2 Kiểm tra library sau khi update
```bash
.\mangahub.exe library list
```

---

## Bước 5: Test qua API trực tiếp (PowerShell)

### 5.1 Search API
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/manga?q=one" | ConvertTo-Json -Depth 5
Invoke-RestMethod -Uri "http://localhost:8080/manga?genre=Shounen&status=ongoing" | ConvertTo-Json -Depth 5
```

### 5.2 Get manga by ID
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/manga/one-piece" | ConvertTo-Json
```

### 5.3 Login & get token
```powershell
$login = Invoke-RestMethod -Uri "http://localhost:8080/auth/login" -Method POST -Body '{"username":"testuser","password":"Test1234"}' -ContentType "application/json"
$token = $login.token
Write-Host "Token: $token"
```

### 5.4 Library operations via API
```powershell
$headers = @{"Authorization"="Bearer $token"; "Content-Type"="application/json"}

# Add to library
Invoke-RestMethod -Uri "http://localhost:8080/users/library" -Method POST -Body '{"manga_id":"naruto","status":"reading","current_chapter":100}' -Headers $headers | ConvertTo-Json

# Get library
Invoke-RestMethod -Uri "http://localhost:8080/users/library" -Method GET -Headers $headers | ConvertTo-Json -Depth 5

# Update progress
Invoke-RestMethod -Uri "http://localhost:8080/users/progress" -Method PUT -Body '{"manga_id":"naruto","chapter":300}' -Headers $headers | ConvertTo-Json

# Remove from library
Invoke-RestMethod -Uri "http://localhost:8080/users/library/naruto" -Method DELETE -Headers $headers | ConvertTo-Json
```

### 5.5 JWT protection test
```powershell
# Gọi API cần auth nhưng không có token → 401
Invoke-RestMethod -Uri "http://localhost:8080/users/library" -Method GET
```

---

## Bước 6: Logout & Re-test

```bash
.\mangahub.exe auth logout

# Thử gọi library khi chưa login → báo lỗi
.\mangahub.exe library list

# Login lại
.\mangahub.exe auth login --username testuser
```
# MangaHub Phase 2 - Testing & Execution Guide (TCP Sync)
## Step 0: Start the System

Run the main server to enable both HTTP and TCP functionalities.

```bash
# Terminal 1
.\mangahub.exe server start
```
## Step 1: Establish TCP Sync Connection

1.1 Connect to Sync Server
```bash
# Terminal 2
.\mangahub.exe sync connect

Expected_Result: ✓ Connected successfully!, Server: localhost:8081, and a unique Session ID.
```

1.2 Enable Real-time Monitoring 
```bash
# Terminal 2
.\mangahub.exe sync monitor
Expected_Result: Monitoring real-time sync updates... (Press Ctrl+C to exit).
```

## Step 2: Test Real-time Synchronization

2.1 Update Progress from another terminal
```bash
# Terminal 3
.\mangahub.exe auth login --username testuser
.\mangahub.exe progress update --manga-id one-piece --chapter 505

Expected_Result: Expected Result: A broadcast message should appear instantly in Terminal 2:
[HH:mm:ss] ← Device updated: one-piece → Chapter 505
```

## Step 3: Statistics & Progress History

3.1 Check Sync Statistics
```bash
# Terminal 3
.\mangahub.exe sync status
Expected_Result:
Connection: ✓ Active

Messages sent: Total count of updates sent by this client.

Messages received: Total count of updates received from other devices.

Last sync update: Should show one-piece ch. 505.
```

3.2 View Progress History
```bash
# View all library entries
.\mangahub.exe progress history

# Filter history by specific Manga ID
.\mangahub.exe progress history --manga-id one-piece
Expected_Result: Displays a clean list of manga with their respective chapters, reading status, and the last update timestamp.
```
## Step 4: Manual Sync & Disconnection

4.1 Manual Synchronization
```bash
# Terminal 3
.\mangahub.exe progress sync
Expected_Result: Displays Syncing progress with server... followed by ✓ Progress successfully synchronized across all devices.
```

4.2 Disconnect TCP
```bash
# Terminal 3
.\mangahub.exe sync disconnect
Expected_Result: Status in sync status will change to Disconnected
```

## Checklist Tóm Tắt

| Feature | Command Test | Expected |
|---------|-------------|----------|
| Manga search | `manga search "Attack on Titan"` | Tìm thấy 1 kết quả |
| Manga search + filter | `manga search "a" --genre Shounen` | Chỉ manga Shounen |
| Manga info | `manga info one-piece` | Hiển thị chi tiết |
| Manga list | `manga list` | 35 manga |
| Register | `auth register --username ...` | Account created |
| Login | `auth login --username ...` | Login successful |
| Auth status | `auth status` | Hiển thị user info |
| Library add | `library add --manga-id one-piece` | Added to library |
| Library list | `library list` | Hiển thị entries |
| Library remove | `library remove --manga-id ...` | Removed |
| Progress update | `progress update --manga-id ... --chapter 500` | Updated |
| JWT protection | API call without token | 401 Unauthorized |
| Invalid chapter | `progress update --chapter 9999` | Error: exceeds total |
| Duplicate add | `library add` manga đã có | Error: already in library |
