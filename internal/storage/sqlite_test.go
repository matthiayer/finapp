package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationsAndEnablesForeignKeys(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	var tableCount int
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'app_settings'`,
	).Scan(&tableCount); err != nil {
		t.Fatalf("query migrated schema: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("app_settings table count = %d, want 1", tableCount)
	}

	var foreignKeysEnabled int
	if err := db.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&foreignKeysEnabled); err != nil {
		t.Fatalf("query foreign_keys pragma: %v", err)
	}
	if foreignKeysEnabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeysEnabled)
	}
}

func TestOpenAppliesMigrationsOnlyOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "finapp.db")

	for range 2 {
		db, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
}
