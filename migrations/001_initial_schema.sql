-- Migration: Initial Schema
-- Version: 1
-- Description: Creates all initial tables, indexes, triggers, and views

-- Recordings table - stores audio file metadata
CREATE TABLE IF NOT EXISTS recordings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL UNIQUE,
    display_name TEXT,
    file_path TEXT NOT NULL UNIQUE,
    file_size INTEGER NOT NULL,
    duration_seconds REAL,
    sample_rate INTEGER NOT NULL,
    channels INTEGER NOT NULL,
    bits_per_sample INTEGER NOT NULL,
    audio_format TEXT NOT NULL,
    recording_mode TEXT NOT NULL,
    with_microphone BOOLEAN DEFAULT FALSE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    recorded_at DATETIME,
    notes TEXT,
    tags TEXT,
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
    content TEXT NOT NULL,
    confidence_score REAL,
    model_used TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT 'en',
    processing_time_seconds REAL,
    whisper_version TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (confidence_score IS NULL OR (confidence_score >= 0.0 AND confidence_score <= 1.0)),
    CHECK (processing_time_seconds IS NULL OR processing_time_seconds >= 0),
    UNIQUE(recording_id, model_used)
);

-- Summaries table - stores AI-generated summaries linked to transcripts
CREATE TABLE IF NOT EXISTS summaries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    transcript_id INTEGER NOT NULL REFERENCES transcripts(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    summary_type TEXT NOT NULL,
    model_used TEXT NOT NULL,
    temperature REAL,
    prompt_used TEXT NOT NULL,
    processing_time_seconds REAL,
    api_endpoint TEXT,
    local_model_path TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (summary_type != ''),
    CHECK (model_used != ''),
    CHECK (temperature IS NULL OR (temperature >= 0.0 AND temperature <= 2.0)),
    CHECK (processing_time_seconds IS NULL OR processing_time_seconds >= 0)
);

-- Tags table for flexible organization
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT,
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

-- Processing metadata table - tracks processing history and logs
CREATE TABLE IF NOT EXISTS processing_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recording_id INTEGER REFERENCES recordings(id) ON DELETE SET NULL,
    transcript_id INTEGER REFERENCES transcripts(id) ON DELETE SET NULL,
    summary_id INTEGER REFERENCES summaries(id) ON DELETE SET NULL,
    process_type TEXT NOT NULL,
    status TEXT NOT NULL,
    model_used TEXT,
    parameters TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration_seconds REAL,
    error_message TEXT,
    log_file_path TEXT,
    CHECK (process_type IN ('transcription', 'summarization')),
    CHECK (status IN ('started', 'completed', 'failed')),
    CHECK (duration_seconds IS NULL OR duration_seconds >= 0)
);

-- Search index for full-text search on transcripts
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
