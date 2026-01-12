package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
)

func TestDownloadFile(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create a temporary file path
	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "test_download.txt")

	// Test downloading
	err := downloadFile(server.URL, dest)
	assert.NoError(t, err)

	// Verify file content
	content, err := os.ReadFile(dest)
	assert.NoError(t, err)
	assert.Equal(t, "OK", string(content))
}

func TestDownloadFile_Error(t *testing.T) {
	// Start a local HTTP server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "test_download_error.txt")

	t.Logf("server.URL: %v", server.URL)
	err := downloadFile(server.URL, dest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad status")
}

func TestDumpToSqlite(t *testing.T) {
	// Use existing test data
	mdxPath := filepath.Join("testdata", "Longman Dictionary of Contemporary English.mdx")
	if _, err := os.Stat(mdxPath); os.IsNotExist(err) {
		t.Skip("Test MDX file not found, skipping dump test")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_vocab.db")

	err := dumpToSqlite(mdxPath, dbPath, 100)
	assert.NoError(t, err)

	// Verify database content
	db, err := sql.Open("sqlite3", "file:"+dbPath)
	assert.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM vocab").Scan(&count)
	assert.NoError(t, err)
	assert.Greater(t, count, 0, "Database should contain some records")
	assert.LessOrEqual(t, count, 100, "Database should contain limited records")
}
