package local

import (
	"artekz/bksync/internal/synchronizer"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

type LocalStorage struct {
}

func NewLocalStorage() *LocalStorage {
	return &LocalStorage{}
}

func (s *LocalStorage) JoinPath(parent, child string) string {
	return filepath.Join(parent, child)
}

func (s *LocalStorage) Store(path string, file *os.File) error {
	// Create or open the destination file
	destFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy the contents of the source file to the destination file
	_, err = io.Copy(destFile, file)
	if err != nil {
		return err
	}

	return nil
}

func (s *LocalStorage) MakeDir(path string) error {
	// Create a directory at the specified path
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (s *LocalStorage) IsDirExists(path string) (bool, error) {
	return s.pathExists(path, true)
}

func (s *LocalStorage) pathExists(path string, wantDirectory bool) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir() == wantDirectory, nil
}

func (s *LocalStorage) ModTime(path string) (time.Time, error) {
	// Get the modification time of the file at the specified path
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (s *LocalStorage) IsFileExists(path string) (bool, error) {
	return s.pathExists(path, false)
}

func (s *LocalStorage) List(path string) ([]*synchronizer.SyncEntry, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var syncEntries []*synchronizer.SyncEntry
	for _, file := range files {
		syncEntries = append(syncEntries, &synchronizer.SyncEntry{
			IsDir: file.IsDir(),
			Name:  file.Name(),
		})
	}

	return syncEntries, nil
}

func (s *LocalStorage) RemoveDir(path string) error {
	return os.RemoveAll(path)
}

func (s *LocalStorage) Delete(path string) error {
	return os.Remove(path)
}

func (s *LocalStorage) Close() error {
	return nil
}

var _ synchronizer.Storage = (*LocalStorage)(nil)
