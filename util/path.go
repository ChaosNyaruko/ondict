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

// VocabDB returns the path to vocab.db.
// On mobile (where SetPaths is called) this lands in cacheDir (evictable) since
// vocab.db is a derived cache rebuilt automatically from the MDX source files.
// On desktop it lives alongside the other config files under ~/.config/ondict.
func VocabDB() string {
	if overrideTmpPath != "" {
		return filepath.Join(TmpDir(), "vocab.db")
	}
	return filepath.Join(ConfigPath(), "vocab.db")
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
