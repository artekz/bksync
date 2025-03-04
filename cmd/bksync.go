package main

import (
	"artekz/bksync/internal/adapter/ftp"
	"artekz/bksync/internal/adapter/local"
	"artekz/bksync/internal/dirchecker"
	"artekz/bksync/internal/synchronizer"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Sync struct {
	Local  string `json:"local"`
	Remote string `json:"remote"`
}

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath("../config")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	storage, err := getStorage()
	if err != nil {
		log.Fatal(err)
	}

	logFileName := fmt.Sprintf("../logs/%s.log", time.Now().Format("2006_01_02_15_04_05"))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   false,
		DisableColors:   true,
		DisableQuote:    true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logger.SetOutput(multiWriter)

	dirchecker := dirchecker.NewDirChecker()
	synchronizer := synchronizer.NewSynchronizer(storage, dirchecker, logger)

	var sync []Sync
	err = viper.UnmarshalKey("sync", &sync)
	if err != nil {
		log.Fatal(err)
	}

	newFiles := 0
	updatedFiles := 0
	skippedFiles := 0
	removedFiles := 0
	for _, s := range sync {
		new, updated, skipped, err := synchronizer.SyncDir(s.Local, s.Remote)
		if err != nil {
			log.Fatal(err)
		}

		newFiles += new
		updatedFiles += updated
		skippedFiles += skipped

		removed, err := synchronizer.RemoveDifferences(s.Local, s.Remote)
		if err != nil {
			log.Fatal(err)
		}
		removedFiles += removed
	}

	logger.Infof("New files: %d", newFiles)
	logger.Infof("Updated files: %d", updatedFiles)
	logger.Infof("Skipped files: %d", skippedFiles)
	logger.Infof("Removed files: %d", removedFiles)
}

func getStorage() (synchronizer.Storage, error) {
	switch viper.GetString("storage") {
	case "ftp":
		conn, err := ftp.Connect()
		if err != nil {
			return nil, err
		}

		return ftp.NewFTPStorage(conn), nil
	case "local":
		return local.NewLocalStorage(), nil
	default:
		return nil, fmt.Errorf("unknown storage type: %s", viper.GetString("storage.type"))
	}
}
