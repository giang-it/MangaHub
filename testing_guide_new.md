# MangaHub - Comprehensive Testing Guide

> [!IMPORTANT]
> **Requirement:** The server must be running before executing any CLI command.
> Open **Terminal 1** to run the server; use other terminals for CLI testing.

---

## STEP 0: Start the Server

```bash
# Terminal 1 - Run server (keep this terminal open throughout testing)
.\mangahub.exe server start
```

**Expected output:**
```
SQLite connection successful and schema is ready.
Server is running on port 8080...
WebSocket Chat Server listening on ws://localhost:9093
UDP Notification Server listening on udp://localhost:9091
gRPC Internal Service listening on grpc://localhost:9092
TCP Sync Server listening on tcp://localhost:8081
```

### Check server status

```bash
# Any terminal
.\mangahub.exe server status
```

**Expected output:**
```
MangaHub Server Status
────────────────────────────────────────────────────────────
✓ HTTP API:        Online  (localhost:8080)
✓ TCP Sync:        Online  (localhost:8081)
✓ UDP Notify:      Online  (localhost:9091)
✓ WS Chat:         Online  (localhost:9093)
✓ gRPC Service:    Online  (localhost:9092)
────────────────────────────────────────────────────────────
Overall System Health: ✓ Healthy
```

---

## PART 1: AUTHENTICATION (auth)

> Use `--session` to run multiple accounts in parallel across different terminals.
> `--session c1` = alice, `--session c2` = bob, `--session c3` = charlie.

---

### 1.1 Register account (auth register)

**Syntax:**
```bash
.\mangahub.exe auth register --username <name> --email <email>
# Then enter password (hidden) and confirm password
```

**Test Case 1: Successful registration**
```bash
.\mangahub.exe auth register --username alice --email alice@example.com
# Enter password: Test1234
# Confirm:        Test1234
```
Expected output:
```
✓ Account created successfully!
User ID: <uuid>
Username: alice
Email: alice@example.com
Created: 2026-xx-xx xx:xx:xx UTC

Please login to start using MangaHub:
mangahub auth login --username alice
```

**Test Case 2: Register second user (bob)**
```bash
.\mangahub.exe auth register --username bob --email bob@example.com
# Enter password: Test1234
# Confirm:        Test1234
```

**Test Case 3: Weak password (< 8 chars, no uppercase/numbers)**
```bash
.\mangahub.exe auth register --username test2 --email test2@example.com
# Enter password: abc
# Confirm:        abc
```
Expected output:
```
✗ Registration failed: Password too weak
Password must be at least 8 characters with mixed case and numbers
```

**Test Case 4: Password mismatch**
```bash
.\mangahub.exe auth register --username test3 --email test3@example.com
# Enter password: Test1234
# Confirm (wrong): Test5678
```
Expected output:
```
✗ Registration failed: Passwords do not match
```

**Test Case 5: Invalid email**
```bash
.\mangahub.exe auth register --username test4 --email not-an-email
# Enter password: Test1234
# Confirm:        Test1234
```
Expected output:
```
✗ Registration failed: Invalid email format
```

**Test Case 6: Username already exists**
```bash
.\mangahub.exe auth register --username alice --email another@example.com
# Enter password: Test1234
# Confirm:        Test1234
```
Expected output:
```
✗ Registration failed: Username or Email already exists
```

---

### 1.2 Login (auth login)

**Test Case 1: Login with username (alice - session c1)**
```bash
# Terminal 2 - session c1 (alice)
.\mangahub.exe --session c1 auth login --username alice
# Enter password: Test1234
```
Expected output:
```
✓ Login successful!
Welcome back, alice!

Session Details:
Token expires: 2026-xx-xx xx:xx:xx UTC (24 hours)
Permissions: read, write, sync

Auto-sync: enabled
Notifications: enabled

Ready to use MangaHub! Try:
mangahub manga search "your favorite manga"
```

**Test Case 2: Login second user (bob - session c2)**
```bash
# Terminal 3 - session c2 (bob) - runs in parallel with alice
.\mangahub.exe --session c2 auth login --username bob
# Enter password: Test1234
```

**Test Case 3: Login with email**
```bash
.\mangahub.exe --session c1 auth login --email alice@example.com
# Enter password: Test1234
```

**Test Case 4: Wrong password**
```bash
.\mangahub.exe --session c1 auth login --username alice
# Enter password: WrongPass
```
Expected output:
```
✗ Login failed: Invalid credentials
```

---

### 1.3 Check login status (auth status)

```bash
# Check session c1 (alice)
.\mangahub.exe --session c1 auth status

# Check session c2 (bob)
.\mangahub.exe --session c2 auth status
```
Expected output (for alice):
```
Authentication Status: Active
User ID: <uuid>
Username: alice
Created At: 2026-xx-xx xx:xx:xx
```

> **Note:** The two commands above return info for two different users, proving session isolation works correctly.

---

### 1.4 Change password (auth change-password)

```bash
.\mangahub.exe --session c1 auth change-password
# Enter Current Password: Test1234
# Enter New Password:     NewPass5678
# Confirm:                NewPass5678
```
Expected output:
```
✓ Password changed successfully!
```

**Login again with new password to confirm:**
```bash
.\mangahub.exe --session c1 auth login --username alice
# Enter password: NewPass5678
```

**Change back to original password for further testing:**
```bash
.\mangahub.exe --session c1 auth change-password
# Enter Current Password: NewPass5678
# Enter New Password:     Test1234
# Confirm:                Test1234
```

---

### 1.5 Logout (auth logout)

```bash
# Logout session c1 (alice)
.\mangahub.exe --session c1 auth logout
```
Expected output:
```
Logged out successfully.
```

**Try calling an auth-required command after logout:**
```bash
.\mangahub.exe --session c1 library list
```
Expected output:
```
Not logged in. Please login first.
```

**Login again to continue testing:**
```bash
.\mangahub.exe --session c1 auth login --username alice
# Enter password: Test1234
```

---

### 1.6 Test via HTTP API directly (PowerShell)

**Register via API:**
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/auth/register" `
  -Method POST `
  -Body '{"username":"apiuser","email":"apiuser@example.com","password":"Test1234"}' `
  -ContentType "application/json"
```

**Login via API and retrieve token:**
```powershell
$login = Invoke-RestMethod -Uri "http://localhost:8080/auth/login" `
  -Method POST `
  -Body '{"username":"alice","password":"Test1234"}' `
  -ContentType "application/json"
$token = $login.token
Write-Host "Token: $token"
```

