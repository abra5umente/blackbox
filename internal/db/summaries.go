package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateSummary creates a new summary in the database
func (db *DB) CreateSummary(summary *Summary) error {
	query := `
		INSERT INTO summaries (
			transcript_id, content, summary_type, model_used,
			temperature, prompt_used, processing_time_seconds,
			api_endpoint, local_model_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		summary.TranscriptID,
		summary.Content,
		summary.SummaryType,
		summary.ModelUsed,
		nullFloat64(summary.Temperature),
		summary.PromptUsed,
		nullFloat64(summary.ProcessingTimeSeconds),
		nullString(summary.APIEndpoint),
		nullString(summary.LocalModelPath),
	)
	if err != nil {
		return fmt.Errorf("failed to create summary: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get summary ID: %w", err)
	}

	summary.ID = int(id)
	summary.CreatedAt = time.Now()
	return nil
}

// GetSummary retrieves a summary by ID
func (db *DB) GetSummary(id int) (*Summary, error) {
	query := `
		SELECT id, transcript_id, content, summary_type, model_used,
		       temperature, prompt_used, processing_time_seconds,
		       api_endpoint, local_model_path, created_at
		FROM summaries WHERE id = ?`

	var summary Summary
	var temperature sql.NullFloat64
	var processingTimeSeconds sql.NullFloat64
	var apiEndpoint sql.NullString
	var localModelPath sql.NullString

	err := db.QueryRow(query, id).Scan(
		&summary.ID,
		&summary.TranscriptID,
		&summary.Content,
		&summary.SummaryType,
		&summary.ModelUsed,
		&temperature,
		&summary.PromptUsed,
		&processingTimeSeconds,
		&apiEndpoint,
		&localModelPath,
		&summary.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("summary not found")
		}
		return nil, fmt.Errorf("failed to get summary: %w", err)
	}

	summary.Temperature = float64Ptr(temperature)
	summary.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
	summary.APIEndpoint = stringPtr(apiEndpoint)
	summary.LocalModelPath = stringPtr(localModelPath)

	return &summary, nil
}

// GetSummaryByTranscriptID retrieves the summary for a transcript
func (db *DB) GetSummaryByTranscriptID(transcriptID int) (*Summary, error) {
	query := `
		SELECT id, transcript_id, content, summary_type, model_used,
		       temperature, prompt_used, processing_time_seconds,
		       api_endpoint, local_model_path, created_at
		FROM summaries WHERE transcript_id = ?`

	var summary Summary
	var temperature sql.NullFloat64
	var processingTimeSeconds sql.NullFloat64
	var apiEndpoint sql.NullString
	var localModelPath sql.NullString

	err := db.QueryRow(query, transcriptID).Scan(
		&summary.ID,
		&summary.TranscriptID,
		&summary.Content,
		&summary.SummaryType,
		&summary.ModelUsed,
		&temperature,
		&summary.PromptUsed,
		&processingTimeSeconds,
		&apiEndpoint,
		&localModelPath,
		&summary.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("summary not found")
		}
		return nil, fmt.Errorf("failed to get summary: %w", err)
	}

	summary.Temperature = float64Ptr(temperature)
	summary.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
	summary.APIEndpoint = stringPtr(apiEndpoint)
	summary.LocalModelPath = stringPtr(localModelPath)

	return &summary, nil
}

// GetSummaryByTranscriptIDAndType retrieves a specific summary by transcript ID and type
func (db *DB) GetSummaryByTranscriptIDAndType(transcriptID int, summaryType string) (*Summary, error) {
	query := `
		SELECT id, transcript_id, content, summary_type, model_used,
		       temperature, prompt_used, processing_time_seconds,
		       api_endpoint, local_model_path, created_at
		FROM summaries WHERE transcript_id = ? AND summary_type = ?`

	var summary Summary
	var temperature sql.NullFloat64
	var processingTimeSeconds sql.NullFloat64
	var apiEndpoint sql.NullString
	var localModelPath sql.NullString

	err := db.QueryRow(query, transcriptID, summaryType).Scan(
		&summary.ID,
		&summary.TranscriptID,
		&summary.Content,
		&summary.SummaryType,
		&summary.ModelUsed,
		&temperature,
		&summary.PromptUsed,
		&processingTimeSeconds,
		&apiEndpoint,
		&localModelPath,
		&summary.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("summary not found")
		}
		return nil, fmt.Errorf("failed to get summary: %w", err)
	}

	summary.Temperature = float64Ptr(temperature)
	summary.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
	summary.APIEndpoint = stringPtr(apiEndpoint)
	summary.LocalModelPath = stringPtr(localModelPath)

	return &summary, nil
}

// ListSummaries retrieves summaries with optional filtering
func (db *DB) ListSummaries(limit, offset int, transcriptID *int, summaryType *string, model *string) ([]*Summary, error) {
	query := `
		SELECT id, transcript_id, content, summary_type, model_used,
		       temperature, prompt_used, processing_time_seconds,
		       api_endpoint, local_model_path, created_at
		FROM summaries WHERE 1=1`

	args := []interface{}{}
	if transcriptID != nil {
		query += " AND transcript_id = ?"
		args = append(args, *transcriptID)
	}
	if summaryType != nil {
		query += " AND summary_type = ?"
		args = append(args, *summaryType)
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
		return nil, fmt.Errorf("failed to list summaries: %w", err)
	}
	defer rows.Close()

	var summaries []*Summary
	for rows.Next() {
		var summary Summary
		var temperature sql.NullFloat64
		var processingTimeSeconds sql.NullFloat64
		var apiEndpoint sql.NullString
		var localModelPath sql.NullString

		err := rows.Scan(
			&summary.ID,
			&summary.TranscriptID,
			&summary.Content,
			&summary.SummaryType,
			&summary.ModelUsed,
			&temperature,
			&summary.PromptUsed,
			&processingTimeSeconds,
			&apiEndpoint,
			&localModelPath,
			&summary.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan summary: %w", err)
		}

		summary.Temperature = float64Ptr(temperature)
		summary.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
		summary.APIEndpoint = stringPtr(apiEndpoint)
		summary.LocalModelPath = stringPtr(localModelPath)

		summaries = append(summaries, &summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating summaries: %w", err)
	}

	return summaries, nil
}

// UpdateSummary updates an existing summary
func (db *DB) UpdateSummary(summary *Summary) error {
	query := `
		UPDATE summaries SET
			content = ?, temperature = ?, prompt_used = ?,
			processing_time_seconds = ?, api_endpoint = ?, local_model_path = ?
		WHERE id = ?`

	result, err := db.Exec(query,
		summary.Content,
		nullFloat64(summary.Temperature),
		summary.PromptUsed,
		nullFloat64(summary.ProcessingTimeSeconds),
		nullString(summary.APIEndpoint),
		nullString(summary.LocalModelPath),
		summary.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update summary: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("summary not found")
	}

	return nil
}

// DeleteSummary deletes a summary
func (db *DB) DeleteSummary(id int) error {
	query := "DELETE FROM summaries WHERE id = ?"

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete summary: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("summary not found")
	}

	return nil
}

// GetSummariesByDateRange retrieves summaries created within a date range
func (db *DB) GetSummariesByDateRange(start, end time.Time, limit, offset int) ([]*Summary, error) {
	query := `
		SELECT id, transcript_id, content, summary_type, model_used,
		       temperature, prompt_used, processing_time_seconds,
		       api_endpoint, local_model_path, created_at
		FROM summaries
		WHERE created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := db.Query(query, start, end, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get summaries by date range: %w", err)
	}
	defer rows.Close()

	var summaries []*Summary
	for rows.Next() {
		var summary Summary
		var temperature sql.NullFloat64
		var processingTimeSeconds sql.NullFloat64
		var apiEndpoint sql.NullString
		var localModelPath sql.NullString

		err := rows.Scan(
			&summary.ID,
			&summary.TranscriptID,
			&summary.Content,
			&summary.SummaryType,
			&summary.ModelUsed,
			&temperature,
			&summary.PromptUsed,
			&processingTimeSeconds,
			&apiEndpoint,
			&localModelPath,
			&summary.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan summary: %w", err)
		}

		summary.Temperature = float64Ptr(temperature)
		summary.ProcessingTimeSeconds = float64Ptr(processingTimeSeconds)
		summary.APIEndpoint = stringPtr(apiEndpoint)
		summary.LocalModelPath = stringPtr(localModelPath)

		summaries = append(summaries, &summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating summaries: %w", err)
	}

	return summaries, nil
}

// GetSummaryStats returns statistics about summaries
func (db *DB) GetSummaryStats() (map[string]int, error) {
	query := `
		SELECT summary_type, COUNT(*) as count
		FROM summaries
		GROUP BY summary_type
		ORDER BY count DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var summaryType string
		var count int
		if err := rows.Scan(&summaryType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan summary stat: %w", err)
		}
		stats[summaryType] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating summary stats: %w", err)
	}

	return stats, nil
}
