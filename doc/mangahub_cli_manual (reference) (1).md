# MangaHub CLI Application - User Manual
## Table of Contents

1. Introduction


2. Installation and Setup


3. Getting Started


4. Authentication Commands


5. Manga Management


6. Library Operations


7. Network Protocol Features


8. Chat System


9. Configuration


10. Troubleshooting


11. Advanced Features

## Introduction


MangaHub CLI is a command-line interface for the MangaHub manga tracking system. It
provides access to all core features including manga discovery, reading progress tracking,
real-time synchronization, and community chat functionality.

### System Requirements

  - Go 1.19 or later


  - SQLite 3.x


  - Network connectivity for synchronization features


  - Terminal with UTF-8 support

### Supported Platforms

  - Linux (x64, ARM)


  - macOS (Intel, Apple Silicon)


  - Windows (x64)


## Installation and Setup
### Download and Install

```
# Download the latest release
wget https://github.com/yourorg/mangahub/releases/latest/mangahub-cli

# Make executable (Linux/macOS)
chmod +x mangahub-cli

# Move to system path
sudo mv mangahub-cli /usr/local/bin/mangahub

# Verify installation
mangahub version

### First-Time Setup

# Initialize configuration
mangahub init

# This creates:
# ~/.mangahub/config.yaml
# ~/.mangahub/data.db
# ~/.mangahub/logs/
```
### Configuration File



The default configuration is created at `~/.mangahub/config.yaml` :

```
server :
 host : "localhost"
 http_port : 8080
 tcp_port : 9090
 udp_port : 9091
 grpc_port : 9092
 websocket_port : 9093

database :
 path : "~/.mangahub/data.db"

user :
 username : ""
 token : ""

sync :
 auto_sync : true
 conflict_resolution : "last_write_wins"

notifications :
 enabled : true
 sound : false

logging :
 level : "info"
 path : "~/.mangahub/logs/"
```

## Getting Started
### Quick Start Guide

# 1. Start the MangaHub server
mangahub server start

# 2. In another terminal, register a new account
mangahub auth register --username myuser --email myuser@example.com

# 3. Login to get authentication token
mangahub auth login --username myuser

# 4. Search for manga
mangahub manga search "one piece"

# 5. Add manga to library
mangahub library add --manga-id one-piece --status reading

# 6. Update reading progress
mangahub progress update --manga-id one-piece --chapter 1095

### Command Structure

All commands follow the pattern:

```
mangahub <command> <subcommand> [flags] [arguments]
```
### Global Flags


  - `--config` : Specify config file path


  - `--verbose` : Enable verbose output


  - `--quiet` : Suppress non-error output


  - `--help` : Show help information

## Authentication Commands
### Register New Account

```
mangahub auth register --username <username> --email <email>
# Prompts for password securely

```

```
# Example
mangahub auth register --username johndoe --email john@example.com

```

**Expected Output:**

```
Password: [hidden input]
Confirm password: [hidden input]

✓ `Account created successfully!`
User ID: usr_1a2b3c4d5e
Username: johndoe
Email: john@example.com
Created: 2024-01-20 10:30:00 UTC

Please login to start using MangaHub:
mangahub auth login --username johndoe

```

**Error Cases:**

```
✗ `Registration failed: Username 'johndoe' already exists`
Try: mangahub auth login --username johndoe

```
```
✗ `Registration failed: Invalid email format`
Please provide a valid email address

```
```
✗ `Registration failed: Password too weak`
Password must be at least 8 characters with mixed case and numbers
```

### Login

```
mangahub auth login --username <username>
# Prompts for password
```
```
# Alternative: login with email
mangahub auth login --email <email>
```
```
# Example
mangahub auth login --username johndoe
```


**Expected Output:**

```
Password: [hidden input]

✓ `Login successful!`
Welcome back, johndoe!

Session Details:
Token expires: 2024-01-21 10:30:00 UTC (24 hours)
Permissions: read, write, sync

Auto-sync: enabled
Notifications: enabled

Ready to use MangaHub! Try:
mangahub manga search "your favorite manga"

```

**Error Cases:**

```
✗ `Login failed: Invalid credentials`
Check your username and password

```
```
✗ `Login failed: Account not found`
Try: mangahub auth register --username johndoe --email john@example.com

```
```
✗ `Login failed: Server connection error`
Check server status: mangahub server status
```

### Logout

