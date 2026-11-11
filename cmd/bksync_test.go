package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.json")
	content := `{
		"storage": "ftp",
		"ftp": {
			"host": "example.test",
			"port": "21",
			"user": "test-user",
			"password": "test-password"
		},
		"sync": [
			{"local": "local-dir", "remote": "/remote-dir"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	config, err := loadConfig(configDir)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if config.Storage != "ftp" {
		t.Fatalf("storage = %q, want %q", config.Storage, "ftp")
	}
	if config.FTP.Host != "example.test" || config.FTP.Port != "21" {
		t.Fatalf("FTP config = %+v", config.FTP)
	}
	if len(config.Sync) != 1 || config.Sync[0].Local != "local-dir" || config.Sync[0].Remote != "/remote-dir" {
		t.Fatalf("sync config = %+v", config.Sync)
	}
}
