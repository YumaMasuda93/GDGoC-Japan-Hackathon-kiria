CREATE TABLE IF NOT EXISTS audio_embeddings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_filename TEXT NOT NULL,
    source_path TEXT NOT NULL UNIQUE,
    mime_type TEXT NOT NULL,
    file_size_bytes INTEGER NOT NULL,
    embedding_model TEXT NOT NULL,
    embedding_json TEXT NOT NULL,
    embedding_dimensions INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
