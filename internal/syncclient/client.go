// Package syncclient is the client side of the cloud-sync protocol described
// in docs/adr/0001-wordbank-history-sync.md. It runs on every device that
// wants to push its local wordbank/history to a sync server and pull back
// updates from other devices.
//
// The client is intentionally stateless beyond a small `meta`-table cursor:
// `sync.last_pull.<resource>` records the server timestamp of the last
// successful pull, so subsequent pulls only fetch deltas.
package syncclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ChaosNyaruko/ondict/store"
	"github.com/ChaosNyaruko/ondict/syncmerge"
)

// Config configures a SyncClient.
type Config struct {
	// BaseURL is the sync server origin, e.g. "https://sync.example.com".
	// The client appends "/sync/v1/..." paths.
	BaseURL string

	// Username and Password are the Basic Auth credentials.
	Username string
	Password string

	// HTTPClient is optional. When nil, http.DefaultClient with a 30s
	// timeout is used.
	HTTPClient *http.Client
}

// Stats summarises a single Sync() invocation.
type Stats struct {
	WordbankPullApplied syncmerge.Stats
	WordbankPushed      int
	HistoryPullApplied  syncmerge.Stats
	HistoryPushed       int
	ServerNow           string
}

// SyncClient binds local stores to a sync server.
type SyncClient struct {
	cfg      Config
	wordbank store.WordbankStore
	history  store.HistoryStore
	meta     CursorStore

	httpDoer *http.Client
}

// CursorStore is the small abstraction for "remember the last-pulled
// timestamp". Implementations live next to the local SQLite store
// (typically the meta table) so the cursor survives process restarts.
type CursorStore interface {
	GetCursor(ctx context.Context, key string) (string, error)
	SetCursor(ctx context.Context, key, value string) error
}

// New constructs a SyncClient. The wordbank, history, and meta stores are
// the *local* (per-device) state to be synced.
func New(cfg Config, wordbank store.WordbankStore, history store.HistoryStore, meta CursorStore) (*SyncClient, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("syncclient: BaseURL is required")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("syncclient: Username and Password are required")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	doer := cfg.HTTPClient
	if doer == nil {
		doer = &http.Client{Timeout: 30 * time.Second}
	}
	return &SyncClient{
		cfg:      cfg,
		wordbank: wordbank,
		history:  history,
		meta:     meta,
		httpDoer: doer,
	}, nil
}

// Sync runs a full pull-then-push cycle for both wordbank and history.
//
// Pull-then-push (instead of push-then-pull) means the client always sees
// the latest server state before re-publishing its own — minimising the
// number of rows the server has to ingest in any given call.
func (c *SyncClient) Sync(ctx context.Context) (Stats, error) {
	var s Stats

	wbPull, err := c.pullWordbank(ctx)
	if err != nil {
		return s, fmt.Errorf("pull wordbank: %w", err)
	}
	s.WordbankPullApplied = wbPull

	wbPushed, err := c.pushWordbank(ctx)
	if err != nil {
		return s, fmt.Errorf("push wordbank: %w", err)
	}
	s.WordbankPushed = wbPushed

	hPull, err := c.pullHistory(ctx)
	if err != nil {
		return s, fmt.Errorf("pull history: %w", err)
	}
	s.HistoryPullApplied = hPull

	hPushed, err := c.pushHistory(ctx)
	if err != nil {
		return s, fmt.Errorf("push history: %w", err)
	}
	s.HistoryPushed = hPushed

	return s, nil
}

// ---------------------------------------------------------------------------
// Wordbank pull / push
// ---------------------------------------------------------------------------

const (
	cursorWordbankPull = "sync.last_pull.wordbank"
	cursorWordbankPush = "sync.last_push.wordbank"
	cursorHistoryPull  = "sync.last_pull.history"
	cursorHistoryPush  = "sync.last_push.history"
)

