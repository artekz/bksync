package local

import (
	"artekz/bksync/internal/synchronizer"
	"io"
	"os"
	"time"
)

type LocalStorage struct {
}

func NewLocalStorage() *LocalStorage {
	return &LocalStorage{}
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

func (s *LocalStorage) IsDirExists(path string) bool {
	// Check if the directory exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func (s *LocalStorage) ModTime(path string) (time.Time, error) {
	// Get the modification time of the file at the specified path
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (s *LocalStorage) IsFileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
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
