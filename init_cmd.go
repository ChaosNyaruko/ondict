package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/util"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

func runInit() {
	configPath := util.ConfigPath()
	dictsPath := util.DictsPath()

	// Ensure dicts path exists
	if err := os.MkdirAll(dictsPath, 0755); err != nil {
		log.Fatalf("Failed to create dicts directory: %v", err)
	}

	configFile := filepath.Join(configPath, "config.json")
	// Check if config file exists, if not create default
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Info("Creating default config file...")
		defaultConfig := sources.Config{
			Dicts: []sources.DictConfig{
				{
					Name: "Longman Dictionary of Contemporary English",
					Type: "LONGMAN/Easy",
				},
			},
		}
		data, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal default config: %v", err)
		}
		if err := os.WriteFile(configFile, data, 0644); err != nil {
			log.Fatalf("Failed to write config file: %v", err)
		}
		log.Infof("Created config file at %s", configFile)
	} else {
		log.Infof("Config file already exists at %s", configFile)
	}

	var wg sync.WaitGroup

	fmt.Print("Do you want to download the default Longman dictionary? (y/n): ")
	var answer string
	fmt.Scanln(&answer)
	if strings.ToLower(answer) != "y" {
		log.Info("Skipping dictionary download.")
		return
	}

	mdxUrl := "https://github.com/ChaosNyaruko/ondict/releases/download/v0.0.5/Longman.Dictionary.of.Contemporary.English.mdx"
	mdxName := "Longman Dictionary of Contemporary English.mdx"
	mdxPath := filepath.Join(dictsPath, mdxName)

	if _, err := os.Stat(mdxPath); os.IsNotExist(err) {
		log.Infof("Downloading %s...", mdxName)
		if err := downloadFile(mdxUrl, mdxPath); err != nil {
			log.Fatalf("Failed to download MDX: %v", err)
		}
		log.Info("MDX download completed.")
	} else {
		log.Infof("%s already exists, skipping download.", mdxName)
	}

	fmt.Print("Do you want to download the pronunciation/image (MDD) file? It is large (~500MB+). (y/n): ")
	fmt.Scanln(&answer)
	if strings.ToLower(answer) == "y" {
		mddUrl := "https://github.com/ChaosNyaruko/ondict/releases/download/v0.0.5/Longman.Dictionary.of.Contemporary.English.mdd"
		mddName := "Longman Dictionary of Contemporary English.mdd"
		mddPath := filepath.Join(dictsPath, mddName)

		if _, err := os.Stat(mddPath); os.IsNotExist(err) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Info("Starting background download of MDD file...")
				if err := downloadFile(mddUrl, mddPath); err != nil {
					log.Errorf("Failed to download MDD: %v", err)
				} else {
					log.Infof("MDD download completed: %s", mddPath)
				}
			}()
			log.Info("MDD download started in background.")
		} else {
			log.Infof("%s already exists, skipping download.", mddName)
		}
	}

	fmt.Print("Do you want to dump the dictionary to SQLite for faster startup? (y/n): ")
	fmt.Scanln(&answer)
	if strings.ToLower(answer) == "y" {
		dbName := filepath.Join(util.ConfigPath(), "vocab.db")
		if err := dumpToSqlite(mdxPath, dbName, 0); err != nil {
			log.Errorf("Failed to dump to sqlite: %v", err)
		}
	}

	fmt.Print("Do you want to dump default MDD resources(LDOCE5) to cache for web server? (y/n): ")
	fmt.Scanln(&answer)
	if strings.ToLower(answer) == "y" {
		mddName := "Longman Dictionary of Contemporary English.mdd"
		mddPath := filepath.Join(dictsPath, mddName)

		if _, err := os.Stat(mddPath); os.IsNotExist(err) {
			fmt.Print("MDD file not found. Download it first? (y/n): ")
			fmt.Scanln(&answer)
			if strings.ToLower(answer) == "y" {
				mddUrl := "https://github.com/ChaosNyaruko/ondict/releases/download/v0.0.5/Longman.Dictionary.of.Contemporary.English.mdd"
				log.Info("Starting background download of MDD file...")
				if err := downloadFile(mddUrl, mddPath); err != nil {
					log.Errorf("Failed to download MDD: %v", err)
				} else {
					log.Infof("MDD download completed: %s", mddPath)
				}
				log.Info("MDD download started in background.")
				if _, err := os.Stat(mddPath); os.IsNotExist(err) {
					log.Info("MDD file not downloaded, skipping dump.")
				} else {
					if err := dumpMDDResources(mddPath); err != nil {
						log.Errorf("Failed to dump MDD resources: %v", err)
					}
				}
			} else {
				log.Info("Skipping MDD resource dump.")
			}
		} else {
			if err := dumpMDDResources(mddPath); err != nil {
				log.Errorf("Failed to dump MDD resources: %v", err)
			}
		}
	}

	wg.Wait()
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Downloading "+filepath.Base(dest),
	)
	_, err = io.Copy(io.MultiWriter(f, bar), resp.Body)
	return err
}

func dumpToSqlite(mdxPath string, dbPath string, limit int) error {
	log.Infof("Dumping to %s...", dbPath)

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		return fmt.Errorf("open db err: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	_, err = db.Exec(`DROP TABLE IF EXISTS vocab;
CREATE TABLE IF NOT EXISTS vocab(
    word TEXT NOT NULL COLLATE NOCASE,
    src TEXT NOT NULL DEFAULT "",
    def TEXT NOT NULL DEFAULT ""
)`)
	if err != nil {
		return fmt.Errorf("create table error: %v", err)
	}

	m := &decoder.MDict{}
	err = m.Decode(mdxPath, false)
	if err != nil {
		return fmt.Errorf("failed to decode mdx file[%v], err: %v", mdxPath, err)
	}
	defer m.Close()

	log.Infof("Decoding dict %q......", mdxPath)
	words, err := m.DumpDict(limit)
	if err != nil {
		return fmt.Errorf("DumpDict %v err: %v", mdxPath, err)
	}

	log.Infof("Inserting dict to database %q.....", mdxPath)
	// TODO: UI, this may overlap with "mdd file downloading progress bar"
	bar := progressbar.Default(int64(len(words)), fmt.Sprintf("Inserting dict to database: %v", dbPath))

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO vocab (word, src, def) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, vs := range words {
		for _, v := range vs {
			_, err := stmt.Exec(k, mdxPath, v)
			if err != nil {
				log.Errorf("insert word %v, err: %v", k, err)
				continue
			}
		}
		bar.Add(1)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Infof("Dump success!")
	return nil
}

func dumpMDDResources(mddPath string) error {
	log.Infof("Dumping MDD resources from %s...", mddPath)
	m := &decoder.MDict{}
	err := m.Decode(mddPath, false)
	if err != nil {
		return fmt.Errorf("failed to decode MDD file: %v", err)
	}
	defer m.Close()

	if err := m.DumpData(); err != nil {
		return fmt.Errorf("failed to dump MDD data: %v", err)
	}
	log.Infof("MDD resources dumped successfully!")
	return nil
}
