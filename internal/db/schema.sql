-- Blackbox SQLite Database Schema
-- This schema stores recordings, transcripts, and AI summaries with full metadata

-- Recordings table - stores audio file metadata
CREATE TABLE IF NOT EXISTS recordings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL UNIQUE,           -- Original filename (e.g., "20250123_143022.wav")
    display_name TEXT,                       -- Human-readable name (optional)
    file_path TEXT NOT NULL UNIQUE,          -- Full path to WAV file
    file_size INTEGER NOT NULL,              -- File size in bytes
    duration_seconds REAL,                   -- Audio duration in seconds
    sample_rate INTEGER NOT NULL,            -- Audio sample rate (Hz)
    channels INTEGER NOT NULL,               -- Number of audio channels
    bits_per_sample INTEGER NOT NULL,        -- Bits per sample (16 for S16LE)
    audio_format TEXT NOT NULL,              -- Audio format ("PCM S16LE")
    recording_mode TEXT NOT NULL,            -- "loopback", "dictation", "mixed"
    with_microphone BOOLEAN DEFAULT FALSE,   -- Whether microphone was mixed in
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    recorded_at DATETIME,                    -- When recording actually started
    notes TEXT,                              -- Optional user notes
    tags TEXT,                               -- Comma-separated tags for organization

    -- Constraints
    CHECK (recording_mode IN ('loopback', 'dictation', 'mixed')),
    CHECK (sample_rate > 0),
    CHECK (channels > 0),
    CHECK (bits_per_sample IN (8, 16, 24, 32)),
    CHECK (file_size >= 0)
);

-- Transcripts table - stores transcription data linked to recordings
CREATE TABLE IF NOT EXISTS transcripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recording_id INTEGER NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    content TEXT NOT NULL,                   -- Full transcript text
    confidence_score REAL,                   -- Overall confidence (0.0-1.0) if available
    model_used TEXT NOT NULL,                -- Whisper model name (e.g., "ggml-base.en")
    language TEXT NOT NULL DEFAULT 'en',     -- Detected/requested language
    processing_time_seconds REAL,            -- Time taken to transcribe
    whisper_version TEXT,                    -- Whisper version if available
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,


    -- Constraints
    CHECK (confidence_score IS NULL OR (confidence_score >= 0.0 AND confidence_score <= 1.0)),
    CHECK (processing_time_seconds IS NULL OR processing_time_seconds >= 0),
    UNIQUE(recording_id, model_used) -- Prevent duplicate transcriptions
);

-- Summaries table - stores AI-generated summaries linked to transcripts
CREATE TABLE IF NOT EXISTS summaries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    transcript_id INTEGER NOT NULL REFERENCES transcripts(id) ON DELETE CASCADE,
    content TEXT NOT NULL,                   -- Summary text
    summary_type TEXT NOT NULL,              -- "meeting", "dictation", "technical", etc.
    model_used TEXT NOT NULL,                -- AI model name (e.g., "gpt-4o-mini", "llama-3")
    temperature REAL,                        -- AI temperature used
    prompt_used TEXT NOT NULL,               -- Full prompt that was used
    processing_time_seconds REAL,            -- Time taken to generate summary
    api_endpoint TEXT,                       -- API endpoint used (for remote AI)
    local_model_path TEXT,                   -- Local model path (for local AI)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    CHECK (summary_type != ''),
    CHECK (model_used != ''),
    CHECK (temperature IS NULL OR (temperature >= 0.0 AND temperature <= 2.0)),
    CHECK (processing_time_seconds IS NULL OR processing_time_seconds >= 0)
);

-- Processing metadata table - tracks processing history and logs
CREATE TABLE IF NOT EXISTS processing_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recording_id INTEGER REFERENCES recordings(id) ON DELETE SET NULL,
    transcript_id INTEGER REFERENCES transcripts(id) ON DELETE SET NULL,
    summary_id INTEGER REFERENCES summaries(id) ON DELETE SET NULL,
    process_type TEXT NOT NULL,              -- "transcription", "summarization"
    status TEXT NOT NULL,                    -- "started", "completed", "failed"
    model_used TEXT,                         -- Model used for this processing step
    parameters TEXT,                         -- JSON parameters used
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration_seconds REAL,
    error_message TEXT,                      -- Error details if failed
    log_file_path TEXT,                      -- Path to log file if any

    -- Constraints
    CHECK (process_type IN ('transcription', 'summarization')),
    CHECK (status IN ('started', 'completed', 'failed')),
    CHECK (duration_seconds IS NULL OR duration_seconds >= 0)
);

