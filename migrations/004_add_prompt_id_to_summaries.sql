-- Migration 004: Add prompt_id column to summaries table
-- This migration adds the prompt_id column to the summaries table to reference prompts

-- Add prompt_id column to summaries table
-- This will fail if the column already exists, but that's fine
-- We'll handle this gracefully in the application
ALTER TABLE summaries ADD COLUMN prompt_id INTEGER REFERENCES prompts(id);

-- Create index for prompt references in summaries
CREATE INDEX IF NOT EXISTS idx_summaries_prompt_id ON summaries(prompt_id);