**Check JWT protection (call auth-required API without token → 401):**
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/users/library" -Method GET
```
Expected output: HTTP 401 Unauthorized

---

### Auth Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Successful registration | `auth register --username alice ...` | ✓ Account created |
| 2 | Register second user | `auth register --username bob ...` | ✓ Account created |
| 3 | Weak password | Register with password "abc" | ✗ Password too weak |
| 4 | Password mismatch | Enter wrong confirm | ✗ Passwords do not match |
| 5 | Invalid email | `--email not-an-email` | ✗ Invalid email format |
| 6 | Duplicate username | Register alice again | ✗ Already exists |
| 7 | Successful login | `--session c1 auth login --username alice` | ✓ Login successful |
| 8 | Session isolation | `--session c1 auth status` vs `--session c2 auth status` | Shows 2 different users |
| 9 | Wrong password | Login with wrong password | ✗ Invalid credentials |
| 10 | Change password | `--session c1 auth change-password` | ✓ Password changed |
| 11 | Logout | `--session c1 auth logout` | Logged out |
| 12 | Call API after logout | `--session c1 library list` after logout | Not logged in |
| 13 | JWT protection | API call without token | 401 Unauthorized |

---

## PART 2: MANGA SEARCH & INFO

> Manga commands **do not require login** (public API). No `--session` needed.

---

### 2.1 Search manga (manga search)

**Test Case 1: Basic search by name**
```bash
.\mangahub.exe manga search "One Piece"
```
Expected output:
```
Searching for "One Piece"...

Found 1 results:

ID                     Title                        Author             Status       Chapters
------------------------------------------------------------------------------------------
one-piece              One Piece                    Oda Eiichiro       Ongoing      1100
```

**Test Case 2: Short keyword search (multiple results)**
```bash
.\mangahub.exe manga search "attack"
```

**Test Case 3: Search + filter by genre**
```bash
.\mangahub.exe manga search "a" --genre Shounen
```
Expected output: Only manga with Shounen genre.

**Test Case 4: Search + filter by status**
```bash
.\mangahub.exe manga search "naruto" --status completed
```

**Test Case 5: Limit results**
```bash
.\mangahub.exe manga search "a" --limit 5
```
Expected output: Maximum 5 results.

**Test Case 6: Pagination**
```bash
.\mangahub.exe manga search "a" --limit 5 --page 2
```

**Test Case 7: Sort results**
```bash
# Sort by chapter count descending
.\mangahub.exe manga search "a" --sort-by total_chapters --order desc
```

**Test Case 8: Combine multiple filters**
```bash
.\mangahub.exe manga search "dragon" --genre Action --status ongoing --limit 10
```

**Test Case 9: No results found**
```bash
.\mangahub.exe manga search "xyznonexistent12345"
```
Expected output:
```
No manga found matching your search criteria.

Suggestions:
- Check spelling and try again
- Use broader search terms
- Browse by genre: mangahub manga search "" --genre action
```

---

### 2.2 View manga details (manga info)

**Test Case 1: View details - not logged in**
```bash
.\mangahub.exe manga info one-piece
```
Expected output (no Progress section):
```
────────────────────────────────────────────────────────────
  ONE PIECE
────────────────────────────────────────────────────────────

Basic Information:
  ID:       one-piece
  Title:    One Piece
  Author:   Oda Eiichiro
  Genres:   Action, Adventure, Comedy, Drama, Fantasy, Shounen
  Status:   Ongoing
  Chapters: 1100

Description:
  <manga description>

Actions:
  Add to library: mangahub library add --manga-id one-piece --status reading
  Update progress: mangahub progress update --manga-id one-piece --chapter 1
```

**Test Case 2: View details - logged in (shows Progress section)**
```bash
# Login first
.\mangahub.exe --session c1 auth login --username alice
# Add to library if not already added
.\mangahub.exe --session c1 library add --manga-id one-piece --status reading
# View details
.\mangahub.exe manga info one-piece
```
Expected output: Has additional **Progress** section with status and current chapter.

**Test Case 3: View popular manga IDs**
```bash
.\mangahub.exe manga info attack-on-titan
.\mangahub.exe manga info death-note
.\mangahub.exe manga info naruto
.\mangahub.exe manga info demon-slayer
```

**Test Case 4: Manga does not exist**
```bash
.\mangahub.exe manga info fake-manga-id
```
Expected output:
```
✗ Manga not found: 'fake-manga-id'

Try searching instead:
mangahub manga search "manga title"
```

---

### 2.3 List all manga (manga list)

**Test Case 1: List all (default 20 results)**
```bash
.\mangahub.exe manga list
```
Expected output: Manga list with total entry count.

**Test Case 2: Pagination**
```bash
.\mangahub.exe manga list --page 1 --limit 10
.\mangahub.exe manga list --page 2 --limit 10
```

**Test Case 3: Filter by genre**
```bash
.\mangahub.exe manga list --genre Shoujo
.\mangahub.exe manga list --genre Seinen
.\mangahub.exe manga list --genre Shounen
.\mangahub.exe manga list --genre Action
```

---

### 2.4 Test Manga API directly (PowerShell)

**Search API:**
```powershell
# Basic search
Invoke-RestMethod -Uri "http://localhost:8080/manga?q=one" | ConvertTo-Json -Depth 5

# Search with filter
Invoke-RestMethod -Uri "http://localhost:8080/manga?q=naruto&status=completed" | ConvertTo-Json -Depth 5

# Search with sort
Invoke-RestMethod -Uri "http://localhost:8080/manga?q=a&sort_by=total_chapters&order=desc&limit=5" | ConvertTo-Json -Depth 3
```

**Get Manga by ID:**
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/manga/one-piece" | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/manga/attack-on-titan" | ConvertTo-Json

# Non-existent manga → 404
Invoke-RestMethod -Uri "http://localhost:8080/manga/nonexistent" | ConvertTo-Json
```

**CRUD Manga (requires token):**
```powershell
# Get token
$login = Invoke-RestMethod -Uri "http://localhost:8080/auth/login" `
  -Method POST -Body '{"username":"alice","password":"Test1234"}' `
  -ContentType "application/json"
$token = $login.token
$headers = @{"Authorization"="Bearer $token"; "Content-Type"="application/json"}

# Create new manga
$newManga = '{"id":"my-manga","title":"My Test Manga","author":"Test Author","status":"ongoing","total_chapters":10}'
Invoke-RestMethod -Uri "http://localhost:8080/manga" -Method POST -Body $newManga -Headers $headers | ConvertTo-Json

# Update manga
$update = '{"title":"My Updated Manga","total_chapters":20}'
Invoke-RestMethod -Uri "http://localhost:8080/manga/my-manga" -Method PUT -Body $update -Headers $headers | ConvertTo-Json

# Delete manga
Invoke-RestMethod -Uri "http://localhost:8080/manga/my-manga" -Method DELETE -Headers $headers | ConvertTo-Json
```

---

