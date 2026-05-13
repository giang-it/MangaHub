# MangaHub - Installation & Setup Guide

This guide provides step-by-step instructions to set up and run the MangaHub system.

## 1. Prerequisites

Before you begin, ensure you have the following installed:
- **Git**: To clone the repository.
- **Docker & Docker Compose**: (Recommended) For quick setup.
- **Go (v1.22+)**: (Optional) If you want to build and run manually.

---

## 2. Clone the Project

Open your terminal (PowerShell, Command Prompt, or Bash) and run:

```bash
git clone https://github.com/giang-it/MangaHub.git
cd MangaHub
```

---

## 3. Option 1: Running with Docker (Recommended)

This is the fastest way to get the system up and running with all services (API, TCP, UDP, gRPC, WebSocket) pre-configured.

### Step 1: Start the containers
```bash
docker-compose up -d
```

### Step 2: Verify the services
The server will be available at:
- **API Server**: `http://localhost:8080`
- **TCP Sync**: `localhost:8081`
- **UDP Notify**: `localhost:9091`
- **gRPC**: `localhost:9092`
- **WebSocket Chat**: `ws://localhost:9093`

---

## 4. Option 2: Manual Build & Run

If you prefer to run the project without Docker, follow these steps:

### Step 1: Install dependencies
```bash
go mod download
```

### Step 2: Build the CLI & Server
```bash
# Build for Windows
go build -o mangahub.exe ./cmd/api-server/main.go

# Build for Linux/macOS
go build -o mangahub ./cmd/api-server/main.go
```

### Step 3: Start the Server
```bash
# Windows
.\mangahub.exe server start

# Linux/macOS
./mangahub server start
```

---

## 5. How to Test

Once the server is running, you can use the built-in CLI commands to interact with the system.

### Basic Workflow:
1. **Register a user**:
   ```bash
   .\mangahub.exe --session c1 auth register --username alice
   # Enter password: Test1234
   ```
2. **Login**:
   ```bash
   .\mangahub.exe --session c1 auth login --username alice
   ```
3. **Search for Manga**:
   ```bash
   .\mangahub.exe manga search "attack"
   ```

> [!TIP]
> For a full list of test cases (over 100 cases covering all protocols), please refer to:
> **[testing_guide.md](.\docs\testing_guide.md)**

---

## 6. Project Structure

- `cmd/`: Entry points for the application.
- `internal/`: Core business logic (Auth, Manga, Library, Chat).
- `pkg/`: Shared utilities (Database, Config, Protocol Handlers).
- `data/`: Contains the initial database seed (`manga_seed.json`).
- `testing_guide_new.md`: Comprehensive test suite.

---

## 7. Troubleshooting

- **Port already in use**: Ensure ports 8080, 8081, 9091, 9092, and 9093 are available.
- **Database issue**: If the database gets corrupted, simply delete `data/mangahub.db` and restart the server.
- **Docker logs**: Use `docker-compose logs -f` to see real-time server activity.
