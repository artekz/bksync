package ftp

import (
	"artekz/bksync/internal/synchronizer"
	"os"
	"path/filepath"
	"strings"
	"time"

	ftpServer "github.com/jlaffaye/ftp"
)

type FTPStorage struct {
	connection *ftpServer.ServerConn
}

func NewFTPStorage(connection *ftpServer.ServerConn) *FTPStorage {
	return &FTPStorage{
		connection: connection,
	}
}

func (s *FTPStorage) Store(path string, file *os.File) error {
	return s.connection.Stor(path, file)
}

func (s *FTPStorage) MakeDir(path string) error {
	// Split the path into components
	dirs := strings.Split(filepath.ToSlash(path), "/")
	currentPath := ""

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		currentPath += "/" + dir
		if !s.IsDirExists(currentPath) {
			err := s.connection.MakeDir(currentPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *FTPStorage) IsDirExists(path string) bool {
	if err := s.connection.ChangeDir(path); err != nil {
		return false
	}

	if err := s.connection.ChangeDir(".."); err != nil {
		return false
	}

	return true
}

func (s *FTPStorage) ModTime(path string) (time.Time, error) {
	if !s.connection.IsGetTimeSupported() {
		return time.Time{}, synchronizer.ErrGetTimeNotSupported
	}

	return s.connection.GetTime(path)
}

func (s *FTPStorage) IsFileExists(path string) bool {
	resp, err := s.connection.Retr(path)
	if err != nil {
		return false
	}

	resp.Close()
	return true
}

func (s *FTPStorage) List(path string) ([]*synchronizer.SyncEntry, error) {
	var syncEntries []*synchronizer.SyncEntry

	files, err := s.connection.List(path)
	if err != nil {
		return nil, err
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
	return s.connection.RemoveDirRecur(path)
}

func (s *FTPStorage) Delete(path string) error {
	return s.connection.Delete(path)
}
