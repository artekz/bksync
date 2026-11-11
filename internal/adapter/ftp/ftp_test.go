package ftp

import (
	"errors"
	"io"
	"net/textproto"
	"os"
	"strconv"
	"testing"
	"time"

	ftpServer "github.com/jlaffaye/ftp"
)

type testFTPConnection struct {
	fileSize     int64
	fileSizeErr  error
	listEntries  []*ftpServer.Entry
	listErr      error
	currentDir   string
	changeDirErr error
	sizePaths    []string
	listPaths    []string
	changedDirs  []string
}

func (c *testFTPConnection) Stor(string, io.Reader) error { return nil }
func (c *testFTPConnection) MakeDir(string) error         { return nil }
func (c *testFTPConnection) CurrentDir() (string, error)  { return c.currentDir, nil }
func (c *testFTPConnection) ChangeDir(path string) error {
	c.changedDirs = append(c.changedDirs, path)
	if path != c.currentDir {
		return c.changeDirErr
	}
	return nil
}
func (c *testFTPConnection) IsGetTimeSupported() bool          { return true }
func (c *testFTPConnection) GetTime(string) (time.Time, error) { return time.Time{}, nil }
func (c *testFTPConnection) FileSize(path string) (int64, error) {
	c.sizePaths = append(c.sizePaths, path)
	return c.fileSize, c.fileSizeErr
}
func (c *testFTPConnection) List(path string) ([]*ftpServer.Entry, error) {
	c.listPaths = append(c.listPaths, path)
	return c.listEntries, c.listErr
}
func (c *testFTPConnection) RemoveDirRecur(string) error { return nil }
func (c *testFTPConnection) Delete(string) error         { return nil }
func (c *testFTPConnection) Quit() error                 { return nil }

func TestNormalizeRemotePath(t *testing.T) {
	if got := normalizeRemotePath(`remote\path\file.txt`); got != "remote/path/file.txt" {
		t.Fatalf("normalizeRemotePath() = %q", got)
	}
}

func TestIsFileExistsUsesSizeAndNormalizesPath(t *testing.T) {
	connection := &testFTPConnection{fileSize: 12}
	storage := &FTPStorage{connection: connection}

	exists, err := storage.IsFileExists(`remote\folder\file.txt`)
	if err != nil {
		t.Fatalf("IsFileExists() error = %v", err)
	}
	if !exists {
		t.Fatal("IsFileExists() = false, want true")
	}
	if len(connection.sizePaths) != 1 || connection.sizePaths[0] != "remote/folder/file.txt" {
		t.Fatalf("SIZE paths = %v", connection.sizePaths)
	}
	if len(connection.listPaths) != 0 {
		t.Fatalf("LIST called for supported SIZE server: %v", connection.listPaths)
	}
}

func TestIsFileExistsUsesListingWhenSizeIsUnsupported(t *testing.T) {
	codes := []int{
		ftpServer.StatusCommandNotImplemented,
		ftpServer.StatusBadCommand,
		ftpServer.StatusNotImplemented,
		ftpServer.StatusNotImplementedParameter,
	}
	for _, code := range codes {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			connection := &testFTPConnection{
				fileSizeErr: &textproto.Error{Code: code},
				listEntries: []*ftpServer.Entry{{Name: "file.txt", Type: ftpServer.EntryTypeFile}},
			}
			storage := &FTPStorage{connection: connection}

			exists, err := storage.IsFileExists("remote/folder/file.txt")
			if err != nil {
				t.Fatalf("IsFileExists() error = %v", err)
			}
			if !exists {
				t.Fatal("IsFileExists() = false, want true")
			}
			if len(connection.listPaths) != 1 || connection.listPaths[0] != "remote/folder" {
				t.Fatalf("LIST paths = %v", connection.listPaths)
			}
		})
	}
}

func TestIsFileExistsReturnsNotExistForMissingFile(t *testing.T) {
	connection := &testFTPConnection{
		fileSizeErr: &textproto.Error{Code: ftpServer.StatusFileUnavailable},
	}
	storage := &FTPStorage{connection: connection}

	exists, err := storage.IsFileExists("missing.txt")
	if exists || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("IsFileExists() = (%t, %v), want (false, os.ErrNotExist)", exists, err)
	}
}

func TestIsDirExistsRestoresWorkingDirectory(t *testing.T) {
	connection := &testFTPConnection{currentDir: "/home"}
	storage := &FTPStorage{connection: connection}

	exists, err := storage.IsDirExists(`remote\folder`)
	if err != nil {
		t.Fatalf("IsDirExists() error = %v", err)
	}
	if !exists {
		t.Fatal("IsDirExists() = false, want true")
	}
	if len(connection.changedDirs) != 2 ||
		connection.changedDirs[0] != "remote/folder" ||
		connection.changedDirs[1] != "/home" {
		t.Fatalf("CWD sequence = %v", connection.changedDirs)
	}
}

func TestFTPPathErrorMapsMissingPaths(t *testing.T) {
	err := ftpPathError("missing.txt", &textproto.Error{
		Code: ftpServer.StatusFileUnavailable,
		Msg:  "file unavailable",
	})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ftpPathError() = %v, want os.ErrNotExist", err)
	}
}

func TestFTPPathErrorPreservesOtherErrors(t *testing.T) {
	wantErr := errors.New("connection failed")
	err := ftpPathError("file.txt", wantErr)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ftpPathError() = %v, want wrapped connection error", err)
	}
}
