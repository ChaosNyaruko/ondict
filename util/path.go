package util

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

var overrideConfigPath string
var overrideTmpPath string

// SetPaths overrides the default config and cache directories.
// Call this before any other util functions, e.g. from a mobile entry point.
func SetPaths(configPath, tmpPath string) {
	overrideConfigPath = configPath
	overrideTmpPath = tmpPath
}

func HistoryFile() string {
	return filepath.Join(ConfigPath(), "history.json")
}

func HistoryTable() string {
	return filepath.Join(ConfigPath(), "history.table")
}

func HistoryDB() string {
	return filepath.Join(ConfigPath(), "history.db")
}

func WordBankDB() string {
	return filepath.Join(ConfigPath(), "wordbank.db")
}

func DictsPath() string {
	return filepath.Join(ConfigPath(), "dicts")
}

func ConfigPath() string {
	if overrideConfigPath != "" {
		if err := os.MkdirAll(overrideConfigPath, 0o755); err != nil {
			log.Fatalf("Mkdir err: %v", err)
		}
		return overrideConfigPath
	}
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
	if overrideTmpPath != "" {
		if err := os.MkdirAll(overrideTmpPath, 0o755); err != nil {
			log.Fatalf("Mkdir err: %v", err)
		}
		return overrideTmpPath
	}
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