### Manga Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Basic search | `manga search "One Piece"` | Results found |
| 2 | Short keyword | `manga search "attack"` | Multiple results |
| 3 | Filter by genre | `manga search "a" --genre Shounen` | Only Shounen manga |
| 4 | Filter by status | `manga search "naruto" --status completed` | Only completed manga |
| 5 | Limit results | `manga search "a" --limit 5` | Max 5 results |
| 6 | Pagination | `manga search "a" --limit 5 --page 2` | Page 2 |
| 7 | Sort | `manga search "a" --sort-by total_chapters --order desc` | Correct order |
| 8 | No results | `manga search "xyznonexistent"` | No manga found |
| 9 | Info - not logged in | `manga info one-piece` | No Progress section |
| 10 | Info - logged in | `manga info one-piece` (after login) | Has Progress section |
| 11 | Info - not found | `manga info fake-id` | ✗ Manga not found |
| 12 | List all | `manga list` | Full list |
| 13 | List by genre | `manga list --genre Shoujo` | Only Shoujo manga |
| 14 | CRUD via API | POST/PUT/DELETE /manga | Success with token |

---

## PART 3: LIBRARY OPERATIONS

> All library commands **require login**.
> Use `--session` to isolate tokens when testing multiple users in parallel.

**Preparation: Ensure you are logged in**
```bash
.\mangahub.exe --session c1 auth login --username alice
# Enter password: Test1234
```

---

### 3.1 Add manga to library (library add)

**Test Case 1: Add manga with default status (reading)**
```bash
.\mangahub.exe --session c1 library add --manga-id one-piece --status reading
```
Expected output:
```
✓ Manga added to library!
  Manga: One Piece
  Status: reading
```

**Test Case 2: Add multiple manga with different statuses**
```bash
.\mangahub.exe --session c1 library add --manga-id death-note --status completed
.\mangahub.exe --session c1 library add --manga-id demon-slayer --status plan-to-read
.\mangahub.exe --session c1 library add --manga-id attack-on-titan --status on-hold
.\mangahub.exe --session c1 library add --manga-id naruto --status dropped
```

**Test Case 3: Add non-existent manga**
```bash
.\mangahub.exe --session c1 library add --manga-id fake-manga-xyz --status reading
```
Expected output:
```
✗ Failed to add: manga not found
```

**Test Case 4: Add manga already in library (duplicate)**
```bash
.\mangahub.exe --session c1 library add --manga-id one-piece --status reading
```
Expected output:
```
✗ Failed to add: manga already in library
```

**Test Case 5: Add manga for second user (bob)**
```bash
.\mangahub.exe --session c2 auth login --username bob
.\mangahub.exe --session c2 library add --manga-id one-piece --status reading
.\mangahub.exe --session c2 library add --manga-id naruto --status completed
```

---

### 3.2 View library (library list)

**Test Case 1: View full library**
```bash
.\mangahub.exe --session c1 library list
```
Expected output: Manga list grouped by status (reading, completed, plan-to-read, on-hold, dropped).

**Test Case 2: Filter by status**
```bash
.\mangahub.exe --session c1 library list --status reading
.\mangahub.exe --session c1 library list --status completed
.\mangahub.exe --session c1 library list --status plan-to-read
.\mangahub.exe --session c1 library list --status on-hold
.\mangahub.exe --session c1 library list --status dropped
```

**Test Case 3: Empty library (new user)**
```bash
# Register new user with empty library
.\mangahub.exe auth register --username charlie --email charlie@example.com
.\mangahub.exe --session c3 auth login --username charlie
.\mangahub.exe --session c3 library list
```
Expected output:
```
Your library is empty.

Get started:
  mangahub manga search "your favorite series"
  mangahub library add --manga-id <id> --status reading
```

**Test Case 4: Library isolation - alice and bob have separate libraries**
```bash
.\mangahub.exe --session c1 library list   # alice's library
.\mangahub.exe --session c2 library list   # bob's library
```
Expected output: Two completely different libraries.

---

### 3.3 Update library entry (library update)

**Test Case 1: Change status**
```bash
.\mangahub.exe --session c1 library update --manga-id demon-slayer --status reading
```
Expected output:
```
✓ Library entry updated!
  Status: reading
```

**Test Case 2: Set rating**
```bash
.\mangahub.exe --session c1 library update --manga-id demon-slayer --rating 10
```
Expected output:
```
✓ Library entry updated!
  Rating: 10/10
```

**Test Case 3: Update both status and rating**
```bash
.\mangahub.exe --session c1 library update --manga-id one-piece --status reading --rating 9
```

---

### 3.4 Remove manga from library (library remove)

**Test Case 1: Remove successfully**
```bash
.\mangahub.exe --session c1 library remove --manga-id naruto
```
Expected output:
```
✓ Manga removed from library
```

**Confirm removal:**
```bash
.\mangahub.exe --session c1 library list --status dropped
```

**Test Case 2: Remove manga not in library**
```bash
.\mangahub.exe --session c1 library remove --manga-id fake-manga-xyz
```
Expected output: Error message.

---

### 3.5 Test Library API directly (PowerShell)

```powershell
# Get token
$login = Invoke-RestMethod -Uri "http://localhost:8080/auth/login" `
  -Method POST -Body '{"username":"alice","password":"Test1234"}' `
  -ContentType "application/json"
$token = $login.token
$h = @{"Authorization"="Bearer $token"; "Content-Type"="application/json"}

# Add manga to library
Invoke-RestMethod -Uri "http://localhost:8080/users/library" `
  -Method POST -Body '{"manga_id":"jujutsu-kaisen","status":"reading"}' -Headers $h | ConvertTo-Json

# View full library
Invoke-RestMethod -Uri "http://localhost:8080/users/library" -Method GET -Headers $h | ConvertTo-Json -Depth 5

# Filter by status
Invoke-RestMethod -Uri "http://localhost:8080/users/library?status=reading" -Method GET -Headers $h | ConvertTo-Json -Depth 3

# Update library entry (status + rating)
Invoke-RestMethod -Uri "http://localhost:8080/users/library" `
  -Method PUT -Body '{"manga_id":"jujutsu-kaisen","status":"completed","rating":9}' -Headers $h | ConvertTo-Json

# Remove manga from library
Invoke-RestMethod -Uri "http://localhost:8080/users/library/jujutsu-kaisen" -Method DELETE -Headers $h | ConvertTo-Json
```

---

