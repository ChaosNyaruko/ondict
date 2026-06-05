package util

import (
	"os"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	pathMu             sync.RWMutex
	overrideConfigPath string
	overrideTmpPath    string
)

// SetPaths overrides the default config and cache directories.
// Call this before any other util functions, e.g. from a mobile entry point.
// Safe to call from multiple goroutines.
func SetPaths(configPath, tmpPath string) {
	pathMu.Lock()
	overrideConfigPath = configPath
	overrideTmpPath = tmpPath
	pathMu.Unlock()
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
	pathMu.RLock()
	t := overrideTmpPath
	pathMu.RUnlock()
	if t != "" {
		return filepath.Join(TmpDir(), "vocab.db")
	}
	return filepath.Join(ConfigPath(), "vocab.db")
}

func ConfigPath() string {
	pathMu.RLock()
	p := overrideConfigPath
	pathMu.RUnlock()
	if p != "" {
		if err := os.MkdirAll(p, 0o755); err != nil {
			log.Fatalf("Mkdir err: %v", err)
		}
		return p
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
	pathMu.RLock()
	t := overrideTmpPath
	pathMu.RUnlock()
	if t != "" {
		if err := os.MkdirAll(t, 0o755); err != nil {
			log.Fatalf("Mkdir err: %v", err)
		}
		return t
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
