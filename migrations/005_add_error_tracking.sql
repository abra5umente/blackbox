-- Migration: Add Error Tracking to Recordings
-- Version: 5
-- Description: Adds error tracking fields for failed transcriptions

-- Add error_message column to store transcription errors
ALTER TABLE recordings ADD COLUMN error_message TEXT;

-- Add retry_file_path column to store path to WAV file in retry/ for failed transcriptions
ALTER TABLE recordings ADD COLUMN retry_file_path TEXT;

-- Add index for failed recordings
CREATE INDEX IF NOT EXISTS idx_recordings_error ON recordings(error_message) WHERE error_message IS NOT NULL;