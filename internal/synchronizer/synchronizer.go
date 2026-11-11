package synchronizer

import (
	"artekz/bksync/internal/dirchecker"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var ErrGetTimeNotSupported = errors.New("GetTime is not supported by the server")

type Storage interface {
	JoinPath(parent, child string) string
	Store(path string, file *os.File) error
	MakeDir(path string) error
	IsDirExists(path string) (bool, error)
	ModTime(path string) (time.Time, error)
	IsFileExists(path string) (bool, error)
	List(path string) ([]*SyncEntry, error)
	RemoveDir(path string) error
	Delete(path string) error
	Close() error
}

type SyncEntry struct {
	IsDir bool
	Name  string
}

type Synchronizer struct {
	storage    Storage
	dirChecker *dirchecker.DirChecker
	logger     *logrus.Logger
}

func NewSynchronizer(storage Storage, dirChecker *dirchecker.DirChecker, logger *logrus.Logger) *Synchronizer {
	return &Synchronizer{
		storage:    storage,
		dirChecker: dirChecker,
		logger:     logger,
	}
}

func (s *Synchronizer) SyncDir(localPath, remotePath string) (int, int, int, error) {
	files, err := os.ReadDir(localPath)
	if err != nil {
		s.logger.Error(err)
		return 0, 0, 0, err
	}
	if err := s.ensureDirectory(remotePath); err != nil {
		return 0, 0, 0, err
	}

	newFiles := 0
	updatedFiles := 0
	skippedFiles := 0

	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		localFilePath := filepath.Join(localPath, file.Name())
		remoteFilePath := s.storage.JoinPath(remotePath, file.Name())

		new, updated, skipped, err := s.syncEntry(localFilePath, remoteFilePath, file.IsDir())
		if err != nil {
			return 0, 0, 0, err
		}

		newFiles += new
		updatedFiles += updated
		skippedFiles += skipped
	}

	return newFiles, updatedFiles, skippedFiles, nil
}

func (s *Synchronizer) syncEntry(localPath, remotePath string, isDir bool) (int, int, int, error) {
	if isDir {
		return s.syncDirectory(localPath, remotePath)
	}

	return s.syncFile(localPath, remotePath)
}

func (s *Synchronizer) syncDirectory(localPath, remotePath string) (int, int, int, error) {
	currentDirHash, changed, err := s.dirChecker.HashChanged(localPath)
	if err != nil {
		return 0, 0, 0, err
	}

	if !changed {
		s.logger.Infof("Directory %s has not changed", localPath)
		return 0, 0, 0, nil
	}
	newFiles, updatedFiles, skippedFiles, err := s.SyncDir(localPath, remotePath)
	if err != nil {
		return 0, 0, 0, err
	}

	if err := s.dirChecker.StoreHash(localPath, currentDirHash); err != nil {
		return 0, 0, 0, err
	}

	return newFiles, updatedFiles, skippedFiles, nil
}

func (s *Synchronizer) ensureDirectory(path string) error {
	directoryExists, err := s.storage.IsDirExists(path)
	if errors.Is(err, os.ErrNotExist) {
		directoryExists, err = false, nil
	}
	if err != nil {
		s.logger.Error(err)
		return err
	}
	if !directoryExists {
		if err := s.storage.MakeDir(path); err != nil {
			s.logger.Errorf("failed to create dir: %s %s", path, err)
			return err
		}
	}

	return nil
}

func (s *Synchronizer) syncFile(localPath, remotePath string) (int, int, int, error) {
	localFile, err := os.Open(localPath)
	if err != nil {
		s.logger.Error(err)
		return 0, 0, 0, err
	}
	defer localFile.Close()

	localFileStat, err := localFile.Stat()
	if err != nil {
		s.logger.Error(err)
		return 0, 0, 0, err
	}

	newFiles, updatedFiles, skippedFiles, storeFile, err := s.fileSyncStatus(remotePath, localFileStat.ModTime())
	if err != nil {
		return 0, 0, 0, err
	}

	if storeFile {
		if err := s.storage.Store(remotePath, localFile); err != nil {
			s.logger.Error(err)
			return 0, 0, 0, err
		}
	}

	return newFiles, updatedFiles, skippedFiles, nil
}

func (s *Synchronizer) fileSyncStatus(remotePath string, localModTime time.Time) (int, int, int, bool, error) {
	fileExists, err := s.storage.IsFileExists(remotePath)
	if errors.Is(err, os.ErrNotExist) {
		fileExists, err = false, nil
	}
	if err != nil {
		s.logger.Error(err)
		return 0, 0, 0, false, err
	}
	if !fileExists {
		s.logger.Infof("sending a new file to the server: %s", remotePath)
		return 1, 0, 0, true, nil
	}

	remoteModTime, err := s.storage.ModTime(remotePath)
	if err != nil {
		if errors.Is(err, ErrGetTimeNotSupported) {
			s.logger.Warnf("GetTime is not supported by the server, updating file: %s", remotePath)
		} else {
			s.logger.Error(err)
			return 0, 0, 0, false, err
		}
	}

	if localModTime.Equal(remoteModTime) || localModTime.Before(remoteModTime) {
		s.logger.Infof("skipping file: %s", remotePath)
		return 0, 0, 1, false, nil
	}

	s.logger.Infof("updating file to the server: %s", remotePath)
	return 0, 1, 0, true, nil
}

func (s *Synchronizer) RemoveDifferences(localPath, remotePath string) (int, error) {
	remoteFiles, err := s.storage.List(remotePath)
	if err != nil {
		s.logger.Error(err)
		return 0, err
	}

	localFiles, err := os.ReadDir(localPath)
	if err != nil {
		s.logger.Error(err)
		return 0, err
	}

	localFileMap := make(map[string]struct{}, len(localFiles))
	for _, file := range localFiles {
		localFileMap[file.Name()] = struct{}{}
	}

	removedFiles := 0
	for _, remoteFile := range remoteFiles {
		if strings.HasPrefix(remoteFile.Name, ".") {
			continue
		}

		localFilePath := filepath.Join(localPath, remoteFile.Name)
		remoteFilePath := s.storage.JoinPath(remotePath, remoteFile.Name)

		removed, err := s.removeDifference(localFilePath, remoteFilePath, remoteFile, localFileMap)
		if err != nil {
			return 0, err
		}
		removedFiles += removed
	}

	return removedFiles, nil
}

func (s *Synchronizer) removeDifference(localPath, remotePath string, remoteFile *SyncEntry, localFiles map[string]struct{}) (int, error) {
	if _, exists := localFiles[remoteFile.Name]; !exists {
		if remoteFile.IsDir {
			s.logger.Infof("deleting remote dir: %s", remotePath)
			if err := s.storage.RemoveDir(remotePath); err != nil {
				s.logger.Error(err)
				return 0, err
			}
			return 0, nil
		}

		s.logger.Infof("deleting remote file: %s", remotePath)
		if err := s.storage.Delete(remotePath); err != nil {
			s.logger.Error(err)
			return 0, err
		}
		return 1, nil
	}

	if remoteFile.IsDir {
		return s.RemoveDifferences(localPath, remotePath)
	}

	return 0, nil
}
