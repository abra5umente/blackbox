-- Migration: Add Audio Data BLOB Storage
-- Version: 2
-- Description: Adds audio_data column to store actual WAV data in the database

-- Add audio_data BLOB column to recordings table
-- This allows storing the actual audio data directly in the database
-- instead of relying on external files
ALTER TABLE recordings ADD COLUMN audio_data BLOB;

-- Add index for audio_data to improve query performance (optional, but recommended)
-- Note: SQLite doesn't index BLOB columns, but we can create an index on audio_data size
CREATE INDEX IF NOT EXISTS idx_recordings_audio_size ON recordings(LENGTH(audio_data));

-- Update recordings table to make file_path optional since we now store data directly
-- We'll keep file_path for reference but make it nullable
-- Note: In a production system, we might want to drop file_path entirely,
-- but keeping it for now as a migration step
UPDATE recordings SET file_path = '' WHERE file_path IS NULL;
