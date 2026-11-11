package main

import (
	"artekz/bksync/internal/adapter/ftp"
	"artekz/bksync/internal/adapter/local"
	"artekz/bksync/internal/dirchecker"
	"artekz/bksync/internal/synchronizer"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Sync struct {
	Local  string `mapstructure:"local"`
	Remote string `mapstructure:"remote"`
}

type Config struct {
	Storage string     `mapstructure:"storage"`
	FTP     ftp.Config `mapstructure:"ftp"`
	Sync    []Sync     `mapstructure:"sync"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (runErr error) {
	configDir, err := filepath.Abs("../config")
	if err != nil {
		return err
	}
	projectDir := filepath.Dir(configDir)

	config, err := loadConfig(configDir)
	if err != nil {
		return err
	}

	logger, logFile, err := newLogger(filepath.Join(projectDir, "logs"))
	if err != nil {
		return err
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close log file: %w", err))
		}
	}()

	storage, err := getStorage(config)
	if err != nil {
		return err
	}
	defer func() {
		if err := storage.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close storage: %w", err))
		}
	}()

	dirChecker := dirchecker.NewDirChecker(filepath.Join(projectDir, "hashes.json"))
	syncer := synchronizer.NewSynchronizer(storage, dirChecker, logger)

	newFiles := 0
	updatedFiles := 0
	skippedFiles := 0
	removedFiles := 0
	for _, syncPair := range config.Sync {
		new, updated, skipped, err := syncer.SyncDir(syncPair.Local, syncPair.Remote)
		if err != nil {
			return fmt.Errorf("sync %s to %s: %w", syncPair.Local, syncPair.Remote, err)
		}

		newFiles += new
		updatedFiles += updated
		skippedFiles += skipped

		removed, err := syncer.RemoveDifferences(syncPair.Local, syncPair.Remote)
		if err != nil {
			return fmt.Errorf("remove differences for %s and %s: %w", syncPair.Local, syncPair.Remote, err)
		}
		removedFiles += removed
	}

	logger.Infof("New files: %d", newFiles)
	logger.Infof("Updated files: %d", updatedFiles)
	logger.Infof("Skipped files: %d", skippedFiles)
	logger.Infof("Removed files: %d", removedFiles)
	return nil
}

func loadConfig(configDir string) (Config, error) {
	configReader := viper.New()
	configReader.SetConfigName("config")
	configReader.SetConfigType("json")
	configReader.AddConfigPath(configDir)
	if err := configReader.ReadInConfig(); err != nil {
		return Config{}, err
	}

	var config Config
	if err := configReader.Unmarshal(&config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func newLogger(logDir string) (*logrus.Logger, *os.File, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}

	logFileName := filepath.Join(logDir, fmt.Sprintf("%s.log", time.Now().Format("2006_01_02_15_04_05")))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, err
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   false,
		DisableColors:   true,
		DisableQuote:    true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
	return logger, logFile, nil
}

func getStorage(config Config) (synchronizer.Storage, error) {
	switch config.Storage {
	case "ftp":
		connection, err := ftp.Connect(config.FTP)
		if err != nil {
			return nil, err
		}
		return ftp.NewFTPStorage(connection), nil
	case "local":
		return local.NewLocalStorage(), nil
	default:
		return nil, fmt.Errorf("unknown storage type: %q", config.Storage)
	}
}
