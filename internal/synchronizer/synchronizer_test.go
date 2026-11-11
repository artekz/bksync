package synchronizer

import (
	"artekz/bksync/internal/dirchecker"
	"errors"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

type testStorage struct {
	files         map[string]time.Time
	fileContents  map[string]string
	directories   map[string]bool
	listings      map[string][]*SyncEntry
	modTimeErr    error
	fileExistsErr error
	dirExistsErr  error
	storeErr      error
	listErr       error
	deleteErr     error

	storedPaths  []string
	madeDirs     []string
	deletedPaths []string
	removedDirs  []string
}

func newTestStorage() *testStorage {
	return &testStorage{
		files:        make(map[string]time.Time),
		fileContents: make(map[string]string),
		directories:  make(map[string]bool),
		listings:     make(map[string][]*SyncEntry),
	}
}

func (s *testStorage) JoinPath(parent, child string) string {
	return pathpkg.Join(parent, child)
}

func (s *testStorage) Store(path string, file *os.File) error {
	if s.storeErr != nil {
		return s.storeErr
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	s.storedPaths = append(s.storedPaths, path)
	s.fileContents[path] = string(content)
	s.files[path] = time.Now()
	return nil
}

func (s *testStorage) MakeDir(path string) error {
	s.directories[path] = true
	s.madeDirs = append(s.madeDirs, path)
	return nil
}

func (s *testStorage) IsDirExists(path string) (bool, error) {
	return s.directories[path], s.dirExistsErr
}

func (s *testStorage) ModTime(path string) (time.Time, error) {
	if s.modTimeErr != nil {
		return time.Time{}, s.modTimeErr
	}
	return s.files[path], nil
}

func (s *testStorage) IsFileExists(path string) (bool, error) {
	_, exists := s.files[path]
	return exists, s.fileExistsErr
}

func (s *testStorage) List(path string) ([]*SyncEntry, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listings[path], nil
}

func (s *testStorage) RemoveDir(path string) error {
	s.removedDirs = append(s.removedDirs, path)
	return nil
}

func (s *testStorage) Delete(path string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedPaths = append(s.deletedPaths, path)
	return nil
}

func (s *testStorage) Close() error {
	return nil
}

func newTestSynchronizer(storage Storage) *Synchronizer {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return NewSynchronizer(storage, nil, logger)
}

func TestSyncDirFileDecisions(t *testing.T) {
	localModTime := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name          string
		remoteExists  bool
		remoteModTime time.Time
		wantNew       int
		wantUpdated   int
		wantSkipped   int
		wantStored    bool
	}{
		{
			name:       "new file",
			wantNew:    1,
			wantStored: true,
		},
		{
			name:          "same modification time",
			remoteExists:  true,
			remoteModTime: localModTime,
			wantSkipped:   1,
		},
		{
			name:          "remote file is newer",
			remoteExists:  true,
			remoteModTime: localModTime.Add(time.Minute),
			wantSkipped:   1,
		},
		{
			name:          "local file is newer",
			remoteExists:  true,
			remoteModTime: localModTime.Add(-time.Minute),
			wantUpdated:   1,
			wantStored:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localDir := t.TempDir()
			localPath := filepath.Join(localDir, "file.txt")
			if err := os.WriteFile(localPath, []byte("local contents"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(localPath, localModTime, localModTime); err != nil {
				t.Fatal(err)
			}

			remoteDir := t.TempDir()
			remotePath := pathpkg.Join(remoteDir, "file.txt")
			storage := newTestStorage()
			if tt.remoteExists {
				storage.files[remotePath] = tt.remoteModTime
			}

			newFiles, updatedFiles, skippedFiles, err := newTestSynchronizer(storage).SyncDir(localDir, remoteDir)
			if err != nil {
				t.Fatalf("SyncDir() error = %v", err)
			}
			if newFiles != tt.wantNew || updatedFiles != tt.wantUpdated || skippedFiles != tt.wantSkipped {
				t.Fatalf("SyncDir() counts = (%d, %d, %d), want (%d, %d, %d)",
					newFiles, updatedFiles, skippedFiles, tt.wantNew, tt.wantUpdated, tt.wantSkipped)
			}
			if gotStored := len(storage.storedPaths) == 1; gotStored != tt.wantStored {
				t.Fatalf("Store called = %t, want %t", gotStored, tt.wantStored)
			}
			if tt.wantStored && storage.fileContents[remotePath] != "local contents" {
				t.Fatalf("stored contents = %q, want %q", storage.fileContents[remotePath], "local contents")
			}
		})
	}
}

func TestSyncDirCreatesDestinationRoot(t *testing.T) {
	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}

	remoteDir := t.TempDir()
	storage := newTestStorage()
	_, _, _, err := newTestSynchronizer(storage).SyncDir(localDir, remoteDir)
	if err != nil {
		t.Fatalf("SyncDir() error = %v", err)
	}
	if len(storage.madeDirs) != 1 || storage.madeDirs[0] != remoteDir {
		t.Fatalf("created directories = %v, want [%q]", storage.madeDirs, remoteDir)
	}
	if len(storage.storedPaths) != 1 || storage.storedPaths[0] != pathpkg.Join(remoteDir, "file.txt") {
		t.Fatalf("stored paths = %v", storage.storedPaths)
	}
}

func TestSyncDirPropagatesStorageErrors(t *testing.T) {
	t.Run("file existence", func(t *testing.T) {
		wantErr := errors.New("file existence failed")
		storage := newTestStorage()
		storage.fileExistsErr = wantErr

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("contents"), 0600); err != nil {
			t.Fatal(err)
		}

		_, _, _, err := newTestSynchronizer(storage).SyncDir(localDir, t.TempDir())
		if !errors.Is(err, wantErr) {
			t.Fatalf("SyncDir() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("modification time", func(t *testing.T) {
		wantErr := errors.New("modification time failed")
		storage := newTestStorage()
		storage.modTimeErr = wantErr

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("contents"), 0600); err != nil {
			t.Fatal(err)
		}

		remoteDir := t.TempDir()
		storage.files[pathpkg.Join(remoteDir, "file.txt")] = time.Now()
		_, _, _, err := newTestSynchronizer(storage).SyncDir(localDir, remoteDir)
		if !errors.Is(err, wantErr) {
			t.Fatalf("SyncDir() error = %v, want %v", err, wantErr)
		}
		if len(storage.storedPaths) != 0 {
			t.Fatalf("Store called %d times after ModTime error, want 0", len(storage.storedPaths))
		}
	})

	t.Run("store", func(t *testing.T) {
		wantErr := errors.New("store failed")
		storage := newTestStorage()
		storage.storeErr = wantErr

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("contents"), 0600); err != nil {
			t.Fatal(err)
		}

		_, _, _, err := newTestSynchronizer(storage).SyncDir(localDir, t.TempDir())
		if !errors.Is(err, wantErr) {
			t.Fatalf("SyncDir() error = %v, want %v", err, wantErr)
		}
	})
}