### Library Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Add manga (reading) | `--session c1 library add --manga-id one-piece --status reading` | ✓ Added |
| 2 | Add manga (completed) | `--session c1 library add --manga-id death-note --status completed` | ✓ Added |
| 3 | Add non-existent manga | `--session c1 library add --manga-id fake-manga` | ✗ manga not found |
| 4 | Duplicate add | `--session c1 library add --manga-id one-piece` again | ✗ already in library |
| 5 | View full library | `--session c1 library list` | Grouped by status |
| 6 | Filter by status | `--session c1 library list --status reading` | Only reading manga |
| 7 | Empty library | `--session c3 library list` (new user) | Your library is empty |
| 8 | Library isolation | c1 library vs c2 library | Two different libraries |
| 9 | Update status | `--session c1 library update --manga-id ... --status completed` | ✓ Updated |
| 10 | Set rating | `--session c1 library update --manga-id ... --rating 10` | ✓ Rating: 10/10 |
| 11 | Remove manga | `--session c1 library remove --manga-id naruto` | ✓ Removed |
| 12 | Remove not in library | `--session c1 library remove --manga-id fake` | Error |
| 11 | Remove manga | `--session c1 library remove --manga-id naruto` | ✓ Removed |
| 12 | Remove not in library | `--session c1 library remove --manga-id fake` | Error |

---

## PART 4: PROGRESS TRACKING

> All progress commands **require login**.
> Manga must already be in the **library** before updating progress.

**Preparation:**
```bash
# Make sure alice is logged in
.\mangahub.exe --session c1 auth login --username alice
# Enter password: Test1234

# Make sure manga is in library
.\mangahub.exe --session c1 library add --manga-id one-piece --status reading
.\mangahub.exe --session c1 library add --manga-id attack-on-titan --status reading
```

---

### 4.1 Update reading progress (progress update)

**Test Case 1: Basic chapter update**
```bash
.\mangahub.exe --session c1 progress update --manga-id one-piece --chapter 500
```
Expected output:
```
✓ Progress updated successfully!
  Manga: One Piece
  Previous: Chapter 0
  Current:  Chapter 500
  Status:   reading
```

**Test Case 2: Update to a higher chapter**
```bash
.\mangahub.exe --session c1 progress update --manga-id one-piece --chapter 1000
```
Expected output:
```
✓ Progress updated successfully!
  Manga: One Piece
  Previous: Chapter 500
  Current:  Chapter 1000
```

**Test Case 3: Update with volume**
```bash
.\mangahub.exe --session c1 progress update --manga-id one-piece --chapter 50 --volume 12
```
Expected output:
```
✓ Progress updated successfully!
  Manga: One Piece
  Current:  Chapter 50
  Volume:   12
```

**Test Case 4: Update with reading notes**
```bash
.\mangahub.exe --session c1 progress update --manga-id one-piece --chapter 1050 --notes "Great arc!"
```
Expected output: Output includes `Notes: Great arc!`.

**Test Case 5: Chapter exceeds total (One Piece has 1100 chapters)**
```bash
.\mangahub.exe --session c1 progress update --manga-id one-piece --chapter 9999
```
Expected output:
```
✗ Progress update failed: chapter exceeds manga's total chapters (1100)
```

**Test Case 6: Manga not in library**
```bash
.\mangahub.exe --session c1 progress update --manga-id naruto --chapter 50
```
Expected output:
```
✗ Progress update failed: Manga not found in your library
```

**Test Case 7: Bob updates progress independently (--session c2)**
```bash
.\mangahub.exe --session c2 library add --manga-id one-piece --status reading
.\mangahub.exe --session c2 progress update --manga-id one-piece --chapter 300
```
Expected output: Bob updates to chapter 300, independent of alice.

---

### 4.2 View progress history (progress history)

**Test Case 1: View full history**
```bash
.\mangahub.exe --session c1 progress history
```
Expected output:
```
Progress History:
-----------------
- One Piece (one-piece): Chapter 1050, Status: reading
  Notes: Great arc!
  Last Updated: 2026-xx-xx xx:xx:xx

- Attack on Titan (attack-on-titan): Chapter 50, Status: reading
  Volume: 12
  Last Updated: 2026-xx-xx xx:xx:xx
```

**Test Case 2: Filter history by manga ID**
```bash
.\mangahub.exe --session c1 progress history --manga-id one-piece
```
Expected output: Only shows One Piece entry.

**Test Case 3: Progress history isolation between alice and bob**
```bash
.\mangahub.exe --session c1 progress history   # alice: One Piece ch.1050
.\mangahub.exe --session c2 progress history   # bob:   One Piece ch.300
```
> **Key point:** Proves progress data is completely isolated per user.

---

### 4.3 Manual sync (progress sync)

```bash
.\mangahub.exe --session c1 progress sync
```
Expected output:
```
Syncing progress with server...
✓ Progress successfully synchronized across all devices.
```

---

### 4.4 Check sync status (progress sync-status)

```bash
# When TCP is NOT connected
.\mangahub.exe --session c1 progress sync-status
```
Expected output:
```
Progress Sync Status:
Auto-sync: disabled
Conflict resolution: last_write_wins
```

```bash
# Connect TCP first, then check again
.\mangahub.exe sync connect
.\mangahub.exe --session c1 progress sync-status
```
Expected output:
```
Progress Sync Status:
Auto-sync: enabled
Conflict resolution: last_write_wins
Last sync update: one-piece ch. 1050
```

---

### 4.5 Test Progress API directly (PowerShell)

```powershell
# Get token
$login = Invoke-RestMethod -Uri "http://localhost:8080/auth/login" `
  -Method POST -Body '{"username":"alice","password":"Test1234"}' `
  -ContentType "application/json"
$token = $login.token
$h = @{"Authorization"="Bearer $token"; "Content-Type"="application/json"}

# Update progress (chapter + volume + notes)
Invoke-RestMethod -Uri "http://localhost:8080/users/progress" `
  -Method PUT `
  -Body '{"manga_id":"one-piece","chapter":1055,"volume":105,"notes":"Awesome!"}' `
  -Headers $h | ConvertTo-Json

# View library to confirm progress was saved
Invoke-RestMethod -Uri "http://localhost:8080/users/library" `
  -Method GET -Headers $h | ConvertTo-Json -Depth 5

# Chapter exceeds limit → error
Invoke-RestMethod -Uri "http://localhost:8080/users/progress" `
  -Method PUT `
  -Body '{"manga_id":"one-piece","chapter":99999}' `
  -Headers $h | ConvertTo-Json