func (c *SyncClient) pullWordbank(ctx context.Context) (syncmerge.Stats, error) {
	since, err := c.meta.GetCursor(ctx, cursorWordbankPull)
	if err != nil {
		return syncmerge.Stats{}, err
	}
	var resp struct {
		Items     []wordbankItem `json:"items"`
		ServerNow string         `json:"server_now"`
	}
	if err := c.postJSON(ctx, "/sync/v1/wordbank/pull", pullRequest{Since: since}, &resp); err != nil {
		return syncmerge.Stats{}, err
	}
	src := newRowSourceWordbank(resp.Items)
	stats, err := syncmerge.MergeWordbank(ctx, c.wordbank, src)
	if err != nil {
		return stats, err
	}
	if err := c.meta.SetCursor(ctx, cursorWordbankPull, resp.ServerNow); err != nil {
		return stats, err
	}
	return stats, nil
}

func (c *SyncClient) pushWordbank(ctx context.Context) (int, error) {
	since, err := c.meta.GetCursor(ctx, cursorWordbankPush)
	if err != nil {
		return 0, err
	}
	t, _ := parseRFC3339OrZero(since)
	rows, err := c.wordbank.ListSince(ctx, t)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	items := make([]wordbankItem, 0, len(rows))
	// The push cursor MUST advance in the same time domain that ListSince
	// filters on (server_seen_at), not the user-content update_time.
	// Otherwise an inbound row with a future update_time would block all
	// subsequent local writes from being pushed (and same-instant rows
	// would be re-pushed forever). See ADR D11 / code-review P1.
	maxSeen := since
	for _, r := range rows {
		items = append(items, wordbankItem{
			Word:       r.Word,
			CreateTime: r.CreateTime,
			UpdateTime: r.UpdateTime,
			DeletedAt:  r.DeletedAt,
		})
		if r.SeenAt > maxSeen {
			maxSeen = r.SeenAt
		}
	}
	var resp struct {
		Applied int `json:"applied"`
	}
	if err := c.postJSON(ctx, "/sync/v1/wordbank/push",
		struct {
			Items []wordbankItem `json:"items"`
		}{Items: items},
		&resp,
	); err != nil {
		return 0, err
	}
	if maxSeen != "" && maxSeen != since {
		if err := c.meta.SetCursor(ctx, cursorWordbankPush, maxSeen); err != nil {
			return resp.Applied, err
		}
	}
	return resp.Applied, nil
}

func (c *SyncClient) pullHistory(ctx context.Context) (syncmerge.Stats, error) {
	since, err := c.meta.GetCursor(ctx, cursorHistoryPull)
	if err != nil {
		return syncmerge.Stats{}, err
	}
	var resp struct {
		Items     []historyItem `json:"items"`
		ServerNow string        `json:"server_now"`
	}
	if err := c.postJSON(ctx, "/sync/v1/history/pull", pullRequest{Since: since}, &resp); err != nil {
		return syncmerge.Stats{}, err
	}
	src := newRowSourceHistory(resp.Items)
	stats, err := syncmerge.MergeHistory(ctx, c.history, src)
	if err != nil {
		return stats, err
	}
	if err := c.meta.SetCursor(ctx, cursorHistoryPull, resp.ServerNow); err != nil {
		return stats, err
	}
	return stats, nil
}

func (c *SyncClient) pushHistory(ctx context.Context) (int, error) {
	since, err := c.meta.GetCursor(ctx, cursorHistoryPush)
	if err != nil {
		return 0, err
	}
	t, _ := parseRFC3339OrZero(since)
	rows, err := c.history.ListSince(ctx, t)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	items := make([]historyItem, 0, len(rows))
	// See pushWordbank for why the cursor is in server_seen_at, not
	// update_time, time domain.
	maxSeen := since
	for _, r := range rows {
		items = append(items, historyItem{
			Word:       r.Word,
			Count:      r.Count,
			CreateTime: r.CreateTime,
			UpdateTime: r.UpdateTime,
			DeletedAt:  r.DeletedAt,
		})
		if r.SeenAt > maxSeen {
			maxSeen = r.SeenAt
		}
	}
	var resp struct {
		Applied int `json:"applied"`
	}
	if err := c.postJSON(ctx, "/sync/v1/history/push",
		struct {
			Items []historyItem `json:"items"`
		}{Items: items},
		&resp,
	); err != nil {
		return 0, err
	}
	if maxSeen != "" && maxSeen != since {
		if err := c.meta.SetCursor(ctx, cursorHistoryPush, maxSeen); err != nil {
			return resp.Applied, err
		}
	}
	return resp.Applied, nil
}

