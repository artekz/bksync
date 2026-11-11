package dirchecker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHashChangedStoreAndReload(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	if err := os.WriteFile(filePath, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}

	hashFilePath := filepath.Join(t.TempDir(), "cache", "hashes.json")
	checker := NewDirChecker(hashFilePath)
	hash, changed, err := checker.HashChanged(root)
	if err != nil {
		t.Fatalf("HashChanged() error = %v", err)
	}
	if !changed {
		t.Fatal("HashChanged() reported a new directory as unchanged")
	}
	if err := checker.StoreHash(root, hash); err != nil {
		t.Fatalf("StoreHash() error = %v", err)
	}

	reloadedChecker := NewDirChecker(hashFilePath)
	_, changed, err = reloadedChecker.HashChanged(root)
	if err != nil {
		t.Fatalf("HashChanged() after reload error = %v", err)
	}
	if changed {
		t.Fatal("HashChanged() reported an unchanged directory as changed")
	}

	if err := os.WriteFile(filePath, []byte("changed contents"), 0600); err != nil {
		t.Fatal(err)
	}
	_, changed, err = reloadedChecker.HashChanged(root)
	if err != nil {
		t.Fatalf("HashChanged() after file update error = %v", err)
	}
	if !changed {
		t.Fatal("HashChanged() reported an updated file as unchanged")
	}
}

func TestGenerateDirHashIgnoresHiddenEntries(t *testing.T) {
	root := t.TempDir()
	visiblePath := filepath.Join(root, "visible.txt")
	if err := os.WriteFile(visiblePath, []byte("visible"), 0600); err != nil {
		t.Fatal(err)
	}

	checker := NewDirChecker(filepath.Join(t.TempDir(), "hashes.json"))
	before, err := checker.GenerateDirHash(root)
	if err != nil {
		t.Fatalf("GenerateDirHash() error = %v", err)
	}

	hiddenDir := filepath.Join(root, ".hidden")
	if err := os.Mkdir(hiddenDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "file.txt"), []byte("hidden"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden-file"), []byte("hidden"), 0600); err != nil {
		t.Fatal(err)
	}

	after, err := checker.GenerateDirHash(root)
	if err != nil {
		t.Fatalf("GenerateDirHash() after hidden changes error = %v", err)
	}
	if after != before {
		t.Fatal("hidden entries changed the directory hash")
	}
}

func TestStoreHashReplacesExistingEntry(t *testing.T) {
	hashFilePath := filepath.Join(t.TempDir(), "hashes.json")
	initial := []HashEntry{
		{Directory: "target", Hash: "old"},
		{Directory: "other", Hash: "keep"},
		{Directory: "target", Hash: "duplicate"},
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hashFilePath, data, 0600); err != nil {
		t.Fatal(err)
	}

	checker := NewDirChecker(hashFilePath)
	if err := checker.StoreHash("target", "new"); err != nil {
		t.Fatalf("StoreHash() error = %v", err)
	}

	data, err = os.ReadFile(hashFilePath)
	if err != nil {
		t.Fatal(err)
	}
	var entries []HashEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("decode stored hashes: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("stored %d entries, want 2", len(entries))
	}

	hashes := make(map[string]string, len(entries))
	for _, entry := range entries {
		hashes[entry.Directory] = entry.Hash
	}
	if hashes["target"] != "new" || hashes["other"] != "keep" {
		t.Fatalf("stored hashes = %v", hashes)
	}
}

func TestHashChangedReturnsCacheReadError(t *testing.T) {
	hashFilePath := filepath.Join(t.TempDir(), "hashes.json")
	if err := os.WriteFile(hashFilePath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}

	checker := NewDirChecker(hashFilePath)
	_, _, err := checker.HashChanged(t.TempDir())
	if err == nil {
		t.Fatal("HashChanged() error = nil, want invalid-cache error")
	}
}

func TestGenerateDirHashDetectsSubsecondModification(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	if err := os.WriteFile(filePath, []byte("same size"), 0600); err != nil {
		t.Fatal(err)
	}

	checker := NewDirChecker(filepath.Join(t.TempDir(), "hashes.json"))
	firstModTime := time.Date(2025, time.January, 1, 0, 0, 0, 100_000_000, time.UTC)
	if err := os.Chtimes(filePath, firstModTime, firstModTime); err != nil {
		t.Fatal(err)
	}
	before, err := checker.GenerateDirHash(root)
	if err != nil {
		t.Fatal(err)
	}

	modTime := firstModTime.Add(700 * time.Millisecond)
	if err := os.Chtimes(filePath, modTime, modTime); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Equal(firstModTime) {
		t.Skip("filesystem does not preserve subsecond modification times")
	}

	after, err := checker.GenerateDirHash(root)
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatal("subsecond modification changed no hash")
	}
}