func TestSyncDirUploadsWhenRemoteTimeIsUnsupported(t *testing.T) {
	localDir := t.TempDir()
	localPath := filepath.Join(localDir, "file.txt")
	if err := os.WriteFile(localPath, []byte("local contents"), 0600); err != nil {
		t.Fatal(err)
	}

	remoteDir := t.TempDir()
	remotePath := pathpkg.Join(remoteDir, "file.txt")
	storage := newTestStorage()
	storage.files[remotePath] = time.Now()
	storage.modTimeErr = ErrGetTimeNotSupported

	newFiles, updatedFiles, skippedFiles, err := newTestSynchronizer(storage).SyncDir(localDir, remoteDir)
	if err != nil {
		t.Fatalf("SyncDir() error = %v", err)
	}
	if newFiles != 0 || updatedFiles != 1 || skippedFiles != 0 {
		t.Fatalf("SyncDir() counts = (%d, %d, %d), want (0, 1, 0)",
			newFiles, updatedFiles, skippedFiles)
	}
	if len(storage.storedPaths) != 1 {
		t.Fatalf("Store called %d times, want 1", len(storage.storedPaths))
	}
}

func TestSyncDirRecursesAndSkipsUnchangedDirectory(t *testing.T) {
	localDir := t.TempDir()
	localSubdir := filepath.Join(localDir, "nested")
	if err := os.Mkdir(localSubdir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localSubdir, "file.txt"), []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}

	remoteDir := t.TempDir()
	storage := newTestStorage()
	checker := dirchecker.NewDirChecker(filepath.Join(t.TempDir(), "hashes.json"))
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	syncer := NewSynchronizer(storage, checker, logger)

	newFiles, updatedFiles, skippedFiles, err := syncer.SyncDir(localDir, remoteDir)
	if err != nil {
		t.Fatalf("first SyncDir() error = %v", err)
	}
	if newFiles != 1 || updatedFiles != 0 || skippedFiles != 0 {
		t.Fatalf("first SyncDir() counts = (%d, %d, %d), want (1, 0, 0)",
			newFiles, updatedFiles, skippedFiles)
	}

	newFiles, updatedFiles, skippedFiles, err = syncer.SyncDir(localDir, remoteDir)
	if err != nil {
		t.Fatalf("second SyncDir() error = %v", err)
	}
	if newFiles != 0 || updatedFiles != 0 || skippedFiles != 0 {
		t.Fatalf("second SyncDir() counts = (%d, %d, %d), want (0, 0, 0)",
			newFiles, updatedFiles, skippedFiles)
	}
	if len(storage.storedPaths) != 1 {
		t.Fatalf("Store called %d times, want 1", len(storage.storedPaths))
	}
}