// ---------------------------------------------------------------------------
// HTTP plumbing
// ---------------------------------------------------------------------------

func (c *SyncClient) postJSON(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	resp, err := c.httpDoer.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		// Best-effort body capture for diagnostics.
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(buf)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func parseRFC3339OrZero(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

// ---------------------------------------------------------------------------
// Wire types (mirror internal/syncserver — duplicated to avoid an import
// cycle and to keep client/server JSON shapes versioned independently).
// ---------------------------------------------------------------------------

type pullRequest struct {
	Since string `json:"since"`
}

type wordbankItem struct {
	Word       string `json:"word"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	DeletedAt  string `json:"deleted_at,omitempty"`
}

type historyItem struct {
	Word       string `json:"word"`
	Count      int    `json:"count"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	DeletedAt  string `json:"deleted_at,omitempty"`
}

// ---------------------------------------------------------------------------
// In-memory adapters used to feed pulled rows into syncmerge.
// ---------------------------------------------------------------------------

type rowSourceWordbank struct{ rows []store.WordbankRow }

func newRowSourceWordbank(items []wordbankItem) *rowSourceWordbank {
	rows := make([]store.WordbankRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, store.WordbankRow{
			Word:       it.Word,
			CreateTime: it.CreateTime,
			UpdateTime: it.UpdateTime,
			DeletedAt:  it.DeletedAt,
		})
	}
	return &rowSourceWordbank{rows: rows}
}

func (r *rowSourceWordbank) Close() error                                           { return nil }
func (r *rowSourceWordbank) Add(context.Context, string) error                      { return errClientReadOnly }
func (r *rowSourceWordbank) Remove(context.Context, string) error                   { return errClientReadOnly }
func (r *rowSourceWordbank) Contains(context.Context, string) (bool, error)         { return false, errClientReadOnly }
func (r *rowSourceWordbank) List(context.Context) ([]store.WordbankRow, error)      { return r.rows, nil }
func (r *rowSourceWordbank) ListSince(_ context.Context, _ time.Time) ([]store.WordbankRow, error) {
	return r.rows, nil
}
func (r *rowSourceWordbank) ListChanged(_ context.Context, _, _ time.Time) ([]store.WordbankRow, error) {
	return r.rows, nil
}
func (r *rowSourceWordbank) Upsert(context.Context, store.WordbankRow) error { return errClientReadOnly }
func (r *rowSourceWordbank) GCTombstones(context.Context, time.Time) (int, error) {
	return 0, errClientReadOnly
}

type rowSourceHistory struct{ rows []store.HistoryRow }

func newRowSourceHistory(items []historyItem) *rowSourceHistory {
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
	return &rowSourceHistory{rows: rows}
}

func (r *rowSourceHistory) Close() error                                          { return nil }
func (r *rowSourceHistory) Append(context.Context, string) error                  { return errClientReadOnly }
func (r *rowSourceHistory) List(context.Context) ([]store.HistoryRow, error)      { return r.rows, nil }
func (r *rowSourceHistory) ListSince(_ context.Context, _ time.Time) ([]store.HistoryRow, error) {
	return r.rows, nil
}
func (r *rowSourceHistory) ListChanged(_ context.Context, _, _ time.Time) ([]store.HistoryRow, error) {
	return r.rows, nil
}
func (r *rowSourceHistory) Review(_ context.Context, _, _ int) ([]store.HistoryRow, error) {
	return nil, errClientReadOnly
}
func (r *rowSourceHistory) Upsert(context.Context, store.HistoryRow) error { return errClientReadOnly }
func (r *rowSourceHistory) GCTombstones(context.Context, time.Time) (int, error) {
	return 0, errClientReadOnly
}

var errClientReadOnly = errors.New("syncclient: in-memory row source is read-only")
