package util

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

func HistoryFile() string {
	return filepath.Join(ConfigPath(), "history.json")
}

func HistoryTable() string {
	return filepath.Join(ConfigPath(), "history.table")
}

func HistoryDB() string {
	return filepath.Join(ConfigPath(), "history.db")
}

func DictsPath() string {
	return filepath.Join(ConfigPath(), "dicts")
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	configPath := filepath.Join(home, ".config", "ondict")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		log.Fatalf("Mkdir err: %v", err)
	}
	return configPath
}

func TmpDir() string {
	home, err := os.UserCacheDir()
	if err != nil {
		log.Fatal(err)
	}
	tmpPath := filepath.Join(home, "ondict")
	if err := os.MkdirAll(tmpPath, 0o755); err != nil {
		log.Fatalf("Mkdir err: %v", err)
	}
	return tmpPath
}
