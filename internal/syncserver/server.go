// Package syncserver implements the cloud-sync HTTP endpoints described in
// docs/adr/0001-wordbank-history-sync.md.
//
// The transport is HTTP REST + JSON (ADR D5) and authentication is a single
// HTTP Basic Auth credential pair (ADR D4). All sync data lives under a
// per-user directory:
//
//	<dataDir>/<user>/wordbank.db
//	<dataDir>/<user>/history.db
//
// A SyncServer is mounted by registering it onto an existing *gin.Engine
// via Mount(). The endpoints all live under the /sync/v1 prefix and reuse
// the host server's middleware stack (logger, recovery, etc.).
package syncserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ChaosNyaruko/ondict/store"
	"github.com/ChaosNyaruko/ondict/syncmerge"
)

// SchemaVersion is the wire-protocol version. Bumped whenever the JSON shape
// of a sync request or response changes incompatibly.
const SchemaVersion = 1

// Config configures a SyncServer.
type Config struct {
	// DataDir is the directory containing per-user subdirectories. Each
	// user has their own wordbank.db / history.db inside <DataDir>/<user>/.
	DataDir string

	// Username and Password are the Basic Auth credentials. Both must be
	// non-empty; an empty credential disables the server (Mount returns an
	// error). Compared with constant-time equality.
	Username string
	Password string
}

// SyncServer is the gin-mountable sync handler.
//
// Per-user stores are opened lazily on first use and cached in memory; we
// never re-open the same SQLite DB twice. Concurrent requests for the same
// user share a single store handle (database/sql provides its own
// connection pool).
type SyncServer struct {
	cfg Config

	mu       sync.Mutex
	wordbank map[string]store.WordbankStore
	history  map[string]store.HistoryStore
}

