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
	Store(path string, file *os.File) error
	MakeDir(path string) error
	IsDirExists(path string) bool
	ModTime(path string) (time.Time, error)
	IsFileExists(path string) bool
	List(path string) ([]*SyncEntry, error)
	RemoveDir(path string) error
	Delete(path string) error
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

	newFiles := 0
	updatedFiles := 0
	skippedFiles := 0

	for _, file := range files {

		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		localFilePath := filepath.Join(localPath, file.Name())
		remoteFilePath := filepath.Join(remotePath, file.Name())

		if file.IsDir() {
			currentDirHash, err := s.dirChecker.GenerateDirHash(localFilePath)
			if err != nil {
				return 0, 0, 0, err
			}

			if currentDirHash == s.dirChecker.GetDriHash(localFilePath) {
				s.logger.Infof("Directory %s has not changed", localFilePath)
				continue
			}

			if !s.storage.IsDirExists(remoteFilePath) {
				err := s.storage.MakeDir(remoteFilePath)
				if err != nil {
					s.logger.Fatalf("failed to create dir: %s %s", remoteFilePath, err)
					return 0, 0, 0, err
				}
			}

			new, updated, skipped, err := s.SyncDir(localFilePath, remoteFilePath)
			if err != nil {
				return 0, 0, 0, err
			}
			newFiles += new
			updatedFiles += updated
			skippedFiles += skipped
			err = s.dirChecker.StoreHash(localFilePath, currentDirHash)
			if err != nil {
				return 0, 0, 0, err
			}
		} else {
			localFile, err := os.Open(localFilePath)
			if err != nil {
				s.logger.Error(err)
				return 0, 0, 0, err
			}

			localFileStat, err := os.Stat(localFilePath)
			if err != nil {
				s.logger.Error(err)
				return 0, 0, 0, err
			}

			storeFile := true
			if s.storage.IsFileExists(remoteFilePath) {
				remoteFileModTime, err := s.storage.ModTime(remoteFilePath)
				if err != nil {
					if errors.Is(err, ErrGetTimeNotSupported) {
						s.logger.Warnf("GetTime is not supported by the server, skipping file: %s", remoteFilePath)
					} else {
						s.logger.Error(err)
						return 0, 0, 0, err
					}
				}

				if localFileStat.ModTime().Equal(remoteFileModTime) ||
					localFileStat.ModTime().Before(remoteFileModTime) {
					storeFile = false
					s.logger.Infof("skipping file: %s", remoteFilePath)
					skippedFiles++
				} else {
					s.logger.Infof("updating file to the server: %s", remoteFilePath)
					updatedFiles++
				}
			} else {
				newFiles++
				s.logger.Infof("sending a new file to the server: %s", remoteFilePath)
			}

			if storeFile {
				err = s.storage.Store(remoteFilePath, localFile)
				if err != nil {
					s.logger.Error(err)
					return 0, 0, 0, err
				}
			}

			localFile.Close()
		}

	}

	return newFiles, updatedFiles, skippedFiles, nil
}

func (s *Synchronizer) RemoveDifferences(localPath, remotePath string) (int, error) {
	removedFiles := 0
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

	localFileMap := make(map[string]bool)
	for _, file := range localFiles {
		localFileMap[file.Name()] = true
	}

	for _, remoteFile := range remoteFiles {
		if strings.HasPrefix(remoteFile.Name, ".") {
			continue
		}

		localFilePath := filepath.Join(localPath, remoteFile.Name)
		remoteFilePath := filepath.Join(remotePath, remoteFile.Name)

		if !localFileMap[remoteFile.Name] {
			if remoteFile.IsDir {
				s.logger.Infof("deleting remote dir: %s", remoteFilePath)
				err := s.storage.RemoveDir(remoteFilePath)
				if err != nil {
					s.logger.Error(err)
					return 0, err
				}
			} else {
				s.logger.Infof("deleting remote file: %s", remoteFilePath)
				err := s.storage.Delete(remoteFilePath)
				if err != nil {
					s.logger.Error(err)
					return 0, err
				}
				removedFiles++
			}
		} else if remoteFile.IsDir {
			removed, err := s.RemoveDifferences(localFilePath, remoteFilePath)
			if err != nil {
				return 0, err
			}
			removedFiles += removed
		}
	}

	return removedFiles, nil
}