func TestRemoveDifferencesRemovesStaleEntriesRecursively(t *testing.T) {
	localDir := t.TempDir()
	localSubdir := filepath.Join(localDir, "keep")
	if err := os.Mkdir(localSubdir, 0700); err != nil {
		t.Fatal(err)
	}

	remoteDir := pathpkg.Join(t.TempDir(), "remote")
	remoteSubdir := pathpkg.Join(remoteDir, "keep")
	storage := newTestStorage()
	storage.listings[remoteDir] = []*SyncEntry{
		{Name: "stale.txt"},
		{Name: "obsolete", IsDir: true},
		{Name: "keep", IsDir: true},
		{Name: ".metadata"},
	}
	storage.listings[remoteSubdir] = []*SyncEntry{
		{Name: "nested-stale.txt"},
	}

	removed, err := newTestSynchronizer(storage).RemoveDifferences(localDir, remoteDir)
	if err != nil {
		t.Fatalf("RemoveDifferences() error = %v", err)
	}
	if removed != 2 {
		t.Fatalf("RemoveDifferences() = %d, want 2", removed)
	}
	if len(storage.deletedPaths) != 2 ||
		storage.deletedPaths[0] != pathpkg.Join(remoteDir, "stale.txt") ||
		storage.deletedPaths[1] != pathpkg.Join(remoteSubdir, "nested-stale.txt") {
		t.Fatalf("deleted paths = %v", storage.deletedPaths)
	}
	if len(storage.removedDirs) != 1 || storage.removedDirs[0] != pathpkg.Join(remoteDir, "obsolete") {
		t.Fatalf("removed directories = %v", storage.removedDirs)
	}
}

func TestRemoveDifferencesPropagatesDeleteError(t *testing.T) {
	wantErr := errors.New("delete failed")
	localDir := t.TempDir()
	remoteDir := pathpkg.Join(t.TempDir(), "remote")
	storage := newTestStorage()
	storage.listings[remoteDir] = []*SyncEntry{{Name: "stale.txt"}}
	storage.deleteErr = wantErr

	removed, err := newTestSynchronizer(storage).RemoveDifferences(localDir, remoteDir)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RemoveDifferences() error = %v, want %v", err, wantErr)
	}
	if removed != 0 {
		t.Fatalf("RemoveDifferences() = %d, want 0", removed)
	}
}