// New constructs a SyncServer. Returns an error if Config is invalid.
func New(cfg Config) (*SyncServer, error) {
	if cfg.DataDir == "" {
		return nil, errors.New("syncserver: DataDir is required")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("syncserver: Username and Password are required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("syncserver: create DataDir %q: %w", cfg.DataDir, err)
	}
	return &SyncServer{
		cfg:      cfg,
		wordbank: map[string]store.WordbankStore{},
		history:  map[string]store.HistoryStore{},
	}, nil
}

// Mount registers the sync routes on r under /sync/v1 with Basic Auth.
func (s *SyncServer) Mount(r *gin.Engine) {
	g := r.Group("/sync/v1", s.basicAuth())
	g.GET("/state", s.handleState)
	g.POST("/wordbank/pull", s.handleWordbankPull)
	g.POST("/wordbank/push", s.handleWordbankPush)
	g.POST("/history/pull", s.handleHistoryPull)
	g.POST("/history/push", s.handleHistoryPush)
}

// Close releases every cached per-user store handle.
func (s *SyncServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var first error
	for _, w := range s.wordbank {
		if err := w.Close(); err != nil && first == nil {
			first = err
		}
	}
	for _, h := range s.history {
		if err := h.Close(); err != nil && first == nil {
			first = err
		}
	}
	s.wordbank = map[string]store.WordbankStore{}
	s.history = map[string]store.HistoryStore{}
	return first
}

// basicAuth returns a middleware that requires the configured credential.
// On success the authenticated user is stashed at c.Keys["sync.user"].
func (s *SyncServer) basicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, p, ok := c.Request.BasicAuth()
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="ondict-sync"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		// Constant-time comparison; mismatched lengths still take the same
		// path so we don't leak length via timing.
		userOK := subtle.ConstantTimeCompare([]byte(u), []byte(s.cfg.Username)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(p), []byte(s.cfg.Password)) == 1
		if !userOK || !passOK {
			c.Header("WWW-Authenticate", `Basic realm="ondict-sync"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("sync.user", u)
		c.Next()
	}
}

// ---------------------------------------------------------------------------
// Wire types
// ---------------------------------------------------------------------------

type stateResponse struct {
	ServerNow     string `json:"server_now"`
	SchemaVersion int    `json:"schema_version"`
}

type pullRequest struct {
	// Since is an RFC3339 UTC timestamp. Empty means "send the full snapshot".
	Since string `json:"since"`
}

type wordbankPullResponse struct {
	Items     []wordbankItem `json:"items"`
	ServerNow string         `json:"server_now"`
}

type wordbankPushRequest struct {
	Items []wordbankItem `json:"items"`
}

type wordbankPushResponse struct {
	Applied int             `json:"applied"`
	Stats   syncmerge.Stats `json:"stats"`
}

type wordbankItem struct {
	Word       string `json:"word"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	DeletedAt  string `json:"deleted_at,omitempty"`
}

type historyPullResponse struct {
	Items     []historyItem `json:"items"`
	ServerNow string        `json:"server_now"`
}

type historyPushRequest struct {
	Items []historyItem `json:"items"`
}

type historyPushResponse struct {
	Applied int             `json:"applied"`
	Stats   syncmerge.Stats `json:"stats"`
}

type historyItem struct {
	Word       string `json:"word"`
	Count      int    `json:"count"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	DeletedAt  string `json:"deleted_at,omitempty"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// rfc3339Ms is the timestamp format the server emits for server_now and
// the format SQLite uses for server_seen_at. Sub-second resolution prevents
// adjacent writes from collapsing into the same string and being missed by
// `WHERE server_seen_at > ?` filters.
const rfc3339Ms = "2006-01-02T15:04:05.000Z07:00"

func nowMs() string { return time.Now().UTC().Format(rfc3339Ms) }

func (s *SyncServer) handleState(c *gin.Context) {
	c.JSON(http.StatusOK, stateResponse{
		ServerNow:     nowMs(),
		SchemaVersion: SchemaVersion,
	})
}

func (s *SyncServer) handleWordbankPull(c *gin.Context) {
	user := c.GetString("sync.user")
	wb, err := s.wordbankFor(user)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	req, err := decodePull(c.Request.Body)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	since, err := parseSince(req.Since)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	// Capture the cutoff BEFORE running the query so any concurrent push
	// whose server_seen_at lands after cutoff is excluded from this
	// response and surfaces on the next pull. Without this, a write that
	// commits between ListSince and ServerNow would be skipped because
	// the client would advance its cursor past it. (code-review P1)
	cutoff := time.Now().UTC()
	rows, err := wb.ListChanged(c.Request.Context(), since, cutoff)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	resp := wordbankPullResponse{
		Items:     make([]wordbankItem, 0, len(rows)),
		ServerNow: cutoff.Format(rfc3339Ms),
	}
	for _, r := range rows {
		resp.Items = append(resp.Items, wordbankItem{
			Word:       r.Word,
			CreateTime: r.CreateTime,
			UpdateTime: r.UpdateTime,
			DeletedAt:  r.DeletedAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (s *SyncServer) handleWordbankPush(c *gin.Context) {
	user := c.GetString("sync.user")
	wb, err := s.wordbankFor(user)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	var req wordbankPushRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// Reuse the merge engine to apply incoming items: each push is
	// equivalent to merging a tiny in-memory wordbank into the user's DB.
	src := newInMemoryWordbank(req.Items)
	stats, err := syncmerge.MergeWordbank(c.Request.Context(), wb, src)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, wordbankPushResponse{
		Applied: stats.Inserted + stats.Updated,
		Stats:   stats,
	})
}

func (s *SyncServer) handleHistoryPull(c *gin.Context) {
	user := c.GetString("sync.user")
	h, err := s.historyFor(user)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	req, err := decodePull(c.Request.Body)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	since, err := parseSince(req.Since)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	// See handleWordbankPull for the cutoff-before-query rationale (P1).
	cutoff := time.Now().UTC()
	rows, err := h.ListChanged(c.Request.Context(), since, cutoff)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	resp := historyPullResponse{
		Items:     make([]historyItem, 0, len(rows)),
		ServerNow: cutoff.Format(rfc3339Ms),
	}
	for _, r := range rows {
		resp.Items = append(resp.Items, historyItem{
			Word:       r.Word,
			Count:      r.Count,
			CreateTime: r.CreateTime,
			UpdateTime: r.UpdateTime,
			DeletedAt:  r.DeletedAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (s *SyncServer) handleHistoryPush(c *gin.Context) {
	user := c.GetString("sync.user")
	h, err := s.historyFor(user)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	var req historyPushRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	src := newInMemoryHistory(req.Items)
	stats, err := syncmerge.MergeHistory(c.Request.Context(), h, src)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, historyPushResponse{
		Applied: stats.Inserted + stats.Updated,
		Stats:   stats,
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func decodePull(body io.Reader) (pullRequest, error) {
	var req pullRequest
	dec := json.NewDecoder(body)
	if err := dec.Decode(&req); err != nil {
		// An empty body is allowed and means "full snapshot".
		if errors.Is(err, io.EOF) {
			return pullRequest{}, nil
		}
		return req, err
	}
	return req, nil
}

func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	// Accept either second- or millisecond-resolution RFC3339. We re-parse
	// the value not for the time.Time itself (the server only uses the
	// string form when filtering SQLite) but to validate the input.
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid since timestamp %q: %w", s, err)
	}
	return t, nil
}

func (s *SyncServer) wordbankFor(user string) (store.WordbankStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.wordbank[user]; ok {
		return w, nil
	}
	dir := filepath.Join(s.cfg.DataDir, user)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w, err := store.OpenSQLiteWordbank(filepath.Join(dir, "wordbank.db"))
	if err != nil {
		return nil, err
	}
	s.wordbank[user] = w
	return w, nil
}

func (s *SyncServer) historyFor(user string) (store.HistoryStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.history[user]; ok {
		return h, nil
	}
	dir := filepath.Join(s.cfg.DataDir, user)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	h, err := store.OpenSQLiteHistory(filepath.Join(dir, "history.db"))
	if err != nil {
		return nil, err
	}
	s.history[user] = h
	return h, nil
}

// ---------------------------------------------------------------------------
// In-memory adapters used by handlePush — the sync engine wants a Store, but
// the inbound request is just a JSON array. These adapters make each push
// look like a one-off "tiny source store" to syncmerge.MergeWordbank /
// syncmerge.MergeHistory.
// ---------------------------------------------------------------------------

type inMemoryWordbank struct{ rows []store.WordbankRow }

func newInMemoryWordbank(items []wordbankItem) *inMemoryWordbank {
	rows := make([]store.WordbankRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, store.WordbankRow{
			Word:       it.Word,
			CreateTime: it.CreateTime,
			UpdateTime: it.UpdateTime,
			DeletedAt:  it.DeletedAt,
		})
	}
	return &inMemoryWordbank{rows: rows}
}

func (m *inMemoryWordbank) Close() error                         { return nil }
func (m *inMemoryWordbank) Add(context.Context, string) error    { return errReadOnly }
func (m *inMemoryWordbank) Remove(context.Context, string) error { return errReadOnly }
func (m *inMemoryWordbank) Contains(context.Context, string) (bool, error) {
	return false, errReadOnly
}
func (m *inMemoryWordbank) List(context.Context) ([]store.WordbankRow, error) {
	return m.rows, nil
}
func (m *inMemoryWordbank) ListSince(_ context.Context, _ time.Time) ([]store.WordbankRow, error) {
	return m.rows, nil
}
func (m *inMemoryWordbank) ListChanged(_ context.Context, _, _ time.Time) ([]store.WordbankRow, error) {
	return m.rows, nil
}
func (m *inMemoryWordbank) Upsert(context.Context, store.WordbankRow) error { return errReadOnly }
func (m *inMemoryWordbank) GCTombstones(context.Context, time.Time) (int, error) {
	return 0, errReadOnly
}

type inMemoryHistory struct{ rows []store.HistoryRow }

func newInMemoryHistory(items []historyItem) *inMemoryHistory {
	rows := make([]store.HistoryRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, store.HistoryRow{
			Word:       it.Word,
			Count:      it.Count,
			CreateTime: it.CreateTime,
			UpdateTime: it.UpdateTime,
			DeletedAt:  it.DeletedAt,
		})
	}
	return &inMemoryHistory{rows: rows}
}

func (m *inMemoryHistory) Close() error                         { return nil }
func (m *inMemoryHistory) Append(context.Context, string) error { return errReadOnly }
func (m *inMemoryHistory) List(context.Context) ([]store.HistoryRow, error) {
	return m.rows, nil
}
func (m *inMemoryHistory) ListSince(_ context.Context, _ time.Time) ([]store.HistoryRow, error) {
	return m.rows, nil
}
func (m *inMemoryHistory) ListChanged(_ context.Context, _, _ time.Time) ([]store.HistoryRow, error) {
	return m.rows, nil
}
func (m *inMemoryHistory) Review(_ context.Context, _, _ int) ([]store.HistoryRow, error) {
	return nil, errReadOnly
}
func (m *inMemoryHistory) Upsert(context.Context, store.HistoryRow) error { return errReadOnly }
func (m *inMemoryHistory) GCTombstones(context.Context, time.Time) (int, error) {
	return 0, errReadOnly
}

var errReadOnly = errors.New("syncserver: in-memory adapter is read-only")