```

---

### Progress Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Basic chapter update | `--session c1 progress update --manga-id one-piece --chapter 500` | ✓ Previous: 0, Current: 500 |
| 2 | Update higher | `--session c1 progress update --chapter 1000` | ✓ Previous: 500, Current: 1000 |
| 3 | Update + volume | `--session c1 progress update --chapter 50 --volume 12` | ✓ Volume: 12 shown |
| 4 | Update + notes | `--session c1 progress update --chapter 1050 --notes "Great arc!"` | ✓ Notes shown |
| 5 | Exceeds chapter limit | `--session c1 progress update --chapter 9999` | ✗ exceeds total chapters |
| 6 | Manga not in library | `--session c1 progress update --manga-id naruto --chapter 50` | ✗ not found in library |
| 7 | Multi-user update | `--session c2 progress update --chapter 300` | ✓ Bob updates independently |
| 8 | View full history | `--session c1 progress history` | All manga listed |
| 9 | Filter by manga | `--session c1 progress history --manga-id one-piece` | Only One Piece |
| 10 | Progress isolation | c1 vs c2 `progress history` | Two completely different histories |
| 11 | Manual sync | `--session c1 progress sync` | ✓ Synchronized |
| 12 | Sync-status (not connected) | `--session c1 progress sync-status` | Auto-sync: disabled |
| 13 | Sync-status (connected) | `sync connect` → `--session c1 progress sync-status` | Auto-sync: enabled |

---

## PART 5: TCP SYNC (Real-time Progress Synchronization)

> TCP Sync allows **real-time monitoring** whenever any user updates their progress.
> Requires **at least 3 terminals**: T1 runs server, T2 monitors, T3 triggers updates.

---

### 5.1 Connect TCP Sync (sync connect)

**Terminal 2:**
```bash
.\mangahub.exe sync connect
```
Expected output:
```
Connecting to TCP sync server at localhost:8081...
✓ Connected successfully!

Connection Details:
Server: localhost:8081
Session ID: sess_xxxxxxxx
Connected at: 2026-xx-xx xx:xx:xx UTC

Sync Status:
Auto-sync: enabled
Conflict resolution: last_write_wins
Devices connected: 1 (CLI)

Real-time sync is now active.
```

---

### 5.2 Monitor real-time updates (sync monitor)

**Scenario: Terminal 2 monitors, Terminal 3 triggers**

**Terminal 2 - Start monitoring:**
```bash
.\mangahub.exe sync connect
.\mangahub.exe sync monitor
```
Expected output: `Monitoring real-time sync updates... (Press Ctrl+C to exit)`

**Terminal 3 - Trigger update as alice:**
```bash
.\mangahub.exe --session c1 auth login --username alice
.\mangahub.exe --session c1 progress update --manga-id one-piece --chapter 1060
```
**Expected output immediately at Terminal 2:**
```
[15:30:45] ← Device updated: one-piece → Chapter 1060
```

**Terminal 3 - Trigger another update:**
```bash
.\mangahub.exe --session c1 progress update --manga-id attack-on-titan --chapter 89
```
**Terminal 2 receives:**
```
[15:31:02] ← Device updated: attack-on-titan → Chapter 89
```

**Terminal 4 - Trigger from bob (different user):**
```bash
.\mangahub.exe --session c2 progress update --manga-id one-piece --chapter 350
```
**Terminal 2 receives (even though bob is a different user):**
```
[15:31:20] ← Device updated: one-piece → Chapter 350
```

---

### 5.3 Check connection status (sync status)

```bash
.\mangahub.exe sync status
```
Expected output:
```
TCP Sync Status:

Connection: ✓ Active
Server: localhost:8081
Uptime: 5m 30s

Sync Statistics:
Messages sent: 3
Messages received: 0
Last sync: one-piece ch. 1060
```

---

### 5.4 Disconnect (sync disconnect)

```bash
.\mangahub.exe sync disconnect
```
Expected output:
```
✓ Disconnected from TCP sync server.
Real-time sync is now paused.
```

**Check after disconnect:**
```bash
.\mangahub.exe sync status
```
Result: `Sync is disconnected. Run 'mangahub sync connect'.`

**Test disconnect a second time:**
```bash
.\mangahub.exe sync disconnect
```
Result: `Already disconnected from TCP sync server.`

---

### 5.5 Test TCP connection manually (PowerShell)

```powershell
# Connect TCP and listen for broadcasts
$tcp = New-Object System.Net.Sockets.TcpClient
$tcp.Connect("localhost", 8081)
$stream = $tcp.GetStream()
$writer = New-Object System.IO.StreamWriter($stream)
$reader = New-Object System.IO.StreamReader($stream)

# Register to receive broadcasts
$writer.WriteLine('{"user_id":"manual-test-client"}')
$writer.Flush()

Write-Host "Listening... (Ctrl+C to stop)"
while ($true) {
    if ($stream.DataAvailable) {
        $line = $reader.ReadLine()
        Write-Host "[$(Get-Date -Format 'HH:mm:ss')] $line"
    }
    Start-Sleep -Milliseconds 200
}
```
> After running this script, trigger a `progress update` from another terminal and observe the JSON broadcast received.

---

### TCP Sync Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Connect TCP | `sync connect` | ✓ Connected, Session ID shown |
| 2 | Monitor real-time | `sync monitor` | Listening for updates |
| 3 | Alice triggers → monitor receives | `--session c1 progress update` in T3 | T2 shows update instantly |
| 4 | Bob triggers → monitor receives | `--session c2 progress update` | T2 receives bob's update |
| 5 | Multiple updates | 3 different updates in sequence | Monitor receives all 3 |
| 6 | Sync status (connected) | `sync status` | Active, uptime, message count |
| 7 | Sync status (disconnected) | `sync status` before connecting | "Sync is disconnected" |
| 8 | Disconnect | `sync disconnect` | ✓ Disconnected |
| 9 | Disconnect again | `sync disconnect` again | "Already disconnected" |
| 10 | Manual TCP connection | PowerShell TcpClient | Receives JSON broadcast |

---

## PART 6: UDP NOTIFICATIONS

> UDP Notification System allows clients to **register for real-time notifications** when new chapters are released.
> Requires **at least 2 terminals**: T2 subscribes (keep open), T3 sends broadcast.
> `notify subscribe` and `notify unsubscribe` **require login**. Other notify commands do not.

---

### 6.1 Subscribe to notifications (notify subscribe)

**Preparation:**
```bash
# Terminal 2 - Login as alice (session c1)
.\mangahub.exe --session c1 auth login --username alice
# Enter password: Test1234
```

**Test Case 1: Successful subscription**
```bash
# Terminal 2 - KEEP THIS TERMINAL OPEN to receive notifications
.\mangahub.exe --session c1 notify subscribe
```
Expected output:
```
✓ Subscribed to chapter release notifications!

Notification Details:
  Server:       localhost:9091
  Subscribed:   2026-xx-xx xx:xx:xx UTC
  User ID:      <uuid>
  Successfully registered for notifications. Total subscribers: 1

You will receive UDP notifications when new chapters are released.
Run 'mangahub notify unsubscribe' to stop receiving notifications.
Run 'mangahub notify test' to trigger a test notification.

