package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreatePrompt creates a new prompt in the database
func (db *DB) CreatePrompt(prompt *Prompt) error {
	query := `
		INSERT INTO prompts (
			name, display_name, description, prompt_text, is_default, is_active
		) VALUES (?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		prompt.Name,
		prompt.DisplayName,
		nullString(prompt.Description),
		prompt.PromptText,
		prompt.IsDefault,
		prompt.IsActive,
	)
	if err != nil {
		return fmt.Errorf("failed to create prompt: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get prompt ID: %w", err)
	}

	prompt.ID = int(id)
	now := time.Now()
	prompt.CreatedAt = now
	prompt.UpdatedAt = now
	return nil
}

// GetPrompt retrieves a prompt by ID
func (db *DB) GetPrompt(id int) (*Prompt, error) {
	query := `
		SELECT id, name, display_name, description, prompt_text,
		       is_default, is_active, created_at, updated_at
		FROM prompts WHERE id = ?`

	var prompt Prompt
	var description sql.NullString

	err := db.QueryRow(query, id).Scan(
		&prompt.ID,
		&prompt.Name,
		&prompt.DisplayName,
		&description,
		&prompt.PromptText,
		&prompt.IsDefault,
		&prompt.IsActive,
		&prompt.CreatedAt,
		&prompt.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("prompt not found")
		}
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	prompt.Description = stringPtr(description)
	return &prompt, nil
}

// GetPromptByName retrieves a prompt by name
func (db *DB) GetPromptByName(name string) (*Prompt, error) {
	query := `
		SELECT id, name, display_name, description, prompt_text,
		       is_default, is_active, created_at, updated_at
		FROM prompts WHERE name = ? AND is_active = TRUE`

	var prompt Prompt
	var description sql.NullString

	err := db.QueryRow(query, name).Scan(
		&prompt.ID,
		&prompt.Name,
		&prompt.DisplayName,
		&description,
		&prompt.PromptText,
		&prompt.IsDefault,
		&prompt.IsActive,
		&prompt.CreatedAt,
		&prompt.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("prompt not found")
		}
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	prompt.Description = stringPtr(description)
	return &prompt, nil
}

// ListPrompts retrieves prompts with optional filtering
func (db *DB) ListPrompts(activeOnly bool, includeDefaults bool) ([]*Prompt, error) {
	query := `
		SELECT id, name, display_name, description, prompt_text,
		       is_default, is_active, created_at, updated_at
		FROM prompts WHERE 1=1`

	args := []interface{}{}
	if activeOnly {
		query += " AND is_active = TRUE"
	}
	if !includeDefaults {
		query += " AND is_default = FALSE"
	}

	query += " ORDER BY is_default DESC, name ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}
	defer rows.Close()

	var prompts []*Prompt
	for rows.Next() {
		var prompt Prompt
		var description sql.NullString

		err := rows.Scan(
			&prompt.ID,
			&prompt.Name,
			&prompt.DisplayName,
			&description,
			&prompt.PromptText,
			&prompt.IsDefault,
			&prompt.IsActive,
			&prompt.CreatedAt,
			&prompt.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan prompt: %w", err)
		}

		prompt.Description = stringPtr(description)
		prompts = append(prompts, &prompt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating prompts: %w", err)
	}

	return prompts, nil
}

// UpdatePrompt updates an existing prompt
func (db *DB) UpdatePrompt(prompt *Prompt) error {
	query := `
		UPDATE prompts SET
			display_name = ?, description = ?, prompt_text = ?,
			is_active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	result, err := db.Exec(query,
		prompt.DisplayName,
		nullString(prompt.Description),
		prompt.PromptText,
		prompt.IsActive,
		prompt.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update prompt: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("prompt not found")
	}

	// Update the UpdatedAt timestamp
	prompt.UpdatedAt = time.Now()
	return nil
}

// DeletePrompt deletes a prompt (only if it's not a default prompt)
func (db *DB) DeletePrompt(id int) error {
	// First check if it's a default prompt
	var isDefault bool
	err := db.QueryRow("SELECT is_default FROM prompts WHERE id = ?", id).Scan(&isDefault)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("prompt not found")
		}
		return fmt.Errorf("failed to check prompt: %w", err)
	}

	if isDefault {
		return fmt.Errorf("cannot delete default prompts")
	}

	query := "DELETE FROM prompts WHERE id = ?"
	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete prompt: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("prompt not found")
	}

	return nil
}

// GetActivePrompts returns only active prompts for UI selection
func (db *DB) GetActivePrompts() ([]*Prompt, error) {
	return db.ListPrompts(true, true)
}

// PromptExists checks if a prompt with the given name exists
func (db *DB) PromptExists(name string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM prompts WHERE name = ?", name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check prompt existence: %w", err)
	}
	return count > 0, nil
}
