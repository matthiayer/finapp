package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FINAPP_LISTEN_ADDR", "")
	t.Setenv("FINAPP_DB_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddress != "127.0.0.1:8080" {
		t.Errorf("ListenAddress = %q, want %q", cfg.ListenAddress, "127.0.0.1:8080")
	}
	if cfg.DatabasePath != "data/finapp.db" {
		t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, "data/finapp.db")
	}
}

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("FINAPP_LISTEN_ADDR", "192.168.68.71:8080")
	t.Setenv("FINAPP_DB_PATH", "/var/lib/finapp/finapp.db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddress != "192.168.68.71:8080" {
		t.Errorf("ListenAddress = %q, want configured address", cfg.ListenAddress)
	}
	if cfg.DatabasePath != "/var/lib/finapp/finapp.db" {
		t.Errorf("DatabasePath = %q, want configured path", cfg.DatabasePath)
	}
}

func TestLoadRejectsInvalidListenAddress(t *testing.T) {
	t.Setenv("FINAPP_LISTEN_ADDR", ":8080")
	t.Setenv("FINAPP_DB_PATH", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an empty listen host")
	}
}
