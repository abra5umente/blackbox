package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	Up      string
}

// migrations contains all database migrations in order
var migrations = []Migration{}

// DB represents the database connection and provides methods for all database operations
type DB struct {
	*sql.DB
}

// NewDB creates a new database connection with the specified database path
func NewDB(dbPath string) (*DB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable foreign keys
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := sqlDB.Exec("PRAGMA journal_mode = WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	db := &DB{sqlDB}

	// Initialize migrations
	if err := db.initializeMigrations(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// initializeMigrations sets up the migration system and applies pending migrations
func (db *DB) initializeMigrations() error {
	// Create migrations table if it doesn't exist
	if err := db.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Load migrations from files
	if err := db.loadMigrationsFromFiles(); err != nil {
		return fmt.Errorf("failed to load migrations from files: %w", err)
	}

	// Apply any pending migrations
	return db.applyPendingMigrations()
}

// createMigrationsTable creates the schema_migrations table
func (db *DB) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`
	_, err := db.Exec(query)
	return err
}

// loadMigrationsFromFiles loads all migration files from the migrations directory
func (db *DB) loadMigrationsFromFiles() error {
	migrationsDir := "migrations"

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Clear existing migrations
	migrations = migrations[:0]

	// Load each migration file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Parse version from filename (format: 001_name.sql)
		filename := entry.Name()
		if len(filename) < 4 || filename[3] != '_' {
			continue
		}

		versionStr := filename[:3]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue
		}

		// Read migration content
		content, err := os.ReadFile(filepath.Join(migrationsDir, filename))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		// Extract name from filename
		name := strings.TrimSuffix(filename[4:], ".sql")

		// Add to migrations
		migration := Migration{
			Version: version,
			Name:    name,
			Up:      string(content),
		}

		// Insert in correct order
		insertIndex := 0
		for i, existing := range migrations {
			if existing.Version < version {
				insertIndex = i + 1
			} else {
				break
			}
		}

		// Insert the migration (create a new slice to avoid corruption)
		newMigrations := make([]Migration, 0, len(migrations)+1)
		newMigrations = append(newMigrations, migrations[:insertIndex]...)
		newMigrations = append(newMigrations, migration)
		newMigrations = append(newMigrations, migrations[insertIndex:]...)
		migrations = newMigrations

	}

	return nil
}

// applyPendingMigrations applies any migrations that haven't been applied yet
func (db *DB) applyPendingMigrations() error {
	// Get list of applied migrations
	applied, err := db.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a map for quick lookup
	appliedMap := make(map[int]bool)
	for _, version := range applied {
		appliedMap[version] = true
	}

	// Apply pending migrations in order
	for _, migration := range migrations {
		if appliedMap[migration.Version] {
			// Check if migration was actually completed successfully
			if migration.Version == 1 {
				var count int
				if err := db.QueryRow("SELECT COUNT(*) FROM recordings").Scan(&count); err == nil {
					fmt.Printf("DEBUG: Migration %d (%s) already applied and schema is complete\n", migration.Version, migration.Name)
					continue
				} else {
					fmt.Printf("DEBUG: Migration %d (%s) recorded but schema incomplete, retrying\n", migration.Version, migration.Name)
					// Delete the migration record so we can try again
					_, err := db.Exec("DELETE FROM schema_migrations WHERE version = ?", migration.Version)
					if err != nil {
						return fmt.Errorf("failed to delete incomplete migration record: %w", err)
					}
				}
			} else {
				fmt.Printf("DEBUG: Migration %d (%s) already applied\n", migration.Version, migration.Name)
				continue
			}
		}

		fmt.Printf("DEBUG: Applying migration %d (%s)\n", migration.Version, migration.Name)

		if err := db.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}
	}

	return nil
}

// getAppliedMigrations returns a list of applied migration versions
func (db *DB) getAppliedMigrations() ([]int, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// applyMigration applies a single migration and records it
func (db *DB) applyMigration(migration Migration) error {
	// Execute migration in a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Split migration into statements and execute them
	statements := splitSQLStatements(migration.Up)

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute migration statement: %w (statement: %s)", err, stmt)
		}
	}

	// Record the migration as applied
	_, err = tx.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", migration.Version, migration.Name)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// splitSQLStatements splits SQL into individual statements
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)
	inComment := false
	inMultilineStatement := false
	beginDepth := 0

	for i, char := range sql {
		// Handle string literals
		if !inString && !inComment && (char == '"' || char == '\'') {
			inString = true
			stringChar = char
		} else if inString && char == stringChar {
			inString = false
		}

		// Handle comments
		if !inString && !inComment && char == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			inComment = true
		}

		// End of comment
		if inComment && char == '\n' {
			inComment = false
		}

		// If we're in a comment, skip processing
		if inComment {
			continue
		}

		// Handle BEGIN/END blocks
		upperChar := strings.ToUpper(string(char))
		if !inString && !inComment {
			if upperChar == "B" && i+4 < len(sql) && strings.ToUpper(sql[i:i+5]) == "BEGIN" {
				beginDepth++
				inMultilineStatement = true
			} else if upperChar == "E" && i+2 < len(sql) && strings.ToUpper(sql[i:i+3]) == "END" {
				beginDepth--
				if beginDepth <= 0 {
					inMultilineStatement = false
					beginDepth = 0
				}
			}
		}

		// Add character to current statement
		current.WriteRune(char)

		// Check if this ends a statement (semicolon not in a string, and not in a multiline statement)
		if char == ';' && !inString && !inMultilineStatement {
			statement := strings.TrimSpace(current.String())
			if statement != "" && statement != ";" {
				statements = append(statements, statement)
			}
			current.Reset()
		}
	}

	// Add any remaining content
	if current.Len() > 0 {
		statement := strings.TrimSpace(current.String())
		if statement != "" {
			statements = append(statements, statement)
		}
	}

	return statements
}

// GetMigrationStatus returns the current migration status
func (db *DB) GetMigrationStatus() (map[string]interface{}, error) {
	applied, err := db.getAppliedMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a map of applied versions for quick lookup
	appliedMap := make(map[int]bool)
	for _, version := range applied {
		appliedMap[version] = true
	}

	// Check which migrations are pending
	var pending []int
	for _, migration := range migrations {
		if !appliedMap[migration.Version] {
			pending = append(pending, migration.Version)
		}
	}

	status := map[string]interface{}{
		"applied": applied,
		"pending": pending,
		"total":   len(migrations),
	}

	return status, nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{tx}, nil
}

// Tx wraps sql.Tx to provide additional functionality
type Tx struct {
	*sql.Tx
}

// Recording represents a recording entity
type Recording struct {
	ID              int        `json:"id"`
	Filename        string     `json:"filename"`
	DisplayName     *string    `json:"display_name,omitempty"`
	FilePath        string     `json:"file_path"`
	FileSize        int64      `json:"file_size"`
	DurationSeconds *float64   `json:"duration_seconds,omitempty"`
	SampleRate      int        `json:"sample_rate"`
	Channels        int        `json:"channels"`
	BitsPerSample   int        `json:"bits_per_sample"`
	AudioFormat     string     `json:"audio_format"`
	RecordingMode   string     `json:"recording_mode"`
	WithMicrophone  bool       `json:"with_microphone"`
	CreatedAt       time.Time  `json:"created_at"`
	RecordedAt      *time.Time `json:"recorded_at,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
	Tags            *string    `json:"tags,omitempty"`
	AudioData       []byte     `json:"audio_data,omitempty"` // BLOB for storing actual audio data
}

// Transcript represents a transcript entity
type Transcript struct {
	ID                    int       `json:"id"`
	RecordingID           int       `json:"recording_id"`
	Content               string    `json:"content"`
	ConfidenceScore       *float64  `json:"confidence_score,omitempty"`
	ModelUsed             string    `json:"model_used"`
	Language              string    `json:"language"`
	ProcessingTimeSeconds *float64  `json:"processing_time_seconds,omitempty"`
	WhisperVersion        *string   `json:"whisper_version,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

// Summary represents a summary entity
type Summary struct {
	ID                    int       `json:"id"`
	TranscriptID          int       `json:"transcript_id"`
	Content               string    `json:"content"`
	SummaryType           string    `json:"summary_type"`
	ModelUsed             string    `json:"model_used"`
	Temperature           *float64  `json:"temperature,omitempty"`
	PromptUsed            string    `json:"prompt_used"`
	ProcessingTimeSeconds *float64  `json:"processing_time_seconds,omitempty"`
	APIEndpoint           *string   `json:"api_endpoint,omitempty"`
	LocalModelPath        *string   `json:"local_model_path,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

// ProcessingMetadata represents processing metadata
type ProcessingMetadata struct {
	ID              int        `json:"id"`
	RecordingID     *int       `json:"recording_id,omitempty"`
	TranscriptID    *int       `json:"transcript_id,omitempty"`
	SummaryID       *int       `json:"summary_id,omitempty"`
	ProcessType     string     `json:"process_type"`
	Status          string     `json:"status"`
	ModelUsed       *string    `json:"model_used,omitempty"`
	Parameters      *string    `json:"parameters,omitempty"`
	StartTime       time.Time  `json:"start_time"`
	EndTime         *time.Time `json:"end_time,omitempty"`
	DurationSeconds *float64   `json:"duration_seconds,omitempty"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
	LogFilePath     *string    `json:"log_file_path,omitempty"`
}

// Tag represents a tag entity
type Tag struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Color       *string   `json:"color,omitempty"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// RecordingWithDetails represents a recording with its transcript and summary
type RecordingWithDetails struct {
	Recording
	TranscriptID      *int       `json:"transcript_id,omitempty"`
	TranscriptContent *string    `json:"transcript_content,omitempty"`
	TranscriptModel   *string    `json:"transcript_model,omitempty"`
	ConfidenceScore   *float64   `json:"confidence_score,omitempty"`
	TranscribedAt     *time.Time `json:"transcribed_at,omitempty"`
	SummaryID         *int       `json:"summary_id,omitempty"`
	SummaryContent    *string    `json:"summary_content,omitempty"`
	SummaryType       *string    `json:"summary_type,omitempty"`
	SummaryModel      *string    `json:"summary_model,omitempty"`
	SummarizedAt      *time.Time `json:"summarized_at,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	RecordingID   int        `json:"recording_id"`
	TranscriptID  int        `json:"transcript_id"`
	Filename      string     `json:"filename"`
	Content       string     `json:"content"`
	Rank          float64    `json:"rank"`
	CreatedAt     time.Time  `json:"created_at"`
	TranscribedAt *time.Time `json:"transcribed_at,omitempty"`
}

// ProcessingStats represents processing statistics
type ProcessingStats struct {
	ProcessType string   `json:"process_type"`
	Status      string   `json:"status"`
	Count       int      `json:"count"`
	AvgDuration *float64 `json:"avg_duration,omitempty"`
	MinDuration *float64 `json:"min_duration,omitempty"`
	MaxDuration *float64 `json:"max_duration,omitempty"`
	Failures    int      `json:"failures"`
}