Listening for notifications... (Press Ctrl+C to exit)
```
> Terminal 2 will be **blocking/listening**. All subsequent steps run in Terminal 3.

**Test Case 2: Subscribe again - duplicate (Terminal 3)**
```bash
.\mangahub.exe --session c1 notify subscribe
```
Expected output:
```
Already subscribed to notifications.
Server: localhost:9091
Run 'mangahub notify unsubscribe' to stop.
```

**Test Case 3: Subscribe when not logged in**
```bash
.\mangahub.exe --session c1 auth logout
.\mangahub.exe --session c1 notify subscribe
```
Expected output:
```
Not logged in. Please login first.
```
```bash
# Login again to continue
.\mangahub.exe --session c1 auth login --username alice
```

**Test Case 4: Subscribe second user bob (Terminal 4)**
```bash
# Terminal 4 - bob subscribes, KEEP OPEN
.\mangahub.exe --session c2 notify subscribe
```
Expected output: `Total subscribers: 2`

---

### 6.2 View subscription status (notify preferences)

**Test Case 1: View preferences when subscribed**
```bash
# Terminal 3 (does not block)
.\mangahub.exe notify preferences
```
Expected output:
```
Notification Preferences:
────────────────────────────────────────
  Status:       ✓ Subscribed
  Server:       localhost:9091
  Subscribed:   2026-xx-xx xx:xx:xx UTC
  User ID:      <uuid>
────────────────────────────────────────

Available Commands:
  mangahub notify unsubscribe  - Stop receiving notifications
  mangahub notify test         - Send a test notification
```

**Test Case 2: View preferences when not subscribed (charlie - session c3)**
```bash
.\mangahub.exe --session c3 auth login --username charlie
.\mangahub.exe --session c3 notify preferences
```
Expected output:
```
Notification Preferences:
────────────────────────────────────────
  Status:       ✗ Not subscribed
────────────────────────────────────────

Available Commands:
  mangahub notify subscribe    - Start receiving notifications
```

---

### 6.3 Send test broadcast (notify test)

> `notify test` does not require login — it sends a UDP packet directly to the server.

**Test Case 1: Generic broadcast (no manga specified)**
```bash
# Terminal 3
.\mangahub.exe notify test
```
Expected output at **Terminal 3** (sender):
```
✓ Test broadcast sent for manga 'test-manga'

Notification Details:
  Type:    chapter_release
  Manga:   test-manga
  Message: [TEST] New chapter of 'test-manga' is now available!
  Server:  localhost:9091

All registered subscribers should receive this notification.
```
Expected output at **Terminal 2** (alice listening):
```
[15:30:45] 🔔 NOTIFICATION RECEIVED!
  Manga: test-manga
  Message: [TEST] New chapter of 'test-manga' is now available!
```
Expected output at **Terminal 4** (bob listening):
```
[15:30:45] 🔔 NOTIFICATION RECEIVED!
  Manga: test-manga
  Message: [TEST] New chapter of 'test-manga' is now available!
```

**Test Case 2: Broadcast for a specific manga**
```bash
.\mangahub.exe notify test --manga-id one-piece
```
Expected output at T3:
```
✓ Test broadcast sent for manga 'one-piece'
  Message: [TEST] New chapter of 'one-piece' is now available!
```
T2 and T4 both receive:
```
[15:31:00] 🔔 NOTIFICATION RECEIVED!
  Manga: one-piece
  Message: [TEST] New chapter of 'one-piece' is now available!
```

**Test Case 3: Multiple consecutive broadcasts**
```bash
.\mangahub.exe notify test --manga-id attack-on-titan
.\mangahub.exe notify test --manga-id death-note
.\mangahub.exe notify test --manga-id naruto
```
Expected output: T2 and T4 receive 3 notifications in sequence.

---

### 6.4 Unsubscribe (notify unsubscribe)

> Press **Ctrl+C** at Terminal 2 to exit listening mode first.

```bash
# Terminal 2
.\mangahub.exe --session c1 notify unsubscribe
```
Expected output:
```
✓ Unsubscribed from notifications.
You will no longer receive chapter release notifications.
Run 'mangahub notify subscribe' to subscribe again.
```

**Check preferences after unsubscribe:**
```bash
.\mangahub.exe notify preferences
```
Expected output: `Status: ✗ Not subscribed`

**Test Case 2: Unsubscribe when not subscribed**
```bash
.\mangahub.exe --session c1 notify unsubscribe
```
Expected output:
```
Not currently subscribed to notifications.
```

---

### 6.5 Test UDP connection manually (PowerShell)

```powershell
# Create UDP client and connect to server
$udpClient = New-Object System.Net.Sockets.UdpClient
$udpClient.Connect("localhost", 9091)
$ep = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
$udpClient.Client.ReceiveTimeout = 3000

