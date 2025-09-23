package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"blackbox/internal/db"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Default database path
	dbPath := "./data/blackbox.db"
	if envPath := os.Getenv("BLACKBOX_DB_PATH"); envPath != "" {
		dbPath = envPath
	}

	switch command {
	case "status":
		if err := showMigrationStatus(dbPath); err != nil {
			log.Fatalf("Migration status failed: %v", err)
		}
	case "up":
		if err := runMigrations(dbPath); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
	case "create":
		if len(os.Args) < 4 {
			fmt.Println("Usage: migrate create <version> <name>")
			os.Exit(1)
		}
		version := os.Args[2]
		name := os.Args[3]
		if err := createMigrationFile(version, name); err != nil {
			log.Fatalf("Create migration failed: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Blackbox Database Migration Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  migrate status              Show current migration status")
	fmt.Println("  migrate up                  Run pending migrations")
	fmt.Println("  migrate create <version> <name>  Create a new migration file")
	fmt.Println()
	fmt.Println("Environment variables:")
	fmt.Println("  BLACKBOX_DB_PATH            Database file path (default: ./data/blackbox.db)")
}

func showMigrationStatus(dbPath string) error {
	database, err := db.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	status, err := database.GetMigrationStatus()
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	fmt.Println("Migration Status:")
	fmt.Println("================")

	applied := status["applied"].([]int)
	pending := status["pending"].([]int)
	total := status["total"].(int)

	fmt.Printf("Total migrations: %d\n", total)
	fmt.Printf("Applied: %d\n", len(applied))
	fmt.Printf("Pending: %d\n", len(pending))

	if len(applied) > 0 {
		fmt.Println("\nApplied migrations:")
		for _, version := range applied {
			fmt.Printf("  ✓ %d\n", version)
		}
	}

	if len(pending) > 0 {
		fmt.Println("\nPending migrations:")
		for _, version := range pending {
			fmt.Printf("  ✗ %d\n", version)
		}
	}

	return nil
}

func runMigrations(dbPath string) error {
	fmt.Println("Running database migrations...")

	database, err := db.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	fmt.Println("✓ Database opened successfully")

	// Get migration status before running
	status, err := database.GetMigrationStatus()
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	applied := status["applied"].([]int)
	pending := status["pending"].([]int)

	fmt.Printf("Applied: %d, Pending: %d\n", len(applied), len(pending))

	if len(pending) == 0 {
		fmt.Println("✓ All migrations are already applied")
		return nil
	}

	fmt.Printf("Applying %d pending migrations...\n", len(pending))

	// The migrations will be applied automatically by initializeMigrations
	// We just need to verify they were applied
	newStatus, err := database.GetMigrationStatus()
	if err != nil {
		return fmt.Errorf("failed to verify migration status: %w", err)
	}

	newPending := newStatus["pending"].([]int)

	if len(newPending) < len(pending) {
		fmt.Printf("✓ Successfully applied %d migrations\n", len(pending)-len(newPending))
	} else {
		return fmt.Errorf("failed to apply migrations")
	}

	return nil
}

func createMigrationFile(versionStr, name string) error {
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return fmt.Errorf("invalid version number: %w", err)
	}

	// Check if migrations directory exists
	migrationsDir := "./migrations"
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("%03d_%s.sql", version, name)
	filepath := filepath.Join(migrationsDir, filename)

	// Check if file already exists
	if _, err := os.Stat(filepath); err == nil {
		return fmt.Errorf("migration file already exists: %s", filepath)
	}

	// Create migration file with template
	template := fmt.Sprintf(`-- Migration: %s
-- Version: %d
-- Description: %s

-- Write your UP migration here
-- Example:
-- CREATE TABLE example (id INTEGER PRIMARY KEY, name TEXT);
-- INSERT INTO example (name) VALUES ('test');

`, name, version, name)

	err = os.WriteFile(filepath, []byte(template), 0644)
	if err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}

	fmt.Printf("✓ Created migration file: %s\n", filepath)
	return nil
}

// createNextMigration creates the next migration file with an auto-incremented version
func createNextMigration(name string) error {
	// Find the highest existing migration version
	migrationsDir := "./migrations"
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	maxVersion := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		filename := entry.Name()
		if len(filename) < 4 || filename[3] != '_' {
			continue
		}

		versionStr := filename[:3]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue
		}

		if version > maxVersion {
			maxVersion = version
		}
	}

	// Create next version
	nextVersion := maxVersion + 1
	return createMigrationFile(strconv.Itoa(nextVersion), name)
}