```
mangahub auth logout
# Removes stored authentication token
```
### Check Authentication Status
```
mangahub auth status
# Shows current login status and user information
```
### Change Password
```
mangahub auth change-password
# Prompts for current password and new password
```
## Manga Management
### Search Manga
```
# Basic search
mangahub manga search <query>
```
```
# Search with filters
mangahub manga search <query> --genre <genre> --status <status>
```
```
# Examples
mangahub manga search "attack on titan"
mangahub manga search "romance" --genre romance --status completed
mangahub manga search "naruto" --limit 5
```

**Expected Output:**


```
Searching for "attack on titan"...

Found 3 results:

┌─────────────────────┬──────────────────────┬───────────┬──────────┬────────
─────┐
│ ID                  │ Title                │ Author     │ Status  │
Chapters  │
├─────────────────────┼──────────────────────┼───────────┼──────────┼────────
─────┤
│ attack-on-titan     │ Attack on Titan      │ Isayama  │ Completed│ 139
│
│                     │ (Shingeki no Kyojin) │ Hajime    │          │
│
├─────────────────────┼──────────────────────┼───────────┼──────────┼────────
─────┤
│ attack-on-titan-jr  │ Attack on Titan:     │ Isayama   │ Completed│ 7
│
│                     │ Junior High          │ Hajime    │          │
│
├─────────────────────┼──────────────────────┼───────────┼──────────┼────────
─────┤
│ aot-before-fall     │ Attack on Titan:     │ Suzukaze  │ Completed│ 17
│
│                     │ Before the Fall      │ Ryo       │          │
│
└─────────────────────┴──────────────────────┴───────────┴──────────┴────────
─────┘

Use 'mangahub manga info <id>' to view details
Use 'mangahub library add --manga-id <id>' to add to your library

```

**No Results Output:**

```
Searching for "nonexistent manga"...

No manga found matching your search criteria.

Suggestions:
- Check spelling and try again

- Use broader search terms

- Browse by genre: mangahub manga list --genre action
```

### View Manga Details
```
mangahub manga info <manga-id>

# Example
mangahub manga info one-piece

```

**Expected Output:**

```
┌─────────────────────────────────────────────────────────────────────┐
│              ONE PIECE                                              │
└─────────────────────────────────────────────────────────────────────┘

Basic Information:
ID: one-piece
Title: One Piece ( ワンピース )
Author: Oda Eiichiro
Artist: Oda Eiichiro
Genres: Action, Adventure, Comedy, Drama, Shounen
Status: Ongoing
Year: 1997

Progress:
Total Chapters: 1,100+
Total Volumes: 107+
Serialization: Weekly Shounen Jump
Publisher: Shueisha

Your Status: Currently Reading
Current Chapter: 1,095
Last Updated: 2024-01-20 15:30:00
Started Reading: 2023-03-15
Personal Rating: 9/10

Description:
Monkey D. Luffy, a boy whose body gained the properties of rubber after
eating a Devil Fruit, explores the Grand Line with his diverse crew of
pirates in search of the treasure known as "One Piece" to become the
next Pirate King.

External Links:
MyAnimeList: https://myanimelist.net/manga/13
MangaDx: https://mangadx.org/title/a1c7c817-4e59-43b7-9365-09675a149a6f

Actions:
Update Progress: mangahub progress update --manga-id one-piece --chapter
1096
Rate/Review: mangahub library update --manga-id one-piece --rating 10
Remove: mangahub library remove --manga-id one-piece

```

**Not Found Output:**

```
✗ `Manga not found: 'nonexistent-id'`


Try searching instead:
mangahub manga search "manga title"
```
### List All Manga
```
# List all manga in database
mangahub manga list

# List with pagination
mangahub manga list --page 2 --limit 20

# Filter by genre
mangahub manga list --genre shounen
```
### Advanced Search Options
```
mangahub manga search "keyword" \
 --genre "action,adventure" \
 --status "ongoing" \
 --author "author name" \
 --year-from 2020 \
 --year-to 2024 \
 --min-chapters 50 \
 --sort-by "popularity" \
 --order "desc"
```
## Library Operations
### Add Manga to Library
```
mangahub library add --manga-id <id> --status <status>

# Status options: reading, completed, plan-to-read, on-hold, dropped
# Examples
mangahub library add --manga-id one-piece --status reading
mangahub library add --manga-id death-note --status completed --rating 9
```

