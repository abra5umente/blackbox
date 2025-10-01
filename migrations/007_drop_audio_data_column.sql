-- Migration: Drop audio_data Column
-- Version: 7
-- Description: Completely removes the audio_data column from recordings table.
--              SQLite doesn't support DROP COLUMN directly, so we recreate the table.
--              This migration reclaims disk space and enforces the project philosophy:
--              audio is NEVER stored in the database.

-- Step 1: Drop views that depend on the recordings table
DROP VIEW IF EXISTS recordings_with_transcripts;

-- Step 2: Create new recordings table without audio_data column
CREATE TABLE recordings_new (
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
    error_message TEXT,
    retry_file_path TEXT,
    CHECK (recording_mode IN ('loopback', 'dictation', 'mixed')),
    CHECK (sample_rate > 0),
    CHECK (channels > 0),
    CHECK (bits_per_sample IN (8, 16, 24, 32)),
    CHECK (file_size >= 0)
);

-- Step 3: Copy data from old table to new table (excluding audio_data)
INSERT INTO recordings_new (
    id, filename, display_name, file_path, file_size, duration_seconds,
    sample_rate, channels, bits_per_sample, audio_format, recording_mode,
    with_microphone, created_at, recorded_at, notes, tags, error_message, retry_file_path
)
SELECT
    id, filename, display_name, file_path, file_size, duration_seconds,
    sample_rate, channels, bits_per_sample, audio_format, recording_mode,
    with_microphone, created_at, recorded_at, notes, tags, error_message, retry_file_path
FROM recordings;

-- Step 4: Drop old table and rename new table
DROP TABLE recordings;
ALTER TABLE recordings_new RENAME TO recordings;

-- Step 5: Recreate indexes on the new table
CREATE INDEX IF NOT EXISTS idx_recordings_filename ON recordings(filename);
CREATE INDEX IF NOT EXISTS idx_recordings_created_at ON recordings(created_at);
CREATE INDEX IF NOT EXISTS idx_recordings_recording_mode ON recordings(recording_mode);

-- Step 6: Recreate the recordings_with_transcripts view
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

-- Note: Foreign key constraints from transcripts table remain intact
-- because we preserved the id column values during the copy operation.
-- SQLite will automatically re-establish the foreign key relationship.
