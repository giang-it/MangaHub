# MangaHub API Documentation

**Base URL:** `http://localhost:8080`  
**Auth Method:** Bearer Token (Header: `Authorization: Bearer <JWT_TOKEN>`)

---

## 1. Authentication & User Management

### • Register Account
- **Endpoint:** `POST /auth/register`
- **Description:** Register a new user account.
- **Request Body (JSON):**
```json
{
  "username": "testuser",
  "email": "test@example.com",
  "password": "Password123"
}
```

### • Login
- **Endpoint:** `POST /auth/login`
- **Description:** Log in and receive a JWT Token.
- **Request Body (JSON):**
```json
{
  "username": "testuser",
  "password": "Password123"
}
```

### • Check Auth Status
- **Endpoint:** `GET /auth/status`
- **Auth Required:** Yes
- **Description:** Get information of the currently logged-in account.

### • Change Password
- **Endpoint:** `POST /auth/change-password`
- **Auth Required:** Yes
- **Description:** Change the user's password.
- **Request Body (JSON):**
```json
{
  "old_password": "OldPassword123",
  "new_password": "NewPassword123"
}
```

---

## 2. Manga Management

### • Search Manga
- **Endpoint:** `GET /manga`
- **Description:** Search and filter the manga list.
- **Query Parameters:**
  - `q`: Search keyword.
  - `genre`: Genre (Action, Shounen, ...).
  - `status`: Status (ongoing, completed).
  - `limit`: Quantity limit (default: 20).
  - `page`: Current page.

### • Get Manga Details
- **Endpoint:** `GET /manga/:id`
- **Description:** Get detailed information of a manga by ID.

### • Create/Update/Delete Manga (Admin Only)
- **Auth Required:** Yes
- **Endpoints:**
  - `POST /manga`: Add new manga.
  - `PUT /manga/:id`: Update manga information.
  - `DELETE /manga/:id`: Delete manga from the system.

---

## 3. User Library & Reading Progress

### • Add to Library
- **Endpoint:** `POST /users/library`
- **Auth Required:** Yes
- **Description:** Add manga to the personal library.
- **Request Body:** `{ "manga_id": "string", "status": "string" }`

### • Get Library
- **Endpoint:** `GET /users/library`
- **Auth Required:** Yes
- **Description:** Get the list of manga in the library.
- **Query Params:** `status` (to filter by Reading, Completed, ...).

### • Update Progress
- **Endpoint:** `PUT /users/progress`
- **Auth Required:** Yes
- **Description:** Update the current reading chapter. This command will trigger a Broadcast via TCP.
- **Request Body:** `{ "manga_id": "string", "chapter": 50 }`

---

## 4. Chat & Real-time Community

### • Get Online Users
- **Endpoint:** `GET /chat/users`
- **Description:** Get the list of online users in all rooms.

### • Get Chat History
- **Endpoint:** `GET /chat/history`
- **Description:** Get the 50 most recent messages of a room.
- **Query Params:** `room` (General or Manga ID).

### • Send Chat Message (via HTTP)
- **Endpoint:** `POST /chat/send`
- **Description:** Send a message to the system (typically used for CLI).
- **Request Body (JSON):**
```json
{
  "username": "string",
  "room": "string",
  "message": "string",
  "is_private": false,
  "target_user": ""
}
```

---

## 5. Network Protocols (Non-HTTP)

### • WebSocket (Chat Server)
- **URL:** `ws://localhost:9093/ws`
- **Params:** `user_id`, `username`, `room`.
- **Description:** Real-time connection to receive and send messages.

### • gRPC Service (Internal)
- **Address:** `localhost:9092`
- **Methods:**
  - `GetManga`: Query manga information.
  - `SearchManga`: Search manga.
  - `UpdateProgress`: Update reading progress.

### • TCP Sync Server
- **Address:** `localhost:8081`
- **Description:** Socket connection to receive progress synchronization messages (JSON format).

### • UDP Notification
- **Address:** `localhost:9091`
- **Actions:** `register`, `unregister`, `broadcast`.
- **Description:** Receive new chapter notifications via UDP protocol.
