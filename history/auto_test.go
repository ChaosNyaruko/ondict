package history

import (
	"os"
	"testing"
	"time"

	"github.com/ChaosNyaruko/ondict/util"
	"github.com/stretchr/testify/assert"
)

func TestTxtWriter_Append(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	w := NewTxtWriter()
	err := w.Append("testword")
	assert.NoError(t, err)

	content, err := os.ReadFile(util.HistoryTable())
	assert.NoError(t, err)
	assert.Contains(t, string(content), "testword")
}

func TestSqlite3Writer_Append(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	w := NewSqlite3Writer()
	err := w.Append("testword")
	assert.NoError(t, err)

	// Check if db exists
	_, err = os.Stat(util.HistoryDB())
	assert.NoError(t, err)
}

func TestHistory_Append(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	txtW := NewTxtWriter()
	sqlW := NewSqlite3Writer()
	h := NewHistory(txtW, sqlW)

	err := h.Append("bothword")
	assert.NoError(t, err)

	// Verify txt
	content, err := os.ReadFile(util.HistoryTable())
	assert.NoError(t, err)
	assert.Contains(t, string(content), "bothword")
}

func TestHistory_Review(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// We need to write some data first
	sqlW := NewSqlite3Writer()
	h := NewHistory(sqlW)

	err := h.Append("reviewword")
	assert.NoError(t, err)
	time.Sleep(100 * time.Millisecond) // Ensure DB write

	// Review checks for words updated > -days and count >= count
	// Append adds with count 1 and current time.
	// So -1 days and count 1 should match.
	res, err := h.Review("1", "1")
	assert.NoError(t, err)
	assert.Contains(t, res, "reviewword")
}
