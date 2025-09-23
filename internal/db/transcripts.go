package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateTranscript creates a new transcript in the database
func (db *DB) CreateTranscript(transcript *Transcript) error {
	query := `
		INSERT INTO transcripts (
			recording_id, content, confidence_score, model_used,
			language, processing_time_seconds, whisper_version
		) VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		transcript.RecordingID,
		transcript.Content,
		nullFloat64(transcript.ConfidenceScore),
		transcript.ModelUsed,
		transcript.Language,
		nullFloat64(transcript.ProcessingTimeSeconds),
		nullString(transcript.WhisperVersion),
	)
	if err != nil {
		return fmt.Errorf("failed to create transcript: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get transcript ID: %w", err)
	}

	transcript.ID = int(id)
	transcript.CreatedAt = time.Now()
	return nil
}

// GetTranscript retrieves a transcript by ID
func (db *DB) GetTranscript(id int) (*Transcript, error) {
	query := `
		SELECT id, recording_id, content, confidence_score, model_used,
		       language, processing_time_seconds, whisper_version, created_at
		FROM transcripts WHERE id = ?`

	var transcript Transcript
	var confidenceScore sql.NullFloat64
	var processingTimeSeconds sql.NullFloat64
	var whisperVersion sql.NullString

	err := db.QueryRow(query, id).Scan(
		&transcript.ID,
		&transcript.RecordingID,
		&transcript.Content,
		&confidenceScore,
		&transcript.ModelUsed,
		&transcript.Language,
		&processingTimeSeconds,
		&whisperVersion,
		&transcript.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transcript not found")
		}
		return nil, fmt.Errorf("failed to get transcript: %w", err)
	}

	transcript.ConfidenceScore = float64Ptr(confidenceScore)
	transcript.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
	transcript.WhisperVersion = stringPtr(whisperVersion)

	return &transcript, nil
}

// GetTranscriptByRecordingID retrieves the transcript for a recording
func (db *DB) GetTranscriptByRecordingID(recordingID int) (*Transcript, error) {
	query := `
		SELECT id, recording_id, content, confidence_score, model_used,
		       language, processing_time_seconds, whisper_version, created_at
		FROM transcripts WHERE recording_id = ?`

	var transcript Transcript
	var confidenceScore sql.NullFloat64
	var processingTimeSeconds sql.NullFloat64
	var whisperVersion sql.NullString

	err := db.QueryRow(query, recordingID).Scan(
		&transcript.ID,
		&transcript.RecordingID,
		&transcript.Content,
		&confidenceScore,
		&transcript.ModelUsed,
		&transcript.Language,
		&processingTimeSeconds,
		&whisperVersion,
		&transcript.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transcript not found")
		}
		return nil, fmt.Errorf("failed to get transcript: %w", err)
	}

	transcript.ConfidenceScore = float64Ptr(confidenceScore)
	transcript.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
	transcript.WhisperVersion = stringPtr(whisperVersion)

	return &transcript, nil
}

// GetTranscriptByRecordingIDAndModel retrieves a specific transcript by recording ID and model
func (db *DB) GetTranscriptByRecordingIDAndModel(recordingID int, model string) (*Transcript, error) {
	query := `
		SELECT id, recording_id, content, confidence_score, model_used,
		       language, processing_time_seconds, whisper_version, created_at
		FROM transcripts WHERE recording_id = ? AND model_used = ?`

	var transcript Transcript
	var confidenceScore sql.NullFloat64
	var processingTimeSeconds sql.NullFloat64
	var whisperVersion sql.NullString

	err := db.QueryRow(query, recordingID, model).Scan(
		&transcript.ID,
		&transcript.RecordingID,
		&transcript.Content,
		&confidenceScore,
		&transcript.ModelUsed,
		&transcript.Language,
		&processingTimeSeconds,
		&whisperVersion,
		&transcript.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transcript not found")
		}
		return nil, fmt.Errorf("failed to get transcript: %w", err)
	}

	transcript.ConfidenceScore = float64Ptr(confidenceScore)
	transcript.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
	transcript.WhisperVersion = stringPtr(whisperVersion)

	return &transcript, nil
}

// ListTranscripts retrieves transcripts with optional filtering
func (db *DB) ListTranscripts(limit, offset int, recordingID *int, model *string) ([]*Transcript, error) {
	query := `
		SELECT id, recording_id, content, confidence_score, model_used,
		       language, processing_time_seconds, whisper_version, created_at
		FROM transcripts WHERE 1=1`

	args := []interface{}{}
	if recordingID != nil {
		query += " AND recording_id = ?"
		args = append(args, *recordingID)
	}
	if model != nil {
		query += " AND model_used = ?"
		args = append(args, *model)
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list transcripts: %w", err)
	}
	defer rows.Close()

	var transcripts []*Transcript
	for rows.Next() {
		var transcript Transcript
		var confidenceScore sql.NullFloat64
		var processingTimeSeconds sql.NullFloat64
		var whisperVersion sql.NullString

		err := rows.Scan(
			&transcript.ID,
			&transcript.RecordingID,
			&transcript.Content,
			&confidenceScore,
			&transcript.ModelUsed,
			&transcript.Language,
			&processingTimeSeconds,
			&whisperVersion,
			&transcript.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transcript: %w", err)
		}

		transcript.ConfidenceScore = float64Ptr(confidenceScore)
		transcript.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
		transcript.WhisperVersion = stringPtr(whisperVersion)

		transcripts = append(transcripts, &transcript)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transcripts: %w", err)
	}

	return transcripts, nil
}

// UpdateTranscript updates an existing transcript
func (db *DB) UpdateTranscript(transcript *Transcript) error {
	query := `
		UPDATE transcripts SET
			content = ?, confidence_score = ?, language = ?,
			processing_time_seconds = ?, whisper_version = ?
		WHERE id = ?`

	result, err := db.Exec(query,
		transcript.Content,
		nullFloat64(transcript.ConfidenceScore),
		transcript.Language,
		nullFloat64(transcript.ProcessingTimeSeconds),
		nullString(transcript.WhisperVersion),
		transcript.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update transcript: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("transcript not found")
	}

	return nil
}

// DeleteTranscript deletes a transcript
func (db *DB) DeleteTranscript(id int) error {
	query := "DELETE FROM transcripts WHERE id = ?"

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete transcript: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("transcript not found")
	}

	return nil
}

// SearchTranscripts performs full-text search on transcript content
func (db *DB) SearchTranscripts(query string, limit, offset int) ([]*SearchResult, error) {
	// Use FTS5 search
	searchQuery := `
		SELECT ts.recording_id, ts.transcript_id, r.filename, ts.content,
		       ts.rank, r.created_at, t.created_at as transcribed_at
		FROM transcript_search ts
		JOIN recordings r ON ts.recording_id = r.id
		LEFT JOIN transcripts t ON ts.transcript_id = t.id
		WHERE transcript_search MATCH ?
		ORDER BY ts.rank
		LIMIT ? OFFSET ?`

	rows, err := db.Query(searchQuery, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search transcripts: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		var result SearchResult
		var transcribedAt sql.NullTime

		err := rows.Scan(
			&result.RecordingID,
			&result.TranscriptID,
			&result.Filename,
			&result.Content,
			&result.Rank,
			&result.CreatedAt,
			&transcribedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		if transcribedAt.Valid {
			result.TranscribedAt = &transcribedAt.Time
		}

		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}

// GetTranscriptsByDateRange retrieves transcripts created within a date range
func (db *DB) GetTranscriptsByDateRange(start, end time.Time, limit, offset int) ([]*Transcript, error) {
	query := `
		SELECT id, recording_id, content, confidence_score, model_used,
		       language, processing_time_seconds, whisper_version, created_at
		FROM transcripts
		WHERE created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := db.Query(query, start, end, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcripts by date range: %w", err)
	}
	defer rows.Close()

	var transcripts []*Transcript
	for rows.Next() {
		var transcript Transcript
		var confidenceScore sql.NullFloat64
		var processingTimeSeconds sql.NullFloat64
		var whisperVersion sql.NullString

		err := rows.Scan(
			&transcript.ID,
			&transcript.RecordingID,
			&transcript.Content,
			&confidenceScore,
			&transcript.ModelUsed,
			&transcript.Language,
			&processingTimeSeconds,
			&whisperVersion,
			&transcript.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transcript: %w", err)
		}

		transcript.ConfidenceScore = float64Ptr(confidenceScore)
		transcript.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
		transcript.WhisperVersion = stringPtr(whisperVersion)

		transcripts = append(transcripts, &transcript)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transcripts: %w", err)
	}

	return transcripts, nil
}
