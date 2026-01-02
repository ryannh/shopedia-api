package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB(dbURL string) (*pgxpool.Pool, error) {
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to DB: %w", err)
	}

	err = db.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to ping DB: %w", err)
	}
	return db, nil
}

// RunMigrations - jalankan semua file .sql di folder migration
func RunMigrations(db *pgxpool.Pool, migrationPath string) error {
	ctx := context.Background()

	// Buat tabel migration_history jika belum ada
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS migration_history (
			id SERIAL PRIMARY KEY,
			filename VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migration_history table: %w", err)
	}

	// Baca semua file .sql di folder migration
	files, err := os.ReadDir(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration folder: %w", err)
	}

	// Filter dan sort file SQL
	var sqlFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	// Jalankan setiap migration yang belum dijalankan
	for _, filename := range sqlFiles {
		// Cek apakah sudah pernah dijalankan
		var exists bool
		err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM migration_history WHERE filename = $1)`,
			filename).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration history: %w", err)
		}

		if exists {
			fmt.Printf("Migration %s already applied, skipping...\n", filename)
			continue
		}

		// Baca file SQL
		filePath := filepath.Join(migrationPath, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		// Jalankan SQL
		_, err = db.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		// Catat ke migration_history
		_, err = db.Exec(ctx,
			`INSERT INTO migration_history (filename) VALUES ($1)`,
			filename)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		fmt.Printf("Migration %s applied successfully\n", filename)
	}

	return nil
}