-- Tags table for flexible organization
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT,                              -- Hex color for UI display
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Recording-Tag junction table (many-to-many)
CREATE TABLE IF NOT EXISTS recording_tags (
    recording_id INTEGER NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (recording_id, tag_id)
);

-- Search index for full-text search on transcripts
-- Note: SQLite FTS5 provides better performance than FTS4
CREATE VIRTUAL TABLE IF NOT EXISTS transcript_search USING fts5(
    content,
    recording_id UNINDEXED,
    transcript_id UNINDEXED,
    tokenize = 'porter ascii'
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_recordings_created_at ON recordings(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_recordings_recorded_at ON recordings(recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_recordings_mode ON recordings(recording_mode);
CREATE INDEX IF NOT EXISTS idx_recordings_tags ON recording_tags(recording_id);

CREATE INDEX IF NOT EXISTS idx_transcripts_recording_id ON transcripts(recording_id);
CREATE INDEX IF NOT EXISTS idx_transcripts_created_at ON transcripts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transcripts_model ON transcripts(model_used);

CREATE INDEX IF NOT EXISTS idx_summaries_transcript_id ON summaries(transcript_id);
CREATE INDEX IF NOT EXISTS idx_summaries_type ON summaries(summary_type);
CREATE INDEX IF NOT EXISTS idx_summaries_model ON summaries(model_used);
CREATE INDEX IF NOT EXISTS idx_summaries_created_at ON summaries(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_processing_metadata_recording ON processing_metadata(recording_id);
CREATE INDEX IF NOT EXISTS idx_processing_metadata_transcript ON processing_metadata(transcript_id);
CREATE INDEX IF NOT EXISTS idx_processing_metadata_status ON processing_metadata(status);
CREATE INDEX IF NOT EXISTS idx_processing_metadata_start_time ON processing_metadata(start_time DESC);

-- Trigger to keep transcript search index in sync
CREATE TRIGGER IF NOT EXISTS trigger_transcript_insert
AFTER INSERT ON transcripts
BEGIN
    INSERT INTO transcript_search (recording_id, transcript_id, content)
    VALUES (new.recording_id, new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS trigger_transcript_delete
AFTER DELETE ON transcripts
BEGIN
    DELETE FROM transcript_search WHERE transcript_id = old.id;
END;

CREATE TRIGGER IF NOT EXISTS trigger_transcript_update
AFTER UPDATE ON transcripts
BEGIN
    UPDATE transcript_search
    SET content = new.content
    WHERE transcript_id = old.id;
END;

-- Views for common queries
CREATE VIEW IF NOT EXISTS recordings_with_transcripts AS
SELECT
    r.id,
    r.filename,
    r.display_name,
    r.duration_seconds,
    r.recording_mode,
    r.with_microphone,
    r.created_at,
    r.recorded_at,
    r.tags,
    t.id as transcript_id,
    t.content as transcript_content,
    t.model_used as transcript_model,
    t.confidence_score,
    t.created_at as transcribed_at,
    s.id as summary_id,
    s.content as summary_content,
    s.summary_type,
    s.model_used as summary_model,
    s.created_at as summarized_at
FROM recordings r
LEFT JOIN transcripts t ON r.id = t.recording_id
LEFT JOIN summaries s ON t.id = s.transcript_id
ORDER BY r.created_at DESC;

CREATE VIEW IF NOT EXISTS processing_stats AS
SELECT
    process_type,
    status,
    COUNT(*) as count,
    AVG(duration_seconds) as avg_duration,
    MIN(duration_seconds) as min_duration,
    MAX(duration_seconds) as max_duration,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failures
FROM processing_metadata
WHERE end_time IS NOT NULL
GROUP BY process_type, status;