### View Library
```
# View entire library
mangahub library list

# Filter by status
mangahub library list --status reading
mangahub library list --status completed

# Sort options
mangahub library list --sort-by title
mangahub library list --sort-by last-updated --order desc

```

**Expected Output:**

```
Your Manga Library (47 entries)

Currently Reading (8):
┌──────────────────┬────────────────────────┬─────────┬────────────┬─────────
─┬──────────┐
│ ID               │ Title                  │ Chapter │ Rating      │ Started
│ Updated │
├──────────────────┼────────────────────────┼─────────┼────────────┼─────────
─┼──────────┤
│ one-piece        │ One Piece              │ 1095/?? │ 9/10       │ 2023-03
│ Just now         │
│ jujutsu-kaisen   │ Jujutsu Kaisen        │ 247/??  │ 8/10       │ 2023-10
│ 2 days  │
│ attack-on-titan │ Attack on Titan    │ 89/139 │ Unrated  │ 2024-01
│ 1 week  │
│ demon-slayer   │ Demon Slayer     │ 156/205 │ 7/10    │ 2023-12
│ 3 days  │
└──────────────────┴────────────────────────┴─────────┴────────────┴─────────
─┴──────────┘

Completed (15):
┌──────────────────┬────────────────────────┬─────────┬────────────┬─────────
─────────────┐
│ ID        │ Title         │ Chapters│ Rating   │
Completed      │
├──────────────────┼────────────────────────┼─────────┼────────────┼─────────
─────────────┤
│ death-note    │ Death Note       │ 108/108 │ 10/10   │ 2023-0815      │
│ fullmetal    │ Fullmetal Alchemist  │ 108/108 │ 9/10    │ 2023-0922      │
│ naruto      │ Naruto         │ 700/700 │ 8/10    │ 2023-1130      │
└──────────────────┴────────────────────────┴─────────┴────────────┴─────────
─────────────┘

Plan to Read (18), On Hold (4), Dropped (2)

Use --status <status> to filter by specific status
Use --verbose for detailed view with descriptions

```

**Empty Library Output:**

```
Your library is empty.

Get started by searching and adding manga:
mangahub manga search "your favorite series"
mangahub library add --manga-id <id> --status reading

```

### Remove from Library

```
mangahub library remove --manga-id <id>

# Example
mangahub library remove --manga-id completed-series
```
### Update Library Entry
```
mangahub library update --manga-id <id> --status <new-status>

# Example
mangahub library update --manga-id one-piece --status completed --rating 10
```
## Progress Tracking
### Update Reading Progress
```
mangahub progress update --manga-id <id> --chapter <number>

# With additional info
mangahub progress update --manga-id <id> --chapter <number> --volume <number>

# Examples
mangahub progress update --manga-id one-piece --chapter 1095
mangahub progress update --manga-id naruto --chapter 700 --volume 72 --notes
"Great ending!"

```

**Expected Output:**

```
Updating reading progress...

✓ `Progress updated successfully!`


Manga: One Piece
Previous: Chapter 1,094
Current: Chapter 1,095 (+1)
Updated: 2024-01-20 16:45:00 UTC

Sync Status:


`Local database:` ✓ `Updated`
`TCP sync server:` ✓ `Broadcasting to 3 connected devices`
`Cloud backup:` ✓ `Synced`


Statistics:
Total chapters read: 1,095
Reading streak: 45 days
Estimated completion: Never (ongoing series)

Next actions:
Continue reading: Chapter 1,096 available
Rate this chapter: mangahub library update --manga-id one-piece --rating 9

```

**Error Cases:**

```
✗ `Progress update failed: Chapter 2000 exceeds manga's total chapters (1100)`
Valid range: 1-1100

```
```
✗ `Progress update failed: Chapter 50 is behind your current progress`

(Chapter 95)
Use --force to set backwards progress: --force --chapter 50

```
```
✗ `Progress update failed: Manga 'invalid-id' not found in your library`
Add to library first: mangahub library add --manga-id invalid-id --status
reading
```

### View Progress History
```
mangahub progress history --manga-id <id>

# View all progress updates
mangahub progress history
```
### Sync Progress
```
# Manual sync with server
mangahub progress sync

# Check sync status
mangahub progress sync-status
```
## Network Protocol Features
### TCP Progress Synchronization
```
# Connect to TCP sync server
mangahub sync connect

# Disconnect from sync server
mangahub sync disconnect

