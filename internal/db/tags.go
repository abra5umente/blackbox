package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateTag creates a new tag in the database
func (db *DB) CreateTag(tag *Tag) error {
	query := `
		INSERT INTO tags (name, color, description)
		VALUES (?, ?, ?)`

	result, err := db.Exec(query,
		tag.Name,
		nullString(tag.Color),
		nullString(tag.Description),
	)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get tag ID: %w", err)
	}

	tag.ID = int(id)
	tag.CreatedAt = time.Now()
	return nil
}

// GetTag retrieves a tag by ID
func (db *DB) GetTag(id int) (*Tag, error) {
	query := `
		SELECT id, name, color, description, created_at
		FROM tags WHERE id = ?`

	var tag Tag
	var color sql.NullString
	var description sql.NullString

	err := db.QueryRow(query, id).Scan(
		&tag.ID,
		&tag.Name,
		&color,
		&description,
		&tag.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tag not found")
		}
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}

	tag.Color = stringPtr(color)
	tag.Description = stringPtr(description)

	return &tag, nil
}

// GetTagByName retrieves a tag by name
func (db *DB) GetTagByName(name string) (*Tag, error) {
	query := `
		SELECT id, name, color, description, created_at
		FROM tags WHERE name = ?`

	var tag Tag
	var color sql.NullString
	var description sql.NullString

	err := db.QueryRow(query, name).Scan(
		&tag.ID,
		&tag.Name,
		&color,
		&description,
		&tag.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tag not found")
		}
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}

	tag.Color = stringPtr(color)
	tag.Description = stringPtr(description)

	return &tag, nil
}

// ListTags retrieves all tags
func (db *DB) ListTags() ([]*Tag, error) {
	query := `
		SELECT id, name, color, description, created_at
		FROM tags
		ORDER BY name ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		var tag Tag
		var color sql.NullString
		var description sql.NullString

		err := rows.Scan(
			&tag.ID,
			&tag.Name,
			&color,
			&description,
			&tag.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}

		tag.Color = stringPtr(color)
		tag.Description = stringPtr(description)

		tags = append(tags, &tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return tags, nil
}

// UpdateTag updates an existing tag
func (db *DB) UpdateTag(tag *Tag) error {
	query := `
		UPDATE tags SET
			name = ?, color = ?, description = ?
		WHERE id = ?`

	result, err := db.Exec(query,
		tag.Name,
		nullString(tag.Color),
		nullString(tag.Description),
		tag.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update tag: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tag not found")
	}

	return nil
}

// DeleteTag deletes a tag
func (db *DB) DeleteTag(id int) error {
	query := "DELETE FROM tags WHERE id = ?"

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tag not found")
	}

	return nil
}

// AddTagToRecording adds a tag to a recording
func (db *DB) AddTagToRecording(recordingID, tagID int) error {
	query := `
		INSERT OR IGNORE INTO recording_tags (recording_id, tag_id)
		VALUES (?, ?)`

	_, err := db.Exec(query, recordingID, tagID)
	if err != nil {
		return fmt.Errorf("failed to add tag to recording: %w", err)
	}

	return nil
}

// RemoveTagFromRecording removes a tag from a recording
func (db *DB) RemoveTagFromRecording(recordingID, tagID int) error {
	query := `
		DELETE FROM recording_tags
		WHERE recording_id = ? AND tag_id = ?`

	result, err := db.Exec(query, recordingID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from recording: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tag not found on recording")
	}

	return nil
}

// GetRecordingTags retrieves all tags for a recording
func (db *DB) GetRecordingTags(recordingID int) ([]*Tag, error) {
	query := `
		SELECT t.id, t.name, t.color, t.description, t.created_at
		FROM tags t
		INNER JOIN recording_tags rt ON t.id = rt.tag_id
		WHERE rt.recording_id = ?
		ORDER BY t.name ASC`

	rows, err := db.Query(query, recordingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recording tags: %w", err)
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		var tag Tag
		var color sql.NullString
		var description sql.NullString

		err := rows.Scan(
			&tag.ID,
			&tag.Name,
			&color,
			&description,
			&tag.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}

		tag.Color = stringPtr(color)
		tag.Description = stringPtr(description)

		tags = append(tags, &tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return tags, nil
}

// GetRecordingsByTag retrieves all recordings with a specific tag
func (db *DB) GetRecordingsByTag(tagID int, limit, offset int) ([]*Recording, error) {
	query := `
		SELECT r.id, r.filename, r.display_name, r.file_path, r.file_size, r.duration_seconds,
		       r.sample_rate, r.channels, r.bits_per_sample, r.audio_format,
		       r.recording_mode, r.with_microphone, r.created_at, r.recorded_at, r.notes, r.tags
		FROM recordings r
		INNER JOIN recording_tags rt ON r.id = rt.recording_id
		WHERE rt.tag_id = ?
		ORDER BY r.created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := db.Query(query, tagID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get recordings by tag: %w", err)
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

// UpdateRecordingTags updates all tags for a recording
func (db *DB) UpdateRecordingTags(recordingID int, tagIDs []int) error {
	// Start a transaction
	tx, err := db.BeginTx(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove all existing tags
	_, err = tx.Exec("DELETE FROM recording_tags WHERE recording_id = ?", recordingID)
	if err != nil {
		return fmt.Errorf("failed to remove existing tags: %w", err)
	}

	// Add new tags
	for _, tagID := range tagIDs {
		_, err = tx.Exec(
			"INSERT INTO recording_tags (recording_id, tag_id) VALUES (?, ?)",
			recordingID, tagID,
		)
		if err != nil {
			return fmt.Errorf("failed to add tag %d: %w", tagID, err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
