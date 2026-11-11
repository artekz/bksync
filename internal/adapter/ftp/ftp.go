package ftp

import (
	"artekz/bksync/internal/synchronizer"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"os"
	pathpkg "path"
	"strings"
	"time"

	ftpServer "github.com/jlaffaye/ftp"
)

type FTPStorage struct {
	connection ftpConnection
}

type ftpConnection interface {
	Stor(path string, r io.Reader) error
	MakeDir(path string) error
	CurrentDir() (string, error)
	ChangeDir(path string) error
	IsGetTimeSupported() bool
	GetTime(path string) (time.Time, error)
	FileSize(path string) (int64, error)
	List(path string) ([]*ftpServer.Entry, error)
	RemoveDirRecur(path string) error
	Delete(path string) error
	Quit() error
}

func NewFTPStorage(connection *ftpServer.ServerConn) *FTPStorage {
	return &FTPStorage{
		connection: connection,
	}
}

func (s *FTPStorage) JoinPath(parent, child string) string {
	return pathpkg.Join(normalizeRemotePath(parent), child)
}

func (s *FTPStorage) Store(path string, file *os.File) error {
	return s.connection.Stor(normalizeRemotePath(path), file)
}

func (s *FTPStorage) MakeDir(path string) error {
	path = normalizeRemotePath(path)
	dirs := strings.Split(strings.Trim(path, "/"), "/")
	currentPath := ""
	if strings.HasPrefix(path, "/") {
		currentPath = "/"
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if currentPath == "" {
			currentPath = dir
		} else if currentPath == "/" {
			currentPath += dir
		} else {
			currentPath += "/" + dir
		}

		exists, err := s.IsDirExists(currentPath)
		if errors.Is(err, os.ErrNotExist) {
			exists, err = false, nil
		}
		if err != nil {
			return err
		}
		if !exists {
			if err := s.connection.MakeDir(currentPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *FTPStorage) IsDirExists(path string) (bool, error) {
	path = normalizeRemotePath(path)
	currentDirectory, err := s.connection.CurrentDir()
	if err != nil {
		return false, fmt.Errorf("get FTP current directory: %w", err)
	}
	if err := s.connection.ChangeDir(path); err != nil {
		return false, ftpPathError(path, err)
	}
	if err := s.connection.ChangeDir(currentDirectory); err != nil {
		return false, fmt.Errorf("restore FTP current directory %q: %w", currentDirectory, err)
	}
	return true, nil
}

func (s *FTPStorage) ModTime(path string) (time.Time, error) {
	if !s.connection.IsGetTimeSupported() {
		return time.Time{}, synchronizer.ErrGetTimeNotSupported
	}

	return s.connection.GetTime(normalizeRemotePath(path))
}

func (s *FTPStorage) IsFileExists(path string) (bool, error) {
	path = normalizeRemotePath(path)
	_, err := s.connection.FileSize(path)
	if err == nil {
		return true, nil
	}

	var responseError *textproto.Error
	if errors.As(err, &responseError) &&
		(responseError.Code == ftpServer.StatusCommandNotImplemented ||
			responseError.Code == ftpServer.StatusBadCommand ||
			responseError.Code == ftpServer.StatusNotImplemented ||
			responseError.Code == ftpServer.StatusNotImplementedParameter) {
		return s.fileExistsInListing(path)
	}

	return false, ftpPathError(path, err)
}

func (s *FTPStorage) fileExistsInListing(path string) (bool, error) {
	parent := pathpkg.Dir(path)
	name := pathpkg.Base(path)
	entries, err := s.List(parent)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.Name == name && !entry.IsDir {
			return true, nil
		}
	}

	return false, fmt.Errorf("%s: %w", path, os.ErrNotExist)
}

func (s *FTPStorage) List(path string) ([]*synchronizer.SyncEntry, error) {
	var syncEntries []*synchronizer.SyncEntry

	path = normalizeRemotePath(path)
	files, err := s.connection.List(path)
	if err != nil {
		return nil, ftpPathError(path, err)
	}

	for _, file := range files {
		syncEntries = append(syncEntries, &synchronizer.SyncEntry{
			IsDir: file.Type == ftpServer.EntryTypeFolder,
			Name:  file.Name,
		})
	}

	return syncEntries, nil
}

func (s *FTPStorage) RemoveDir(path string) error {
	return s.connection.RemoveDirRecur(normalizeRemotePath(path))
}

func (s *FTPStorage) Delete(path string) error {
	return s.connection.Delete(normalizeRemotePath(path))
}

func (s *FTPStorage) Close() error {
	return s.connection.Quit()
}

func normalizeRemotePath(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}

func ftpPathError(path string, err error) error {
	var responseError *textproto.Error
	if errors.As(err, &responseError) && responseError.Code == ftpServer.StatusFileUnavailable {
		return fmt.Errorf("%s: %w", path, os.ErrNotExist)
	}
	return fmt.Errorf("%s: %w", path, err)
}

var _ synchronizer.Storage = (*FTPStorage)(nil)