# Check connection status
mangahub sync status

# View real-time progress updates
mangahub sync monitor

```

**Expected Output for** **`mangahub sync connect`** **:**

```
Connecting to TCP sync server at localhost:9090...

✓ `Connected successfully!`

Connection Details:
Server: localhost:9090
User: johndoe (usr_1a2b3c4d5e)
Session ID: sess_9x8y7z6w5v
Connected at: 2024-01-20 17:00:00 UTC

Sync Status:
Auto-sync: enabled
Conflict resolution: last_write_wins
Devices connected: 3 (mobile, desktop, web)

Real-time sync is now active. Your progress will be synchronized across all
devices.

```

**Expected Output for** **`mangahub sync status`** **:**

```
TCP Sync Status:

`Connection:` ✓ `Active`
Server: localhost:9090
Uptime: 2h 15m 30s
Last heartbeat: 2 seconds ago

Session Info:
User: johndoe
Session ID: sess_9x8y7z6w5v
Devices online: 3

Sync Statistics:
Messages sent: 47
Messages received: 23
Last sync: 30 seconds ago (One Piece ch. 1095)
Sync conflicts: 0

Network Quality: Excellent (RTT: 15ms)

```

**Expected Output for** **`mangahub sync monitor`** **:**

```
Monitoring real-time sync updates... (Press Ctrl+C to exit)

[17:05:12] ← Device 'mobile' updated: Jujutsu Kaisen → Chapter 248
[17:05:45] → Broadcasting update: Attack on Titan → Chapter 90
[17:06:23] ← Device 'web' updated: Demon Slayer → Chapter 157
[17:07:01] ← Device 'mobile' updated: One Piece → Chapter 1096
[17:07:35] → Broadcasting update: One Piece → Chapter 1096 (sync conflict
resolved)

```


Real-time sync monitoring active. Updates appear as they happen.

### UDP Notifications
```
# Subscribe to chapter release notifications
mangahub notify subscribe

# Unsubscribe from notifications
mangahub notify unsubscribe

# View notification preferences
mangahub notify preferences

# Test notification system
mangahub notify test
```
### gRPC Service Operations
```
# Query manga via gRPC
mangahub grpc manga get --id <manga-id>

# Search via gRPC
mangahub grpc manga search --query <search-term>

# Update progress via gRPC
mangahub grpc progress update --manga-id <id> --chapter <number>
```
## Chat System
### Connect to Chat
```
# Join general chat
mangahub chat join

# Join specific manga discussion
mangahub chat join --manga-id <id>

# Example
mangahub chat join --manga-id one-piece

```

**Expected Output for** **`mangahub chat join`** **:**

```
Connecting to WebSocket chat server at ws://localhost:9093...

```

✓ `Connected to General Chat`

```
Chat Room: #general
Connected users: 12
Your status: Online

```

```
Recent messages:
[16:45] alice: Just finished reading the latest chapter!
[16:47] bob: Which manga are you reading?
[16:48] alice: Attack on Titan, it's getting intense
[16:50] charlie: No spoilers please!

─────────────────────────────────────────────────────────────
You are now in chat. Type your message and press Enter.
Type /help for commands or /quit to leave.

johndoe>

```

**Expected Output for Chat Commands:**

```
johndoe> /help

Chat Commands:
/help      - Show this help
/users     - List online users
/quit      - Leave chat
/pm <user> <msg>- Private message
/manga <id>   - Switch to manga chat
/history    - Show recent history
/status     - Connection status

johndoe> /users

Online Users (12):
● alice (General Chat)
● bob (General Chat)
● charlie (General Chat)
● diana (One Piece Discussion)
● elena (Attack on Titan Discussion)
● frank (General Chat)
[... 6 more users]

johndoe> Hello everyone!
[17:02] johndoe: Hello everyone!
[17:02] alice: Hey johndoe! Welcome to the chat
[17:03] bob: Hi there! What are you reading these days?

johndoe> /quit
Leaving chat...

✓ `Disconnected from chat server`
```

### Send Messages

```
# Send message to current chat
mangahub chat send "Hello everyone!"

# Send message to specific manga chat
mangahub chat send "Great chapter!" --manga-id one-piece
```
### Chat Commands (Interactive Mode)


When in chat mode, use these commands: - `/help` - Show chat commands


  - `/users`  - List online users


  - `/quit`  - Leave chat


  - `/pm <username> <message>`  - Private message


  - `/manga <id>`  - Switch to manga-specific chat

### View Chat History

```
# View recent messages
mangahub chat history

