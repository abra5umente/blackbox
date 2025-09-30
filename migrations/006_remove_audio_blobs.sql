-- Migration: Remove Audio BLOBs from Database
-- Version: 6
-- Description: Removes all audio_data BLOBs from recordings table to reclaim disk space.
--              Audio is no longer stored in database - WAV files are now temporary and deleted after transcription.
--              This migration is IRREVERSIBLE - audio data will be permanently deleted.

-- Remove all audio data from recordings
UPDATE recordings SET audio_data = NULL;

-- Note: To reclaim disk space, run VACUUM manually after this migration:
-- sqlite3 ./data/blackbox.db "VACUUM;"
-- (VACUUM cannot be run inside a transaction, so it must be done separately)