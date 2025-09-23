package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateProcessingMetadata creates a new processing metadata entry
func (db *DB) CreateProcessingMetadata(metadata *ProcessingMetadata) error {
	query := `
		INSERT INTO processing_metadata (
			recording_id, transcript_id, summary_id, process_type, status,
			model_used, parameters, start_time, end_time, duration_seconds,
			error_message, log_file_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		nullInt(metadata.RecordingID),
		nullInt(metadata.TranscriptID),
		nullInt(metadata.SummaryID),
		metadata.ProcessType,
		metadata.Status,
		nullString(metadata.ModelUsed),
		nullString(metadata.Parameters),
		metadata.StartTime,
		nullTime(metadata.EndTime),
		nullFloat64(metadata.DurationSeconds),
		nullString(metadata.ErrorMessage),
		nullString(metadata.LogFilePath),
	)
	if err != nil {
		return fmt.Errorf("failed to create processing metadata: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get processing metadata ID: %w", err)
	}

	metadata.ID = int(id)
	return nil
}

// GetProcessingMetadata retrieves processing metadata by ID
func (db *DB) GetProcessingMetadata(id int) (*ProcessingMetadata, error) {
	query := `
		SELECT id, recording_id, transcript_id, summary_id, process_type, status,
		       model_used, parameters, start_time, end_time, duration_seconds,
		       error_message, log_file_path
		FROM processing_metadata WHERE id = ?`

	var metadata ProcessingMetadata
	var recordingID sql.NullInt64
	var transcriptID sql.NullInt64
	var summaryID sql.NullInt64
	var modelUsed sql.NullString
	var parameters sql.NullString
	var endTime sql.NullTime
	var durationSeconds sql.NullFloat64
	var errorMessage sql.NullString
	var logFilePath sql.NullString

	err := db.QueryRow(query, id).Scan(
		&metadata.ID,
		&recordingID,
		&transcriptID,
		&summaryID,
		&metadata.ProcessType,
		&metadata.Status,
		&modelUsed,
		&parameters,
		&metadata.StartTime,
		&endTime,
		&durationSeconds,
		&errorMessage,
		&logFilePath,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("processing metadata not found")
		}
		return nil, fmt.Errorf("failed to get processing metadata: %w", err)
	}

	metadata.RecordingID = intPtr(int(recordingID.Int64))
	metadata.TranscriptID = intPtr(int(transcriptID.Int64))
	metadata.SummaryID = intPtr(int(summaryID.Int64))
	metadata.ModelUsed = stringPtr(modelUsed)
	metadata.Parameters = stringPtr(parameters)
	metadata.EndTime = timePtr(endTime)
	metadata.DurationSeconds = float64Ptr(durationSeconds)
	metadata.ErrorMessage = stringPtr(errorMessage)
	metadata.LogFilePath = stringPtr(logFilePath)

	return &metadata, nil
}

// ListProcessingMetadata retrieves processing metadata with optional filtering
func (db *DB) ListProcessingMetadata(limit, offset int, processType, status *string) ([]*ProcessingMetadata, error) {
	query := `
		SELECT id, recording_id, transcript_id, summary_id, process_type, status,
		       model_used, parameters, start_time, end_time, duration_seconds,
		       error_message, log_file_path
		FROM processing_metadata WHERE 1=1`

	args := []interface{}{}
	if processType != nil {
		query += " AND process_type = ?"
		args = append(args, *processType)
	}
	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}

	query += " ORDER BY start_time DESC"

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
		return nil, fmt.Errorf("failed to list processing metadata: %w", err)
	}
	defer rows.Close()

	var metadataList []*ProcessingMetadata
	for rows.Next() {
		var metadata ProcessingMetadata
		var recordingID sql.NullInt64
		var transcriptID sql.NullInt64
		var summaryID sql.NullInt64
		var modelUsed sql.NullString
		var parameters sql.NullString
		var endTime sql.NullTime
		var durationSeconds sql.NullFloat64
		var errorMessage sql.NullString
		var logFilePath sql.NullString

		err := rows.Scan(
			&metadata.ID,
			&recordingID,
			&transcriptID,
			&summaryID,
			&metadata.ProcessType,
			&metadata.Status,
			&modelUsed,
			&parameters,
			&metadata.StartTime,
			&endTime,
			&durationSeconds,
			&errorMessage,
			&logFilePath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan processing metadata: %w", err)
		}

		metadata.RecordingID = intPtr(int(recordingID.Int64))
		metadata.TranscriptID = intPtr(int(transcriptID.Int64))
		metadata.SummaryID = intPtr(int(summaryID.Int64))
		metadata.ModelUsed = stringPtr(modelUsed)
		metadata.Parameters = stringPtr(parameters)
		metadata.EndTime = timePtr(endTime)
		metadata.DurationSeconds = float64Ptr(durationSeconds)
		metadata.ErrorMessage = stringPtr(errorMessage)
		metadata.LogFilePath = stringPtr(logFilePath)

		metadataList = append(metadataList, &metadata)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating processing metadata: %w", err)
	}

	return metadataList, nil
}

// UpdateProcessingMetadata updates processing metadata
func (db *DB) UpdateProcessingMetadata(metadata *ProcessingMetadata) error {
	query := `
		UPDATE processing_metadata SET
			status = ?, end_time = ?, duration_seconds = ?,
			error_message = ?, log_file_path = ?
		WHERE id = ?`

	result, err := db.Exec(query,
		metadata.Status,
		nullTime(metadata.EndTime),
		nullFloat64(metadata.DurationSeconds),
		nullString(metadata.ErrorMessage),
		nullString(metadata.LogFilePath),
		metadata.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update processing metadata: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("processing metadata not found")
	}

	return nil
}

// CompleteProcessingMetadata marks processing as completed and sets end time and duration
func (db *DB) CompleteProcessingMetadata(id int, durationSeconds float64, logFilePath *string) error {
	endTime := time.Now()
	startTime := time.Time{}

	// Get start time
	query := "SELECT start_time FROM processing_metadata WHERE id = ?"
	err := db.QueryRow(query, id).Scan(&startTime)
	if err != nil {
		return fmt.Errorf("failed to get start time: %w", err)
	}

	// Calculate actual duration if not provided
	if durationSeconds == 0 {
		durationSeconds = endTime.Sub(startTime).Seconds()
	}

	// Update metadata
	query = `
		UPDATE processing_metadata SET
			status = 'completed', end_time = ?, duration_seconds = ?
		WHERE id = ?`

	result, err := db.Exec(query, endTime, durationSeconds, id)
	if err != nil {
		return fmt.Errorf("failed to complete processing metadata: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("processing metadata not found")
	}

	return nil
}

// FailProcessingMetadata marks processing as failed with error message
func (db *DB) FailProcessingMetadata(id int, errorMessage string) error {
	endTime := time.Now()
	startTime := time.Time{}

	// Get start time
	query := "SELECT start_time FROM processing_metadata WHERE id = ?"
	err := db.QueryRow(query, id).Scan(&startTime)
	if err != nil {
		return fmt.Errorf("failed to get start time: %w", err)
	}

	// Calculate duration
	durationSeconds := endTime.Sub(startTime).Seconds()

	// Update metadata
	query = `
		UPDATE processing_metadata SET
			status = 'failed', end_time = ?, duration_seconds = ?, error_message = ?
		WHERE id = ?`

	result, err := db.Exec(query, endTime, durationSeconds, errorMessage, id)
	if err != nil {
		return fmt.Errorf("failed to fail processing metadata: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("processing metadata not found")
	}

	return nil
}

// GetProcessingStats returns aggregated processing statistics
func (db *DB) GetProcessingStats() ([]*ProcessingStats, error) {
	query := `
		SELECT process_type, status,
		       COUNT(*) as count,
		       AVG(duration_seconds) as avg_duration,
		       MIN(duration_seconds) as min_duration,
		       MAX(duration_seconds) as max_duration
		FROM processing_metadata
		WHERE end_time IS NOT NULL
		GROUP BY process_type, status
		ORDER BY process_type, status`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get processing stats: %w", err)
	}
	defer rows.Close()

	var stats []*ProcessingStats
	for rows.Next() {
		var stat ProcessingStats
		var avgDuration sql.NullFloat64
		var minDuration sql.NullFloat64
		var maxDuration sql.NullFloat64

		err := rows.Scan(
			&stat.ProcessType,
			&stat.Status,
			&stat.Count,
			&avgDuration,
			&minDuration,
			&maxDuration,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan processing stat: %w", err)
		}

		stat.AvgDuration = float64Ptr(avgDuration)
		stat.MinDuration = float64Ptr(minDuration)
		stat.MaxDuration = float64Ptr(maxDuration)

		// Calculate failures for completed processes
		if stat.Status == "completed" {
			failureQuery := `
				SELECT COUNT(*) FROM processing_metadata
				WHERE process_type = ? AND status = 'failed'`
			var failures int
			err := db.QueryRow(failureQuery, stat.ProcessType).Scan(&failures)
			if err == nil {
				stat.Failures = failures
			}
		}

		stats = append(stats, &stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating processing stats: %w", err)
	}

	return stats, nil
}

// GetRecentProcessingMetadata gets recent processing metadata
func (db *DB) GetRecentProcessingMetadata(processType *string, limit int) ([]*ProcessingMetadata, error) {
	query := `
		SELECT id, recording_id, transcript_id, summary_id, process_type, status,
		       model_used, parameters, start_time, end_time, duration_seconds,
		       error_message, log_file_path
		FROM processing_metadata WHERE 1=1`

	args := []interface{}{}
	if processType != nil {
		query += " AND process_type = ?"
		args = append(args, *processType)
	}

	query += " ORDER BY start_time DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent processing metadata: %w", err)
	}
	defer rows.Close()

	var metadataList []*ProcessingMetadata
	for rows.Next() {
		var metadata ProcessingMetadata
		var recordingID sql.NullInt64
		var transcriptID sql.NullInt64
		var summaryID sql.NullInt64
		var modelUsed sql.NullString
		var parameters sql.NullString
		var endTime sql.NullTime
		var durationSeconds sql.NullFloat64
		var errorMessage sql.NullString
		var logFilePath sql.NullString

		err := rows.Scan(
			&metadata.ID,
			&recordingID,
			&transcriptID,
			&summaryID,
			&metadata.ProcessType,
			&metadata.Status,
			&modelUsed,
			&parameters,
			&metadata.StartTime,
			&endTime,
			&durationSeconds,
			&errorMessage,
			&logFilePath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan processing metadata: %w", err)
		}

		metadata.RecordingID = intPtr(int(recordingID.Int64))
		metadata.TranscriptID = intPtr(int(transcriptID.Int64))
		metadata.SummaryID = intPtr(int(summaryID.Int64))
		metadata.ModelUsed = stringPtr(modelUsed)
		metadata.Parameters = stringPtr(parameters)
		metadata.EndTime = timePtr(endTime)
		metadata.DurationSeconds = float64Ptr(durationSeconds)
		metadata.ErrorMessage = stringPtr(errorMessage)
		metadata.LogFilePath = stringPtr(logFilePath)

		metadataList = append(metadataList, &metadata)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating processing metadata: %w", err)
	}

	return metadataList, nil
}

// Helper functions for null types
func nullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}