# View messages for specific manga
mangahub chat history --manga-id one-piece --limit 50
```
## Statistics and Analytics
### Reading Statistics
```
# View personal reading statistics
mangahub stats overview

# Detailed breakdown
mangahub stats detailed

# Stats for specific time period
mangahub stats --from 2024-01-01 --to 2024-12-31
```
### Export Data
```
# Export library to JSON
mangahub export library --format json --output library.json

# Export reading progress
mangahub export progress --format csv --output progress.csv

# Full data export
mangahub export all --output mangahub-backup.tar.gz

```

## Server Management
### Start Server Components

```
# Start all servers
mangahub server start

# Start specific servers
mangahub server start --http-only
mangahub server start --tcp-only
mangahub server start --udp-only

```

**Expected Output for** **`mangahub server start`** **:**

```
Starting MangaHub Server Components...

[1/5] HTTP API Server
✓ `Starting on http://localhost:8080`
✓ `Database connection established`
✓ `JWT middleware loaded`
✓ `12 routes registered`
Status: Running

[2/5] TCP Sync Server
✓ `Starting on tcp://localhost:9090`
✓ `Connection pool initialized (max: 100)`
✓ `Broadcast channels ready`
Status: Listening for connections

[3/5] UDP Notification Server
✓ `Starting on udp://localhost:9091`
✓ `Client registry initialized`
✓ `Notification queue ready`
Status: Ready for broadcasts

[4/5] gRPC Internal Service
✓ `Starting on grpc://localhost:9092`
✓ `3 services registered`
✓ `Protocol buffers loaded`
Status: Serving

[5/5] WebSocket Chat Server
✓ `Starting on ws://localhost:9093`
✓ `Chat rooms initialized`
✓ `User registry ready`

Status: Ready for connections

All servers started successfully!

Server URLs:
  HTTP API:  http://localhost:8080
  TCP Sync:  tcp://localhost:9090
  UDP Notify: udp://localhost:9091
  gRPC:    grpc://localhost:9092
  WebSocket:  ws://localhost:9093

Logs: tail -f ~/.mangahub/logs/server.log
Stop: mangahub server stop
```
### Server Status
```
# Check server status
mangahub server status

# Detailed health check
mangahub server health

```

**Expected Output for** **`mangahub server status`** **:**

```
MangaHub Server Status

┌─────────────────────┬──────────┬─────────────────────┬────────────┬────────
────┐
│ Service             │ Status   │ Address             │ Uptime     │ Load
│
├─────────────────────┼──────────┼─────────────────────┼────────────┼────────
────┤
`│ HTTP API           │✓  Online │ localhost:8080      │ 2h 15m     │ 12`
req/min │
`│ TCP Sync           │✓  Online │ localhost:9090      │ 2h 15m     │ 3`
clients │
`│ UDP Notifications  │✓  Online │ localhost:9091      │ 2h 15m     │ 8`
clients │
`│ gRPC Internal      │✓  Online │ localhost:9092      │ 2h 15m     │ 5`
req/min │
`│ WebSocket Chat     │✓  Online │ localhost:9093      │ 2h 15m     │ 12`
users  │
└─────────────────────┴──────────┴─────────────────────┴────────────┴────────
────┘

`Overall System Health:` ✓ `Healthy`

Database:

  `Connection:` ✓ `Active`
  Size: 2.1 MB
  Tables: 3 (users, manga, user_progress)

  Last backup: 2024-01-20 12:00:00

Memory Usage: 45.2 MB / 512 MB (8.8%)
CPU Usage: 2.3% average
Disk Space: 892 MB / 10 GB available

```
**Error Status Output:**
```
MangaHub Server Status

┌─────────────────────┬──────────┬─────────────────────┬────────────┬────────
────┐
│ Service             │ Status   │ Address             │ Uptime     │ Load
│
├─────────────────────┼──────────┼─────────────────────┼────────────┼────────
────┤
`│ HTTP API           │✓  Online │ localhost:8080      │ 45m        │ 8`
req/min │
`│ TCP Sync           │✗ Error   │ localhost:9090      │ -          │ -`
│
`│ UDP Notifications  │ ⚠ Warn   │ localhost:9091      │ 45m        │ 0`
clients │
`│ gRPC Internal      │✓  Online │ localhost:9092      │ 45m        │ 2`
req/min │
`│ WebSocket Chat     │✓  Online │ localhost:9093      │ 45m        │ 5 users`
│
└─────────────────────┴──────────┴─────────────────────┴────────────┴────────
────┘
`Overall System Health:` ⚠ `Degraded`

