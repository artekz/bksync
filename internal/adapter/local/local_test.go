package local

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestJoinPathPreservesUNCPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("UNC path semantics are Windows-specific")
	}

	storage := NewLocalStorage()
	got := storage.JoinPath("//ARTEK-TERMINAL/Dysk E/Artur", "file.docx")
	want := filepath.Join("//ARTEK-TERMINAL/Dysk E/Artur", "file.docx")

	if got != want {
		t.Fatalf("JoinPath() = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, `\\ARTEK-TERMINAL\Dysk E\Artur\`) {
		t.Fatalf("JoinPath() lost UNC prefix: %q", got)
	}
}

func TestExistsChecksDistinguishFilesDirectoriesAndMissingPaths(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, "dir")
	if err := os.Mkdir(dirPath, 0700); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, "file.txt")
	if err := os.WriteFile(filePath, []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}

	storage := NewLocalStorage()
	tests := []struct {
		name        string
		path        string
		isDirectory bool
		wantExists  bool
	}{
		{name: "existing directory", path: dirPath, isDirectory: true, wantExists: true},
		{name: "file is not directory", path: filePath, isDirectory: true},
		{name: "missing directory", path: filepath.Join(root, "missing"), isDirectory: true},
		{name: "existing file", path: filePath, wantExists: true},
		{name: "directory is not file", path: dirPath},
		{name: "missing file", path: filepath.Join(root, "missing.txt")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exists bool
			var err error
			if tt.isDirectory {
				exists, err = storage.IsDirExists(tt.path)
			} else {
				exists, err = storage.IsFileExists(tt.path)
			}
			if err != nil {
				t.Fatalf("existence check error = %v", err)
			}
			if exists != tt.wantExists {
				t.Fatalf("existence check = %t, want %t", exists, tt.wantExists)
			}
		})
	}
}