# === Test 1: Register ===
$bytes = [System.Text.Encoding]::UTF8.GetBytes('{"action":"register","user_id":"ps-test-user"}')
$udpClient.Send($bytes, $bytes.Length)
$data = $udpClient.Receive([ref]$ep)
Write-Host "Register: $([System.Text.Encoding]::UTF8.GetString($data))"
```
Expected output:
```json
{"status":"registered","message":"Successfully registered for notifications. Total subscribers: N"}
```

```powershell
# === Test 2: Send broadcast ===
$bytes2 = [System.Text.Encoding]::UTF8.GetBytes('{"action":"broadcast","manga_id":"one-piece","message":"Chapter 1101 released!","type":"chapter_release"}')
$udpClient.Send($bytes2, $bytes2.Length)
$data2 = $udpClient.Receive([ref]$ep)
Write-Host "Broadcast: $([System.Text.Encoding]::UTF8.GetString($data2))"
```
Expected output:
```json
{"status":"broadcast_triggered","message":"Broadcast initiated for manga 'one-piece'"}
```

```powershell
# === Test 3: Ping server ===
$bytes3 = [System.Text.Encoding]::UTF8.GetBytes('{"action":"ping","user_id":"ps-test-user"}')
$udpClient.Send($bytes3, $bytes3.Length)
$data3 = $udpClient.Receive([ref]$ep)
Write-Host "Ping: $([System.Text.Encoding]::UTF8.GetString($data3))"
```
Expected output:
```json
{"status":"pong","message":"Server is alive"}
```

```powershell
# === Test 4: Unregister ===
$bytes4 = [System.Text.Encoding]::UTF8.GetBytes('{"action":"unregister","user_id":"ps-test-user"}')
$udpClient.Send($bytes4, $bytes4.Length)
$data4 = $udpClient.Receive([ref]$ep)
Write-Host "Unregister: $([System.Text.Encoding]::UTF8.GetString($data4))"
$udpClient.Close()
```
Expected output:
```json
{"status":"unregistered","message":"Successfully unregistered"}
```

---

### UDP Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Subscribe successfully | `--session c1 notify subscribe` (logged in) | ✓ Subscribed, listening |
| 2 | Subscribe duplicate | `--session c1 notify subscribe` again | "Already subscribed" |
| 3 | Subscribe not logged in | `--session c1 notify subscribe` after logout | "Not logged in" |
| 4 | Two users subscribe | c1 alice (T2) + c2 bob (T4) subscribe | Total subscribers: 2 |
| 5 | Preferences (subscribed) | `notify preferences` | ✓ Subscribed + full details |
| 6 | Preferences (not subscribed) | `--session c3 notify preferences` | ✗ Not subscribed |
| 7 | Generic broadcast | `notify test` | Both T2 alice and T4 bob receive |
| 8 | Broadcast specific manga | `notify test --manga-id one-piece` | one-piece notification |
| 9 | 3 consecutive broadcasts | 3 × `notify test --manga-id ...` | Receive all 3 |
| 10 | Unsubscribe | `--session c1 notify unsubscribe` | ✓ Unsubscribed |
| 11 | Preferences after unsubscribe | `notify preferences` | ✗ Not subscribed |
| 12 | Unsubscribe when not subscribed | `--session c1 notify unsubscribe` again | "Not currently subscribed" |
| 13 | UDP register manually | PowerShell `action:register` | JSON status=registered |
| 14 | UDP broadcast manually | PowerShell `action:broadcast` | JSON status=broadcast_triggered |
| 15 | UDP ping | PowerShell `action:ping` | JSON status=pong |
| 16 | UDP unregister manually | PowerShell `action:unregister` | JSON status=unregistered |

---

## PART 7: gRPC SERVICES

> gRPC provides high-performance internal communication on port **9092**.
> `grpc manga get` and `grpc manga search` **do not require login**.
> `grpc progress update` **requires login** (`--session`).

---

### 7.1 Query manga via gRPC (grpc manga get)

**Test Case 1: Get manga by ID**
```bash
.\mangahub.exe grpc manga get --id "one-piece"
```
Expected output:
```
✓ [gRPC] Found: One Piece - Oda Eiichiro
```

**Test Case 2: Other manga IDs**
```bash
.\mangahub.exe grpc manga get --id "attack-on-titan"
.\mangahub.exe grpc manga get --id "death-note"
.\mangahub.exe grpc manga get --id "naruto"
```

**Test Case 3: Non-existent manga**
```bash
.\mangahub.exe grpc manga get --id "fake-manga-xyz"
```
Expected output:
```
✗ gRPC Error: rpc error: code = NotFound ...
```

---

### 7.2 Search manga via gRPC (grpc manga search)

**Test Case 1: Simple search**
```bash
.\mangahub.exe grpc manga search --query "piece"
```
Expected output:
```
✓ [gRPC] Search Results for 'piece':
  - one-piece: One Piece (Oda Eiichiro)
```

**Test Case 2: Short keyword (multiple results)**
```bash
.\mangahub.exe grpc manga search --query "at"
```

**Test Case 3: No results found**
```bash
.\mangahub.exe grpc manga search --query "xyznonexistent"
```
Expected output:
```
No manga found matching your search criteria via gRPC.
```

---

### 7.3 Update progress via gRPC (grpc progress update)

> Requires login and manga must be in library.

**Preparation:**
```bash
.\mangahub.exe --session c1 auth login --username alice
.\mangahub.exe --session c1 library add --manga-id one-piece --status reading
```

**Test Case 1: Successful progress update**
```bash
.\mangahub.exe --session c1 grpc progress update --manga-id "one-piece" --chapter 505
```
Expected output:
```
✓ [gRPC] Updated successfully via gRPC
```

**Test Case 2: Update when not logged in**
```bash
.\mangahub.exe --session c1 auth logout
.\mangahub.exe --session c1 grpc progress update --manga-id "one-piece" --chapter 510
```
Expected output:
```
Not logged in.
```

**Test Case 3: gRPC update also triggers TCP broadcast**
> Open Terminal 2 with `sync monitor` first:
```bash
# Terminal 2
.\mangahub.exe sync connect
.\mangahub.exe sync monitor
```
```bash
# Terminal 3 - gRPC update (alice)
.\mangahub.exe --session c1 auth login --username alice
.\mangahub.exe --session c1 grpc progress update --manga-id "one-piece" --chapter 510
```
Expected output at **Terminal 2**:
```
[15:45:00] ← Device updated: one-piece → Chapter 510
```
> Proves gRPC update **also triggers TCP broadcast**, same as HTTP update.

**Test Case 4: Bob updates via gRPC (session c2)**
```bash
.\mangahub.exe --session c2 grpc progress update --manga-id "one-piece" --chapter 200
```
Expected output: `✓ [gRPC] Updated successfully via gRPC`

---

### gRPC Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Get manga by ID | `grpc manga get --id one-piece` | ✓ [gRPC] Found: One Piece |
| 2 | Get other manga | `grpc manga get --id death-note` | ✓ [gRPC] Found: Death Note |
| 3 | Get non-existent manga | `grpc manga get --id fake-id` | ✗ gRPC Error: NotFound |
| 4 | Search manga | `grpc manga search --query "piece"` | ✓ [gRPC] Search Results |
| 5 | Search multiple results | `grpc manga search --query "at"` | Multiple results |
| 6 | Search no results | `grpc manga search --query "xyzxyz"` | No manga found via gRPC |
| 7 | Update progress (logged in) | `--session c1 grpc progress update --manga-id one-piece --chapter 505` | ✓ Updated via gRPC |
| 8 | Update not logged in | `--session c1 grpc progress update` after logout | "Not logged in" |
| 9 | gRPC triggers TCP broadcast | gRPC update → sync monitor receives | Monitor shows update instantly |
| 10 | Multi-user gRPC | `--session c2 grpc progress update` | ✓ Bob updates independently |

---

*Continue: Part 8 - WebSocket Chat*

## PART 8: WEBSOCKET CHAT

> WebSocket Chat enables **real-time messaging** on port **9093**.
> `chat join` and `chat send` **require login** � always use `--session`.
> Use different `--session` values to run multiple users simultaneously in separate terminals.

---

### 8.1 Join General Chat (chat join)

**Preparation (Terminal 2 - alice, session c1):**
```bash
.\mangahub.exe --session c1 auth login --username alice 
# Test1234
.\mangahub.exe --session c1 chat join
```
Expected output:
```
Connecting to WebSocket chat server at ws://localhost:9093...
? Connected to General Chat

Chat Room: #general
Connected users: 1
Your status: Online

Recent messages:
(empty if no messages yet)

------------------------------------------------------------
You are now in chat. Type your message and press Enter.
Type /help for commands or /quit to leave.