Issues Detected:

✗ `TCP Sync Server: Port 9090 already in use`
Solution: Kill process on port 9090 or change port in config

⚠ `UDP Notifications: No clients registered`
This is normal if no users have subscribed to notifications
Run 'mangahub server health' for detailed diagnostics
```

### Stop Servers
```
# Stop all servers
mangahub server stop

# Stop specific server
mangahub server stop --component http
```


### Server Status

```
# Check server status
mangahub server status

# Detailed health check
mangahub server health
```
### Server Logs
```
# View server logs
mangahub server logs

# Follow logs in real-time
mangahub server logs --follow

# Filter logs by level
mangahub server logs --level error
```
## Configuration
### View Configuration
```
# Show current configuration
mangahub config show

# Show specific section
mangahub config show server
```
### Update Configuration
```
# Set configuration value
mangahub config set server.host "192.168.1.100"
mangahub config set notifications.enabled false

# Reset to defaults
mangahub config reset
```
### Profile Management
```
# Create new profile
mangahub profile create --name work

# Switch profiles
mangahub profile switch --name work

# List profiles
mangahub profile list

```
## Advanced Features
### Batch Operations
```
# Batch add manga to library
mangahub library batch-add --file manga-list.txt --status plan-to-read

# Batch update progress
mangahub progress batch-update --file progress-updates.csv
```
### Backup and Restore
```
# Create backup
mangahub backup create --output backup-2024.tar.gz

# Restore from backup
mangahub backup restore --input backup-2024.tar.gz
```
### Database Operations
```
# Database integrity check
mangahub db check

# Optimize database
mangahub db optimize

# Database statistics
mangahub db stats
```
## Troubleshooting
### Common Issues

_Authentication Problems_
```
# Clear authentication data
mangahub auth clear

# Re-register if needed
mangahub auth register --username <username> --email <email>

```

**Expected Output for** **`mangahub auth clear`** **:**

```
Clearing authentication data...

✓ `Authentication token removed`
✓ `User session cleared`
✓ `Sync connections terminated`
✓ `Cache cleared`

You are now logged out. To continue using MangaHub:

mangahub auth login --username <your-username>
```
Or register a new account:
```
mangahub auth register --username <username> --email <email>

```

_Connection Issues_
```
# Test server connectivity
mangahub server ping

# Reset network connections
mangahub sync reconnect

```

**Expected Output for** **`mangahub server ping`** **:**

```
Testing MangaHub server connectivity...


`HTTP API (localhost:8080):` ✓ `Online (15ms)`
`└─ Authentication endpoint:` ✓ `Responding`
`└─ Manga search endpoint:` ✓ `Responding`
`└─ Database connection:` ✓ `Active`

`TCP Sync (localhost:9090):` ✓ `Online (8ms)`
`└─ Connection accepted:` ✓ `Success`
`└─ Authentication test:` ✓ `Success`
`└─ Heartbeat response:` ✓ `Success`

`UDP Notify (localhost:9091):` ✓ `Online (3ms)`
`└─ Registration test:` ✓ `Success`
`└─ Echo test:` ✓ `Success`

`gRPC Service (localhost:9092):` ✓ `Online (12ms)`
`└─ Health check:` ✓ `Serving`
`└─ Service discovery:` ✓ `3 services found`

`WebSocket Chat (localhost:9093):` ✓ `Online (18ms)`
`└─ Upgrade handshake:` ✓ `Success`
`└─ Echo test:` ✓ `Success`

`Overall connectivity:` ✓ `All services reachable`

Network quality: Excellent

```

**Connection Issues Output:**

```
Testing MangaHub server connectivity...


`HTTP API (localhost:8080):` ✗ `Timeout (>5000ms)`
└─ Error: Connection refused


`TCP Sync (localhost:9090):` ✗ `Failed`
└─ Error: No route to host


`UDP Notify (localhost:9091):` ⚠ `Partial (250ms)`
`└─ Registration test:` ✗ `Timeout`
`└─ Echo test:` ✓ `Success (slow)`

