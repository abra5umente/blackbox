package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateRecording creates a new recording in the database
func (db *DB) CreateRecording(recording *Recording) error {
	query := `
		INSERT INTO recordings (
			filename, display_name, file_path, file_size, duration_seconds,
			sample_rate, channels, bits_per_sample, audio_format,
			recording_mode, with_microphone, recorded_at, notes, tags, audio_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		recording.Filename,
		nullString(recording.DisplayName),
		recording.FilePath,
		recording.FileSize,
		nullFloat64(recording.DurationSeconds),
		recording.SampleRate,
		recording.Channels,
		recording.BitsPerSample,
		recording.AudioFormat,
		recording.RecordingMode,
		recording.WithMicrophone,
		nullTime(recording.RecordedAt),
		nullString(recording.Notes),
		nullString(recording.Tags),
		recording.AudioData,
	)
	if err != nil {
		return fmt.Errorf("failed to create recording: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get recording ID: %w", err)
	}

	recording.ID = int(id)
	recording.CreatedAt = time.Now()
	return nil
}

// GetRecording retrieves a recording by ID
func (db *DB) GetRecording(id int) (*Recording, error) {
	query := `
		SELECT id, filename, display_name, file_path, file_size, duration_seconds,
		       sample_rate, channels, bits_per_sample, audio_format,
		       recording_mode, with_microphone, created_at, recorded_at, notes, tags, audio_data
		FROM recordings WHERE id = ?`

	var recording Recording
	var displayName, notes, tags sql.NullString
	var recordedAt sql.NullTime
	var durationSeconds sql.NullFloat64

	err := db.QueryRow(query, id).Scan(
		&recording.ID,
		&recording.Filename,
		&displayName,
		&recording.FilePath,
		&recording.FileSize,
		&durationSeconds,
		&recording.SampleRate,
		&recording.Channels,
		&recording.BitsPerSample,
		&recording.AudioFormat,
		&recording.RecordingMode,
		&recording.WithMicrophone,
		&recording.CreatedAt,
		&recordedAt,
		&notes,
		&tags,
		&recording.AudioData,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("recording not found")
		}
		return nil, fmt.Errorf("failed to get recording: %w", err)
	}

	recording.DisplayName = stringPtr(displayName)
	recording.DurationSeconds = float64Ptr(durationSeconds)
	recording.RecordedAt = timePtr(recordedAt)
	recording.Notes = stringPtr(notes)
	recording.Tags = stringPtr(tags)

	return &recording, nil
}

// GetRecordingByFilename retrieves a recording by filename
func (db *DB) GetRecordingByFilename(filename string) (*Recording, error) {
	query := `
		SELECT id, filename, display_name, file_path, file_size, duration_seconds,
		       sample_rate, channels, bits_per_sample, audio_format,
		       recording_mode, with_microphone, created_at, recorded_at, notes, tags, audio_data
		FROM recordings WHERE filename = ?`

	var recording Recording
	var displayName, notes, tags sql.NullString
	var recordedAt sql.NullTime
	var durationSeconds sql.NullFloat64

	err := db.QueryRow(query, filename).Scan(
		&recording.ID,
		&recording.Filename,
		&displayName,
		&recording.FilePath,
		&recording.FileSize,
		&durationSeconds,
		&recording.SampleRate,
		&recording.Channels,
		&recording.BitsPerSample,
		&recording.AudioFormat,
		&recording.RecordingMode,
		&recording.WithMicrophone,
		&recording.CreatedAt,
		&recordedAt,
		&notes,
		&tags,
		&recording.AudioData,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("recording not found")
		}
		return nil, fmt.Errorf("failed to get recording: %w", err)
	}

	recording.DisplayName = stringPtr(displayName)
	recording.DurationSeconds = float64Ptr(durationSeconds)
	recording.RecordedAt = timePtr(recordedAt)
	recording.Notes = stringPtr(notes)
	recording.Tags = stringPtr(tags)

	return &recording, nil
}

// ListRecordings retrieves recordings with optional filtering and pagination
func (db *DB) ListRecordings(limit, offset int, mode, tag *string) ([]*Recording, error) {
	query := `
		SELECT id, filename, display_name, file_path, file_size, duration_seconds,
		       sample_rate, channels, bits_per_sample, audio_format,
		       recording_mode, with_microphone, created_at, recorded_at, notes, tags, audio_data
		FROM recordings WHERE 1=1`

	args := []interface{}{}
	if mode != nil {
		query += " AND recording_mode = ?"
		args = append(args, *mode)
	}
	if tag != nil {
		query += " AND tags LIKE ?"
		args = append(args, "%"+*tag+"%")
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
		return nil, fmt.Errorf("failed to list recordings: %w", err)
	}
	defer rows.Close()

	var recordings []*Recording
	for rows.Next() {
		var recording Recording
		var displayName, notes, tags sql.NullString
		var recordedAt sql.NullTime
		var durationSeconds sql.NullFloat64

		err := rows.Scan(
			&recording.ID,
			&recording.Filename,
			&displayName,
			&recording.FilePath,
			&recording.FileSize,
			&durationSeconds,
			&recording.SampleRate,
			&recording.Channels,
			&recording.BitsPerSample,
			&recording.AudioFormat,
			&recording.RecordingMode,
			&recording.WithMicrophone,
			&recording.CreatedAt,
			&recordedAt,
			&notes,
			&tags,
			&recording.AudioData,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recording: %w", err)
		}

		recording.DisplayName = stringPtr(displayName)
		recording.DurationSeconds = float64Ptr(durationSeconds)
		recording.RecordedAt = timePtr(recordedAt)
		recording.Notes = stringPtr(notes)
		recording.Tags = stringPtr(tags)

		recordings = append(recordings, &recording)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recordings: %w", err)
	}

	return recordings, nil
}

// UpdateRecording updates an existing recording
func (db *DB) UpdateRecording(recording *Recording) error {
	query := `
		UPDATE recordings SET
			display_name = ?, file_size = ?, duration_seconds = ?,
			recorded_at = ?, notes = ?, tags = ?, audio_data = ?
		WHERE id = ?`

	result, err := db.Exec(query,
		nullString(recording.DisplayName),
		recording.FileSize,
		nullFloat64(recording.DurationSeconds),
		nullTime(recording.RecordedAt),
		nullString(recording.Notes),
		nullString(recording.Tags),
		recording.AudioData,
		recording.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update recording: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("recording not found")
	}

	return nil
}

// DeleteRecording deletes a recording and all its dependent data (CASCADE)
func (db *DB) DeleteRecording(id int) error {
	query := "DELETE FROM recordings WHERE id = ?"

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete recording: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("recording not found")
	}

	return nil
}

// GetRecordingWithDetails retrieves a recording with its transcript and summary
func (db *DB) GetRecordingWithDetails(id int) (*RecordingWithDetails, error) {
	query := `
		SELECT
			r.id, r.filename, r.display_name, r.file_path, r.file_size, r.duration_seconds,
			r.sample_rate, r.channels, r.bits_per_sample, r.audio_format,
			r.recording_mode, r.with_microphone, r.created_at, r.recorded_at, r.notes, r.tags, r.audio_data,
			t.id as transcript_id, t.content as transcript_content, t.model_used as transcript_model,
			t.confidence_score, t.created_at as transcribed_at,
			s.id as summary_id, s.content as summary_content, s.summary_type,
			s.model_used as summary_model, s.created_at as summarized_at
		FROM recordings r
		LEFT JOIN transcripts t ON r.id = t.recording_id
		LEFT JOIN summaries s ON t.id = s.transcript_id
		WHERE r.id = ?`

	var details RecordingWithDetails
	var displayName, notes, tags sql.NullString
	var recordedAt sql.NullTime
	var durationSeconds sql.NullFloat64
	var transcriptID, summaryID sql.NullInt64
	var transcriptContent, transcriptModel sql.NullString
	var confidenceScore sql.NullFloat64
	var transcribedAt sql.NullTime
	var summaryContent, summaryType, summaryModel sql.NullString
	var summarizedAt sql.NullTime

	err := db.QueryRow(query, id).Scan(
		&details.ID,
		&details.Filename,
		&displayName,
		&details.FilePath,
		&details.FileSize,
		&durationSeconds,
		&details.SampleRate,
		&details.Channels,
		&details.BitsPerSample,
		&details.AudioFormat,
		&details.RecordingMode,
		&details.WithMicrophone,
		&details.CreatedAt,
		&recordedAt,
		&notes,
		&tags,
		&details.AudioData,
		&transcriptID,
		&transcriptContent,
		&transcriptModel,
		&confidenceScore,
		&transcribedAt,
		&summaryID,
		&summaryContent,
		&summaryType,
		&summaryModel,
		&summarizedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("recording not found")
		}
		return nil, fmt.Errorf("failed to get recording with details: %w", err)
	}

	details.DisplayName = stringPtr(displayName)
	details.DurationSeconds = float64Ptr(durationSeconds)
	details.RecordedAt = timePtr(recordedAt)
	details.Notes = stringPtr(notes)
	details.Tags = stringPtr(tags)

	if transcriptID.Valid {
		details.TranscriptID = intPtr(int(transcriptID.Int64))
		details.TranscriptContent = stringPtr(transcriptContent)
		details.TranscriptModel = stringPtr(transcriptModel)
		details.ConfidenceScore = float64Ptr(confidenceScore)
		details.TranscribedAt = timePtr(transcribedAt)
	}

	if summaryID.Valid {
		details.SummaryID = intPtr(int(summaryID.Int64))
		details.SummaryContent = stringPtr(summaryContent)
		details.SummaryType = stringPtr(summaryType)
		details.SummaryModel = stringPtr(summaryModel)
		details.SummarizedAt = timePtr(summarizedAt)
	}

	return &details, nil
}

// Helper functions for null types
func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func stringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func float64Ptr(nf sql.NullFloat64) *float64 {
	if nf.Valid {
		return &nf.Float64
	}
	return nil
}

func timePtr(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

func intPtr(i int) *int {
	return &i
}

// nullBytes handles nullable byte slices for BLOB columns
func nullBytes(data []byte) interface{} {
	if data == nil {
		return nil
	}
	return data
}
