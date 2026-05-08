CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE IF NOT EXISTS manga (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    author TEXT,
    genres TEXT,
    status TEXT,
    total_chapters INTEGER DEFAULT 0,
    description TEXT
);


CREATE TABLE IF NOT EXISTS user_progress (
    user_id TEXT,
    manga_id TEXT,
    current_chapter INTEGER DEFAULT 0,
    current_volume INTEGER DEFAULT 0,
    status TEXT,
    rating INTEGER DEFAULT 0,
    notes TEXT DEFAULT '',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, manga_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (manga_id) REFERENCES manga(id)
);