`gRPC Service (localhost:9092):` ✗ `Failed`
└─ Error: Connection refused

`WebSocket Chat (localhost:9093):` ✗ `Failed`
└─ Error: Connection refused

`Overall connectivity:` ✗ `Major issues detected`

Troubleshooting suggestions:
1. Check if servers are running: mangahub server status

2. Start servers: mangahub server start

3. Check firewall settings

4. Verify config file: mangahub config show server

5. Check logs: mangahub server logs --level error
```

_Database Issues_

```
# Repair database
mangahub db repair

# Reinitialize if needed
mangahub init --force

```

**Expected Output for** **`mangahub db repair`** **:**

```
Running database integrity check and repair...

Database: ~/.mangahub/data.db
Size: 2.3 MB

Checking tables...

`users table:` ✓ `15 records, no corruption`
`manga table:` ✓ `42 records, no corruption`
`user_progress table:` ⚠ `127 records, 3 orphaned entries found`

Repairing issues...


✓ `Removed 3 orphaned progress entries`
✓ `Rebuilt indexes for performance`
✓ `Updated database statistics`
✓ `Compressed database (saved 0.3 MB)`

Database repair completed successfully!

Summary:
Issues found: 3 orphaned entries
Issues fixed: 3
Performance: Improved (faster queries expected)
Size after repair: 2.0 MB

Your data is intact and the database is now optimized.

```

**Database Corruption Output:**

```
Running database integrity check and repair...

Database: ~/.mangahub/data.db
Size: 2.3 MB

✗ `Critical database corruption detected!`

Issues found:
- users table: 5 corrupted records
- manga table: Schema mismatch
- user_progress table: Index corruption


⚠ `Automatic repair failed. Manual intervention required.`

Recovery options:
1. Restore from backup:
mangahub backup restore --input backup-2024.tar.gz

2. Reinitialize database (DESTROYS ALL DATA):
mangahub init --force --wipe-data

3. Export recoverable data first:
mangahub export library --output library-backup.json --ignore-errors

Contact support if you need assistance with data recovery.

```

_Connection Issues_
```
# Test server connectivity
mangahub server ping

# Reset network connections
mangahub sync reconnect
```
_Database Issues_
```
# Repair database
mangahub db repair

# Reinitialize if needed
mangahub init --force
```
### Debug Mode
```
# Run with debug logging
mangahub --verbose <command>

# Enable trace logging
mangahub config set logging.level trace
```
### Log Analysis
```
# View error logs
mangahub logs errors

# Search logs
mangahub logs search "connection failed"

# Clear old logs
mangahub logs clean --older-than 30d
```
## Examples and Use Cases
### Daily Usage Workflow
```
# Morning routine
mangahub server start &
mangahub sync connect
mangahub notify subscribe

# Check for new chapters
mangahub manga search --new-chapters

# Update reading progress
mangahub progress update --manga-id current-read --chapter 42

# Join community chat
mangahub chat join
```
### Bulk Library Management
```
# Export current library for backup
mangahub export library --format json --output backup.json

# Import manga from another service
mangahub import --format mal --input myanimelist-export.xml

# Bulk status update
mangahub library batch-update --status completed --file completed-manga.txt
```
### Server Administration
```
# Start production server
mangahub server start --config production.yaml --daemon

# Monitor server health
mangahub server health --continuous

# Rotate logs
mangahub logs rotate
```
## API Integration
### Custom Scripts
The CLI can be used in shell scripts:

```
#!/bin/bash
# Auto-update reading progress from external source

while IFS=',' read -r manga_id chapter ; do
mangahub progress update --manga-id "$manga_id" --chapter "$chapter"
done < progress-updates.csv
```
### JSON Output
Most commands support JSON output for programmatic use:
```
# Get library as JSON
mangahub library list --output json

# Search results as JSON
mangahub manga search "keyword" --output json | jq '.results[].title'
```
## Support and Updates
### Getting Help
```
# General help
mangahub help

# Command-specific help

mangahub manga help
mangahub library help
```
### Version Information
```
# Check version
mangahub version

# Check for updates
mangahub update check

# Update to latest version
mangahub update install
```
### Bug Reports
To report issues: 
1. Run command with `--verbose` flag
2. Check logs: `mangahub logs errors`
3. Include system info: `mangahub system info`

This manual covers all major functionality of the MangaHub CLI application. For additional
features or specific use cases, refer to the built-in help system or consult the online
documentation.


