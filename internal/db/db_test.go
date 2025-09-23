package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDatabaseIntegration(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database
	database, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	t.Logf("Database created successfully at %s", dbPath)

	// Test creating a recording
	recording := &Recording{
		Filename:       "test_20250123_143022.wav",
		FilePath:       filepath.Join(tmpDir, "test_20250123_143022.wav"),
		FileSize:       1024,
		SampleRate:     16000,
		Channels:       1,
		BitsPerSample:  16,
		AudioFormat:    "PCM S16LE",
		RecordingMode:  "loopback",
		WithMicrophone: false,
		RecordedAt:     &time.Time{},
	}

	err = database.CreateRecording(recording)
	if err != nil {
		t.Fatalf("Failed to create recording: %v", err)
	}

	if recording.ID == 0 {
		t.Fatal("Recording ID should be set after creation")
	}

	// Test retrieving the recording
	retrieved, err := database.GetRecording(recording.ID)
	if err != nil {
		t.Fatalf("Failed to get recording: %v", err)
	}

	if retrieved.Filename != recording.Filename {
		t.Fatalf("Expected filename %s, got %s", recording.Filename, retrieved.Filename)
	}

	// Test creating a transcript
	transcript := &Transcript{
		RecordingID: recording.ID,
		Content:     "This is a test transcript.",
		ModelUsed:   "ggml-base.en",
		Language:    "en",
	}

	err = database.CreateTranscript(transcript)
	if err != nil {
		t.Fatalf("Failed to create transcript: %v", err)
	}

	// Test creating a summary
	summary := &Summary{
		TranscriptID: transcript.ID,
		Content:      "This is a test summary.",
		SummaryType:  "meeting",
		ModelUsed:    "gpt-4",
		PromptUsed:   "Test prompt",
	}

	err = database.CreateSummary(summary)
	if err != nil {
		t.Fatalf("Failed to create summary: %v", err)
	}

	// Test getting recording with details
	details, err := database.GetRecordingWithDetails(recording.ID)
	if err != nil {
		t.Fatalf("Failed to get recording with details: %v", err)
	}

	if details.Filename != recording.Filename {
		t.Fatalf("Expected filename %s, got %s", recording.Filename, details.Filename)
	}

	if details.TranscriptContent == nil || *details.TranscriptContent != transcript.Content {
		t.Fatalf("Expected transcript content %s, got %v", transcript.Content, details.TranscriptContent)
	}

	if details.SummaryContent == nil || *details.SummaryContent != summary.Content {
		t.Fatalf("Expected summary content %s, got %v", summary.Content, details.SummaryContent)
	}

	t.Logf("Database integration test passed!")
}

func TestSearchTranscripts(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database
	database, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create test recordings and transcripts
	for i := 0; i < 3; i++ {
		recording := &Recording{
			Filename:       fmt.Sprintf("test_%d.wav", i),
			FilePath:       filepath.Join(tmpDir, fmt.Sprintf("test_%d.wav", i)),
			FileSize:       1024,
			SampleRate:     16000,
			Channels:       1,
			BitsPerSample:  16,
			AudioFormat:    "PCM S16LE",
			RecordingMode:  "loopback",
			WithMicrophone: false,
		}

		err = database.CreateRecording(recording)
		if err != nil {
			t.Fatalf("Failed to create recording %d: %v", i, err)
		}

		transcript := &Transcript{
			RecordingID: recording.ID,
			Content:     fmt.Sprintf("This is test transcript number %d with searchable content.", i),
			ModelUsed:   "ggml-base.en",
			Language:    "en",
		}

		err = database.CreateTranscript(transcript)
		if err != nil {
			t.Fatalf("Failed to create transcript %d: %v", i, err)
		}
	}

	// Debug: Check what's in transcript_search table
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM transcript_search").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count transcript_search entries: %v", err)
	}
	t.Logf("Found %d entries in transcript_search table", count)

	if count > 0 {
		rows, err := database.Query("SELECT rowid, content, recording_id, transcript_id FROM transcript_search LIMIT 5")
		if err != nil {
			t.Fatalf("Failed to query transcript_search: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var rowid int
			var content string
			var recordingID, transcriptID sql.NullInt64
			err = rows.Scan(&rowid, &content, &recordingID, &transcriptID)
			if err != nil {
				t.Fatalf("Failed to scan transcript_search row: %v", err)
			}
			t.Logf("transcript_search row: rowid=%d, content=%s, recording_id=%v, transcript_id=%v", rowid, content, recordingID, transcriptID)
		}
	}

	// Test search
	results, err := database.SearchTranscripts("searchable", 10, 0)
	if err != nil {
		t.Fatalf("Failed to search transcripts: %v", err)
	}

	t.Logf("Search returned %d results", len(results))
	for i, result := range results {
		t.Logf("Result %d: filename=%s, content=%s, rank=%f", i, result.Filename, result.Content, result.Rank)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 search results, got %d", len(results))
	}

	for _, result := range results {
		if !contains(result.Content, "searchable") {
			t.Fatalf("Search result should contain 'searchable': %s", result.Content)
		}
	}

	t.Logf("Search test passed! Found %d results", len(results))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s[len(s)-len(substr):] == substr ||
			s[:len(substr)] == substr ||
			strings.Contains(s, substr))
}