alice>
```

**Preparation (Terminal 3 - bob, session c2):**
```bash
.\mangahub.exe --session c2 chat join
```
Expected output at **Terminal 2** (alice receives bob's join notification):
```
[15:30:00] *** bob joined the chat ***
```

---

### 8.2 Send and receive public messages

**Test Case 1: Alice sends a message**
```
# Terminal 2 (alice is in chat)
alice> Hello everyone!
```
Expected output at **Terminal 3** (bob receives instantly):
```
[15:30:15] alice: Hello everyone!
```

**Test Case 2: Bob replies**
```
# Terminal 3
bob> Hi alice, nice to meet you!
```
Expected output at **Terminal 2**:
```
[15:30:25] bob: Hi alice, nice to meet you!
```

---

### 8.3 In-chat commands (/command)

**Test Case 1: /help - View command list**
```
alice> /help
```
Expected output:
```
Chat Commands:
/help           - Show this help
/users          - List online users
/quit           - Leave chat
/pm <user> <msg>- Private message
/manga <id>     - Switch to manga chat room
/history        - Show recent message history
/status         - Connection status
```

**Test Case 2: /users - View online users**
```
alice> /users
```
Expected output:
```
Online Users (2):
? alice (General Chat)
? bob (General Chat)
```

**Test Case 3: /status - View connection status**
```
alice> /status
```
Expected output:
```
Connection Status: Connected
Room: #general
Server: ws://localhost:9093/ws
```

**Test Case 4: /history - View message history**
```
alice> /history
```
Expected output: List of recently sent messages in the current room.

---

### 8.4 Private messages (/pm)

**Test Case 1: Alice sends PM to bob**
```
alice> /pm bob Hey bob, this is private!
```
Expected output at **Terminal 2** (alice):
```
[15:31:00] [Private to bob]: Hey bob, this is private!
```
Expected output at **Terminal 3** (bob):
```
[15:31:00] [Private from alice]: Hey bob, this is private!
```
> Other users in the same room **cannot see** this message.

**Test Case 2: Bob replies privately to alice**
```
bob> /pm alice Got it, thanks!
```

**Test Case 3: PM to non-existent user**
```
alice> /pm unknownuser Hello?
```

---

### 8.5 Switch chat room (/manga)

**Test Case 1: Alice switches to one-piece room**
```
alice> /manga one-piece
```
Expected output:
```
? Connected to One Piece Discussion

Chat Room: one-piece
Connected users: 1
Your status: Online
```

**Test Case 2: Bob stays in #general, alice in one-piece � messages do not cross**
```
# Terminal 2 (alice in one-piece room)
alice> This is One Piece room only!

# Terminal 3 (bob in #general)
bob> This is general only!
```
Expected output: Alice **cannot see** bob's message and vice versa.
> Proves **room isolation** works correctly.

**Test Case 3: Bob also joins the one-piece room**
```
bob> /quit
```
```bash
# Terminal 3
.\mangahub.exe --session c2 chat join --manga-id one-piece
```
Expected output: Alice receives *** bob joined the chat ***, both users can now chat together.

---

### 8.6 Join manga room directly with --manga-id

```bash
.\mangahub.exe --session c1 chat join --manga-id naruto
.\mangahub.exe --session c2 chat join --manga-id naruto
```
Expected output: Both users enter the naruto room and can chat with each other.

---

### 8.7 Leave chat (/quit)

```
alice> /quit
```
Expected output at **Terminal 2**:
```
Leaving chat...
? Disconnected from chat server
```
Expected output at **Terminal 3** (bob receives notification):
```
[15:35:00] *** alice left the chat ***
```

---

### 8.8 View chat history (chat history)

```bash
.\mangahub.exe --session c1 chat history
.\mangahub.exe --session c1 chat history --manga-id one-piece
.\mangahub.exe --session c1 chat history --manga-id one-piece --limit 10
```

---

### 8.9 Send message without joining (chat send)

```bash
.\mangahub.exe --session c1 chat send "Hello from CLI!"
.\mangahub.exe --session c1 chat send "Great chapter!" --manga-id one-piece
```
Expected output:
```
? Message sent successfully
```

---

### 8.10 Test Chat API directly (PowerShell)

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/chat/users" | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/chat/history?room=%23general" | ConvertTo-Json -Depth 3
Invoke-RestMethod -Uri "http://localhost:8080/chat/history?room=one-piece" | ConvertTo-Json -Depth 3
$msg = '{"username":"apiuser","room":"#general","message":"Hello from API!"}'
Invoke-RestMethod -Uri "http://localhost:8080/chat/send" -Method POST -Body $msg -ContentType "application/json" | ConvertTo-Json
```

---

### WebSocket Chat Checklist

| # | Test Case | Command | Expected Output |
|---|-----------|---------|-----------------|
| 1 | Join General Chat | --session c1 chat join | Connected to General Chat |
| 2 | User 2 joins, user 1 notified | --session c2 chat join | c1 sees *** bob joined *** |
| 3 | Send public message | Alice types, Bob receives | Message appears instantly |
| 4 | /help | /help in chat | Command list shown |
| 5 | /users | /users in chat | Online user list |
| 6 | /status | /status in chat | Connection info |
| 7 | /history | /history in chat | Recent messages |
| 8 | Private message | /pm bob Hello! | Only bob sees it |
| 9 | Room isolation | c1 in one-piece, c2 in #general | Cannot see each other |
| 10 | Switch room | /manga one-piece | Exits #general, enters one-piece |
| 11 | Join manga room directly | --session c1 chat join --manga-id naruto | Connected to Naruto Discussion |
| 12 | /quit | /quit in chat | Disconnected; other user sees left |
| 13 | Chat history | --session c1 chat history | Messages in room |
| 14 | Chat history manga | --session c1 chat history --manga-id one-piece --limit 10 | Max 10 messages |
| 15 | Chat send | --session c1 chat send "Hello!" | Message sent |
| 16 | Chat users API | GET /chat/users | JSON user list |

---

## SUMMARY

| Part | Feature | Test Cases |
|------|---------|------------|
| 1 | Authentication (register, login, logout, change-password) | 13 |
| 2 | Manga (search, info, list, CRUD) | 14 |
| 3 | Library (add, list, update, remove) | 12 |
| 4 | Progress Tracking (update, history, sync) | 13 |
| 5 | TCP Sync (connect, monitor, status, disconnect) | 10 |
| 6 | UDP Notifications (subscribe, broadcast, unsubscribe) | 16 |
| 7 | gRPC Services (get, search, update progress) | 10 |
| 8 | WebSocket Chat (join, PM, rooms, commands) | 16 |
| **Total** | | **104 test cases** |

> **Session Reference:**
> - --session c1 = alice (all commands requiring login)
> - --session c2 = bob (all commands requiring login)
> - --session c3 = charlie (all commands requiring login)
> - No session = public commands: manga search/info/list, server start/status, sync connect/monitor/status/disconnect, notify test/preferences, grpc manga get/search
