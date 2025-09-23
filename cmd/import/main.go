package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blackbox/internal/db"
)

type ImportConfig struct {
	DatabasePath   string `json:"database_path"`
	ImportDir      string `json:"import_dir"`
	DryRun         bool   `json:"dry_run"`
	Verbose        bool   `json:"verbose"`
	BatchSize      int    `json:"batch_size"`
	AutoDetectMode bool   `json:"auto_detect_mode"`
	DefaultMode    string `json:"default_mode"`
}

type ImportStats struct {
	RecordingsProcessed int      `json:"recordings_processed"`
	TranscriptsImported int      `json:"transcripts_imported"`
	SummariesImported   int      `json:"summaries_imported"`
	Errors              []string `json:"errors"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "config":
		if err := createConfigFile(); err != nil {
			log.Fatalf("Failed to create config: %v", err)
		}
	case "run":
		if err := runImport(); err != nil {
			log.Fatalf("Import failed: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Blackbox Data Import Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  import config    Create default import configuration")
	fmt.Println("  import run       Run the import process")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  The import tool reads config/import.json for settings")
	fmt.Println("  Run 'import config' to create a default configuration file")
}

func createConfigFile() error {
	configDir := "./config"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	config := ImportConfig{
		DatabasePath:   "./data/blackbox.db",
		ImportDir:      "./out",
		DryRun:         false,
		Verbose:        true,
		BatchSize:      100,
		AutoDetectMode: true,
		DefaultMode:    "loopback",
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(configDir, "import.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Created import configuration: %s\n", configPath)
	fmt.Println("Edit this file to customize import settings before running 'import run'")
	return nil
}

// WAVHeader represents the RIFF WAVE header structure
type WAVHeader struct {
	ChunkID       [4]byte // "RIFF"
	ChunkSize     uint32
	Format        [4]byte // "WAVE"
	Subchunk1ID   [4]byte // "fmt "
	Subchunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Subchunk2ID   [4]byte // "data"
	Subchunk2Size uint32
}

// extractWAVMetadata extracts metadata from a WAV file
func extractWAVMetadata(filePath string) (*WAVHeader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV file: %w", err)
	}
	defer file.Close()

	var header WAVHeader
	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read WAV header: %w", err)
	}

	return &header, nil
}

// detectRecordingMode attempts to detect recording mode from content
func detectRecordingMode(content string) string {
	content = strings.ToLower(content)

	// Check for meeting indicators
	meetingIndicators := []string{
		"meeting", "call", "discussion", "conference", "team",
		"participants", "agenda", "minutes", "attendees", "speaker",
		"everyone", "thanks", "goodbye", "bye", "next time",
	}

	meetingScore := 0
	for _, indicator := range meetingIndicators {
		if strings.Contains(content, indicator) {
			meetingScore++
		}
	}

	// Check for dictation indicators
	dictationIndicators := []string{
		"note", "reminder", "to do", "todo", "task", "remember",
		"personal", "myself", "i need to", "i should", "i will",
		"i have to", "i'm going to", "i plan to", "i think",
		"let me", "i'll", "i'd like to",
	}

	dictationScore := 0
	for _, indicator := range dictationIndicators {
		if strings.Contains(content, indicator) {
			dictationScore++
		}
	}

	// Check for technical indicators
	technicalIndicators := []string{
		"code", "function", "variable", "class", "method", "api",
		"database", "server", "client", "request", "response",
		"error", "bug", "fix", "implement", "deploy", "build",
		"test", "debug", "log", "configuration", "settings",
		"parameter", "argument", "return", "value", "type",
		"interface", "struct", "object", "array", "string",
		"number", "boolean", "null", "undefined", "exception",
	}

	technicalScore := 0
	for _, indicator := range technicalIndicators {
		if strings.Contains(content, indicator) {
			technicalScore++
		}
	}

	// Determine the mode with highest score
	maxScore := 0
	mode := "loopback"

	if meetingScore > maxScore {
		maxScore = meetingScore
		mode = "mixed" // Meeting content often has multiple speakers
	}

	if dictationScore > maxScore {
		maxScore = dictationScore
		mode = "dictation"
	}

	if technicalScore > maxScore {
		maxScore = technicalScore
		mode = "mixed" // Technical discussions can have multiple speakers
	}

	return mode
}

// detectModelFromContent attempts to detect the whisper model used from transcript content
func detectModelFromContent(content string) string {
	content = strings.ToLower(content)

	// Look for common whisper model names in content or log files
	modelIndicators := map[string]string{
		"ggml-base":   "ggml-base.en",
		"base":        "ggml-base.en",
		"tiny":        "ggml-tiny.en",
		"small":       "ggml-small.en",
		"medium":      "ggml-medium.en",
		"large":       "ggml-large-v3",
		"ggml-tiny":   "ggml-tiny.en",
		"ggml-small":  "ggml-small.en",
		"ggml-medium": "ggml-medium.en",
		"ggml-large":  "ggml-large-v3",
		"whisper-1":   "whisper-1",
		"turbo":       "whisper-1",
	}

	for indicator, model := range modelIndicators {
		if strings.Contains(content, indicator) {
			return model
		}
	}

	// Default to base model for imported files
	return "ggml-base.en"
}

// detectSummaryType attempts to detect the summary type from content
func detectSummaryType(content string) string {
	content = strings.ToLower(content)

	// Check for meeting indicators
	meetingIndicators := []string{
		"meeting", "call", "discussion", "conference", "team",
		"participants", "agenda", "minutes", "attendees", "decisions",
		"action items", "next steps", "follow-up",
	}

	for _, indicator := range meetingIndicators {
		if strings.Contains(content, indicator) {
			return "meeting"
		}
	}

	// Check for dictation indicators
	dictationIndicators := []string{
		"note", "reminder", "to do", "todo", "task", "personal",
		"dictation", "notes", "myself", "remember",
	}

	for _, indicator := range dictationIndicators {
		if strings.Contains(content, indicator) {
			return "dictation"
		}
	}

	// Check for technical indicators
	technicalIndicators := []string{
		"technical", "code", "function", "variable", "class", "method",
		"api", "database", "server", "client", "implementation",
		"debug", "error", "fix", "deploy", "build", "test",
	}

	for _, indicator := range technicalIndicators {
		if strings.Contains(content, indicator) {
			return "technical"
		}
	}

	// Default to general summary
	return "general"
}

func runImport() error {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Verbose {
		fmt.Printf("Starting import from %s to %s\n", config.ImportDir, config.DatabasePath)
		if config.DryRun {
			fmt.Println("DRY RUN - No data will be imported")
		}
		if config.AutoDetectMode {
			fmt.Println("Auto-detecting recording modes from content")
		}
	}

	// Open database
	database, err := db.NewDB(config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Collect files to import
	wavFiles, err := findWavFiles(config.ImportDir)
	if err != nil {
		return fmt.Errorf("failed to find WAV files: %w", err)
	}

	if config.Verbose {
		fmt.Printf("Found %d WAV files to process\n", len(wavFiles))
	}

	stats := &ImportStats{
		Errors: make([]string, 0),
	}

	// Process each WAV file
	for _, wavFile := range wavFiles {
		if err := processWavFile(database, wavFile, config, stats); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("Error processing %s: %v", wavFile, err))
			if config.Verbose {
				fmt.Printf("✗ Error processing %s: %v\n", wavFile, err)
			}
		}
	}

	// Print summary
	printImportSummary(stats)

	if len(stats.Errors) > 0 {
		return fmt.Errorf("import completed with %d errors", len(stats.Errors))
	}

	return nil
}

func loadConfig() (*ImportConfig, error) {
	configPath := "./config/import.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s (run 'import config' first)", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ImportConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func findWavFiles(dir string) ([]string, error) {
	var wavFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			wavFiles = append(wavFiles, path)
		}

		return nil
	})

	return wavFiles, err
}

func processWavFile(database *db.DB, wavPath string, config *ImportConfig, stats *ImportStats) error {
	if config.Verbose {
		fmt.Printf("Processing: %s\n", wavPath)
	}

	// Extract metadata from filename (format: YYYYMMDD_HHMMSS.wav)
	filename := filepath.Base(wavPath)
	baseName := strings.TrimSuffix(filename, ".wav")

	// Try to parse timestamp from filename
	var recordedAt *time.Time
	if timestamp, err := parseTimestampFromFilename(baseName); err == nil {
		recordedAt = &timestamp
	} else if config.Verbose {
		fmt.Printf("  Warning: Could not parse timestamp from filename: %v\n", err)
	}

	// Get file info
	fileInfo, err := os.Stat(wavPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Extract audio metadata from WAV header
	header, err := extractWAVMetadata(wavPath)
	if err != nil {
		if config.Verbose {
			fmt.Printf("  Warning: Could not extract WAV metadata: %v\n", err)
		}
		// Use defaults if header extraction fails
		header = &WAVHeader{
			SampleRate:    16000,
			NumChannels:   1,
			BitsPerSample: 16,
		}
	}

	// Calculate duration based on file size and audio format
	dataSize := fileInfo.Size() - 44 // Subtract header size
	durationSeconds := float64(dataSize) / (float64(header.SampleRate) * float64(header.NumChannels) * float64(header.BitsPerSample) / 8)

	// Determine recording mode
	recordingMode := config.DefaultMode
	if config.AutoDetectMode {
		// Look for transcript to analyze content
		txtPath := strings.TrimSuffix(wavPath, ".wav") + ".txt"
		if transcriptContent, err := os.ReadFile(txtPath); err == nil {
			detectedMode := detectRecordingMode(string(transcriptContent))
			if config.Verbose {
				fmt.Printf("  Detected mode: %s (auto-detection)\n", detectedMode)
			}
			recordingMode = detectedMode
		} else if config.Verbose {
			fmt.Printf("  Using default mode: %s (no transcript found for analysis)\n", recordingMode)
		}
	}

	// Determine if microphone was used (this is a guess based on mode)
	withMicrophone := recordingMode != "loopback"

	// Read the audio data
	audioData, err := os.ReadFile(wavPath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	// Create recording entry
	recording := &db.Recording{
		Filename:        filename,
		FilePath:        wavPath,
		FileSize:        fileInfo.Size(),
		DurationSeconds: &durationSeconds,
		SampleRate:      int(header.SampleRate),
		Channels:        int(header.NumChannels),
		BitsPerSample:   int(header.BitsPerSample),
		AudioFormat:     "PCM S16LE", // Assuming S16LE format
		RecordingMode:   recordingMode,
		WithMicrophone:  withMicrophone,
		RecordedAt:      recordedAt,
		CreatedAt:       fileInfo.ModTime(),
		AudioData:       audioData,
	}

	if !config.DryRun {
		// Check if recording already exists
		existing, err := database.GetRecordingByFilename(recording.Filename)
		if err == nil {
			// Recording exists, update it instead of creating a new one
			recording.ID = existing.ID
			if err := database.UpdateRecording(recording); err != nil {
				return fmt.Errorf("failed to update existing recording: %w", err)
			}
			if config.Verbose {
				fmt.Printf("  Updated existing recording\n")
			}
		} else if strings.Contains(err.Error(), "recording not found") {
			// Recording doesn't exist, create it
			if err := database.CreateRecording(recording); err != nil {
				return fmt.Errorf("failed to create recording: %w", err)
			}
			if config.Verbose {
				fmt.Printf("  Created new recording\n")
			}
		} else {
			// Some other error occurred
			return fmt.Errorf("failed to check existing recording: %w", err)
		}
		stats.RecordingsProcessed++
	}

	// Look for transcript file
	txtPath := strings.TrimSuffix(wavPath, ".wav") + ".txt"
	if _, err := os.Stat(txtPath); err == nil {
		if err := processTranscriptFile(database, txtPath, recording.ID, config, stats); err != nil {
			if config.Verbose {
				fmt.Printf("  Warning: failed to process transcript: %v\n", err)
			}
		}
	}

	return nil
}

func processTranscriptFile(database *db.DB, txtPath string, recordingID int, config *ImportConfig, stats *ImportStats) error {
	content, err := os.ReadFile(txtPath)
	if err != nil {
		return fmt.Errorf("failed to read transcript file: %w", err)
	}

	// Try to detect model from content or filename
	modelUsed := detectModelFromContent(string(content))

	// Estimate processing time based on transcript length (rough estimate: ~10x real-time for whisper)
	words := strings.Fields(string(content))
	estimatedProcessingTime := float64(len(words)) / 50.0 // Assume ~50 words per second processing

	// Create transcript entry
	transcript := &db.Transcript{
		RecordingID:           recordingID,
		Content:               string(content),
		ModelUsed:             modelUsed,
		Language:              "en",
		ProcessingTimeSeconds: &estimatedProcessingTime,
		CreatedAt:             getFileModTime(txtPath),
	}

	if !config.DryRun {
		// Check if transcript already exists for this recording
		existing, err := database.GetTranscriptByRecordingID(recordingID)
		if err == nil {
			// Transcript exists, update it
			transcript.ID = existing.ID
			if err := database.UpdateTranscript(transcript); err != nil {
				return fmt.Errorf("failed to update existing transcript: %w", err)
			}
			if config.Verbose {
				fmt.Printf("    Updated existing transcript\n")
			}
		} else if strings.Contains(err.Error(), "transcript not found") {
			// Transcript doesn't exist, create it
			if err := database.CreateTranscript(transcript); err != nil {
				return fmt.Errorf("failed to create transcript: %w", err)
			}
			if config.Verbose {
				fmt.Printf("    Created new transcript\n")
			}
		} else {
			// Some other error occurred
			return fmt.Errorf("failed to check existing transcript: %w", err)
		}
		stats.TranscriptsImported++
	}

	// Look for summary file
	summaryPath := strings.TrimSuffix(txtPath, ".txt") + "_summary.txt"
	if _, err := os.Stat(summaryPath); err == nil {
		if err := processSummaryFile(database, summaryPath, transcript.ID, config, stats); err != nil {
			if config.Verbose {
				fmt.Printf("    Warning: failed to process summary: %v\n", err)
			}
		}
	}

	return nil
}

func processSummaryFile(database *db.DB, summaryPath string, transcriptID int, config *ImportConfig, stats *ImportStats) error {
	content, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("failed to read summary file: %w", err)
	}

	// Detect summary type based on content
	summaryType := detectSummaryType(string(content))

	// Try to detect model from summary content
	modelUsed := detectModelFromContent(string(content))

	// Create summary entry
	summary := &db.Summary{
		TranscriptID: transcriptID,
		Content:      string(content),
		SummaryType:  summaryType,
		ModelUsed:    modelUsed,
		PromptUsed:   "imported",
		CreatedAt:    getFileModTime(summaryPath),
	}

	if !config.DryRun {
		// Check if summary already exists for this transcript
		existing, err := database.GetSummaryByTranscriptID(transcriptID)
		if err == nil {
			// Summary exists, update it
			summary.ID = existing.ID
			if err := database.UpdateSummary(summary); err != nil {
				return fmt.Errorf("failed to update existing summary: %w", err)
			}
			if config.Verbose {
				fmt.Printf("      Updated existing summary\n")
			}
		} else if strings.Contains(err.Error(), "summary not found") {
			// Summary doesn't exist, create it
			if err := database.CreateSummary(summary); err != nil {
				return fmt.Errorf("failed to create summary: %w", err)
			}
			if config.Verbose {
				fmt.Printf("      Created new summary\n")
			}
		} else {
			// Some other error occurred
			return fmt.Errorf("failed to check existing summary: %w", err)
		}
		stats.SummariesImported++
	}

	return nil
}

func parseTimestampFromFilename(filename string) (time.Time, error) {
	// Expected format: YYYYMMDD_HHMMSS
	if len(filename) != 15 || filename[8] != '_' {
		return time.Time{}, fmt.Errorf("invalid filename format")
	}

	// Parse date part: YYYYMMDD
	dateStr := filename[:8]
	date, err := time.Parse("20060102", dateStr)
	if err != nil {
		return time.Time{}, err
	}

	// Parse time part: HHMMSS
	timeStr := filename[9:15]
	t, err := time.Parse("150405", timeStr)
	if err != nil {
		return time.Time{}, err
	}

	// Combine date and time
	result := time.Date(date.Year(), date.Month(), date.Day(),
		t.Hour(), t.Minute(), t.Second(), 0, time.UTC)

	return result, nil
}

func getFileModTime(path string) time.Time {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Now()
}

func printImportSummary(stats *ImportStats) {
	fmt.Println("\nImport Summary:")
	fmt.Println("==============")
	fmt.Printf("Recordings processed: %d\n", stats.RecordingsProcessed)
	fmt.Printf("Transcripts imported: %d\n", stats.TranscriptsImported)
	fmt.Printf("Summaries imported: %d\n", stats.SummariesImported)

	if len(stats.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(stats.Errors))
		for _, err := range stats.Errors {
			fmt.Printf("  - %s\n", err)
		}
	} else {
		fmt.Println("✓ Import completed successfully!")
	}
}
