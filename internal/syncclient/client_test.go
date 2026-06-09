package syncclient

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/internal/syncserver"
	"github.com/ChaosNyaruko/ondict/store"
)

func init() { gin.SetMode(gin.TestMode) }

// setupServerAndClient stands up a real syncserver.SyncServer + httptest
// server and constructs a SyncClient targeted at it backed by fresh local
// SQLite stores. This mirrors what the Android app and desktop daemon will
// do at runtime.
func setupServerAndClient(t *testing.T) (*SyncClient, *store.SQLiteWordbank, *store.SQLiteHistory, func()) {
	t.Helper()

	srv, err := syncserver.New(syncserver.Config{
		DataDir:  filepath.Join(t.TempDir(), "server"),
		Username: "alice",
		Password: "s3cret",
	})
	require.NoError(t, err)
	r := gin.New()
	srv.Mount(r)
	httpServer := httptest.NewServer(r)

	wbDir := filepath.Join(t.TempDir(), "client")
	wb, err := store.OpenSQLiteWordbank(filepath.Join(wbDir, "wordbank.db"))
	require.NoError(t, err)
	h, err := store.OpenSQLiteHistory(filepath.Join(wbDir, "history.db"))
	require.NoError(t, err)
	cursors := store.NewCursorStore(wb.DB())

	client, err := New(Config{
		BaseURL:  httpServer.URL,
		Username: "alice",
		Password: "s3cret",
	}, wb, h, cursors)
	require.NoError(t, err)

	cleanup := func() {
		_ = wb.Close()
		_ = h.Close()
		_ = srv.Close()
		httpServer.Close()
	}
	return client, wb, h, cleanup
}

func TestSyncClient_PushThenPullRoundtripsViaServer(t *testing.T) {
	ctx := context.Background()
	client, wb, h, cleanup := setupServerAndClient(t)
	defer cleanup()

	// Seed local stores with one wordbank and one history row each.
	require.NoError(t, wb.Upsert(ctx, store.WordbankRow{
		Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))
	require.NoError(t, h.Upsert(ctx, store.HistoryRow{
		Word: "doctor", Count: 4, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))

	// First sync: nothing pulled (server is empty), one of each pushed.
	stats, err := client.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, stats.WordbankPushed)
	require.Equal(t, 1, stats.HistoryPushed)

	// Second sync: nothing to push (cursors moved), nothing to pull.
	stats, err = client.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, stats.WordbankPushed)
	require.Equal(t, 0, stats.HistoryPushed)
}

func TestSyncClient_TwoClientsSharedServer(t *testing.T) {
	ctx := context.Background()

	// Shared server.
	srv, err := syncserver.New(syncserver.Config{
		DataDir:  filepath.Join(t.TempDir(), "server"),
		Username: "alice",
		Password: "s3cret",
	})
	require.NoError(t, err)
	defer srv.Close()
	r := gin.New()
	srv.Mount(r)
	httpServer := httptest.NewServer(r)
	defer httpServer.Close()

	// Two independent clients.
	openClient := func(prefix string) (*SyncClient, *store.SQLiteWordbank, *store.SQLiteHistory) {
		dir := filepath.Join(t.TempDir(), prefix)
		wb, err := store.OpenSQLiteWordbank(filepath.Join(dir, "wordbank.db"))
		require.NoError(t, err)
		h, err := store.OpenSQLiteHistory(filepath.Join(dir, "history.db"))
		require.NoError(t, err)
		c, err := New(Config{
			BaseURL:  httpServer.URL,
			Username: "alice",
			Password: "s3cret",
		}, wb, h, store.NewCursorStore(wb.DB()))
		require.NoError(t, err)
		return c, wb, h
	}

	c1, wb1, _ := openClient("device1")
	c2, wb2, _ := openClient("device2")

	require.NoError(t, wb1.Upsert(ctx, store.WordbankRow{
		Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))
	require.NoError(t, wb2.Upsert(ctx, store.WordbankRow{
		Word: "beta", CreateTime: "2024-02-01T00:00:00Z", UpdateTime: "2024-07-01T00:00:00Z",
	}))

	// Each client syncs once.
	_, err = c1.Sync(ctx)
	require.NoError(t, err)
	_, err = c2.Sync(ctx)
	require.NoError(t, err)

	// device1 syncs again — must now see beta.
	stats, err := c1.Sync(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.WordbankPullApplied.Inserted, 1, "device1 must pull beta from device2 via server")

	rows1, err := wb1.List(ctx)
	require.NoError(t, err)
	gotWords := map[string]bool{}
	for _, r := range rows1 {
		gotWords[r.Word] = true
	}
	require.True(t, gotWords["alpha"])
	require.True(t, gotWords["beta"], "device1 must now contain beta after second sync")
}

// Regression for code-review P1a: when a row carries a future update_time
// (e.g. arrived from another device with a wildly skewed clock, or just
// because the user's clock was set), the push cursor must still advance
// based on local server_seen_at. Otherwise the local push cursor would
// jump into the future and permanently skip subsequent local writes.
func TestSyncClient_PushCursorIgnoresFutureUpdateTime(t *testing.T) {
	ctx := context.Background()
	client, wb, _, cleanup := setupServerAndClient(t)
	defer cleanup()

	// Inject a row with update_time far in the future (year 2099). The
	// SeenAt column is still set to "now" because Upsert always stamps it
	// to wall-clock time on the local DB.
	require.NoError(t, wb.Upsert(ctx, store.WordbankRow{
		Word:       "future",
		CreateTime: "2099-01-01T00:00:00Z",
		UpdateTime: "2099-01-01T00:00:00Z",
	}))
	stats, err := client.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, stats.WordbankPushed, "future row must still be pushed")

	// Second sync — nothing new locally, so push count must be 0. With the
	// pre-fix code the cursor would now be "2099-..." which is fine; but
	// the *real* failure mode is the next case.
	stats, err = client.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, stats.WordbankPushed)

	// Add a normal row whose update_time is "now" (in 2026). With the old
	// cursor-by-update_time code, "now" < "2099" → row would be SKIPPED
	// and never pushed. The fix advances the cursor by SeenAt (always
	// monotonic local wall-clock), so the new row IS pushed.
	require.NoError(t, wb.Add(ctx, "today"))
	stats, err = client.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, stats.WordbankPushed, "new local writes must keep being pushed even after a future-dated row")
}

// TestSyncClient_SwitchServerFullPush verifies that switching to a different
// sync server (different BaseURL) triggers a full push of all local rows,
// even though a previous sync with the old server already advanced the push
// cursor. Cursor keys are namespaced by server URL (FNV-32 hash), so the
// new server's cursor starts empty and ListSince returns everything.
func TestSyncClient_SwitchServerFullPush(t *testing.T) {
	ctx := context.Background()

	// ── Old server ──────────────────────────────────────────────────────────
	oldSrv, err := syncserver.New(syncserver.Config{
		DataDir:  filepath.Join(t.TempDir(), "old-server"),
		Username: "alice",
		Password: "s3cret",
	})
	require.NoError(t, err)
	defer oldSrv.Close()
	oldR := gin.New()
	oldSrv.Mount(oldR)
	oldHTTP := httptest.NewServer(oldR)
	defer oldHTTP.Close()

	// ── New (empty) server ───────────────────────────────────────────────────
	newSrv, err := syncserver.New(syncserver.Config{
		DataDir:  filepath.Join(t.TempDir(), "new-server"),
		Username: "alice",
		Password: "s3cret",
	})
	require.NoError(t, err)
	defer newSrv.Close()
	newR := gin.New()
	newSrv.Mount(newR)
	newHTTP := httptest.NewServer(newR)
	defer newHTTP.Close()

	// ── Client local stores ──────────────────────────────────────────────────
	dir := t.TempDir()
	wb, err := store.OpenSQLiteWordbank(filepath.Join(dir, "wordbank.db"))
	require.NoError(t, err)
	defer wb.Close()
	h, err := store.OpenSQLiteHistory(filepath.Join(dir, "history.db"))
	require.NoError(t, err)
	defer h.Close()
	cursors := store.NewCursorStore(wb.DB())

	// Seed two wordbank rows.
	require.NoError(t, wb.Upsert(ctx, store.WordbankRow{
		Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))
	require.NoError(t, wb.Upsert(ctx, store.WordbankRow{
		Word: "beta", CreateTime: "2024-02-01T00:00:00Z", UpdateTime: "2024-07-01T00:00:00Z",
	}))

	// ── Sync with OLD server ─────────────────────────────────────────────────
	oldClient, err := New(Config{
		BaseURL:  oldHTTP.URL,
		Username: "alice",
		Password: "s3cret",
	}, wb, h, cursors)
	require.NoError(t, err)

	stats, err := oldClient.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, stats.WordbankPushed, "both rows must be pushed to old server")

	// Verify old server has the rows.
	stats, err = oldClient.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, stats.WordbankPushed, "second sync to old server must push nothing (cursor advanced)")

	// ── Switch to NEW server ─────────────────────────────────────────────────
	newClient, err := New(Config{
		BaseURL:  newHTTP.URL,
		Username: "alice",
		Password: "s3cret",
	}, wb, h, cursors)
	require.NoError(t, err)

	// New server is empty; new cursor is also empty → full push expected.
	stats, err = newClient.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, stats.WordbankPushed,
		"switching to a new server must trigger a full push of all local rows "+
			"(per-server cursor starts empty)")

	// Second sync to new server must be incremental (nothing new locally).
	stats, err = newClient.Sync(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, stats.WordbankPushed, "second sync to new server must push nothing")
}

// Regression for code-review (round 2) P2: a legacy second-resolution
// cursor written by an earlier version of the client (e.g. "...:00Z")
// must not stall the cursor's advance once millisecond-resolution
// server_seen_at values arrive. The bug is that ASCII '.' (0x2E) <
// 'Z' (0x5A), so a millisecond row "...:NN.fffZ" sorts BEFORE the
// legacy "...:NNZ" lexically, and `r.SeenAt > maxSeen` is false even
// when the row's instant is at or after the cursor's intended instant.
// The fix normalises the legacy cursor to "...:NN.000Z" before any
// string comparison.
func TestSyncClient_PushCursorNormalizesLegacySecondResolutionCursor(t *testing.T) {
	ctx := context.Background()
	client, wb, _, cleanup := setupServerAndClient(t)
	defer cleanup()

	require.NoError(t, wb.Add(ctx, "alpha"))

	// Plant a hand-crafted second-resolution cursor matching the same
	// wall-clock second as the row's server_seen_at. This is the shape
	// the previous client version (using time.RFC3339, second-resolution)
	// would have written.
	rows, err := wb.ListSince(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Contains(t, rows[0].SeenAt, ".", "row SeenAt must be in ms format for the test premise")
	legacySecondCursor := rows[0].SeenAt[:19] + "Z" // "...:NN.fffZ" → "...:NNZ"

	require.NoError(t, client.meta.SetCursor(ctx, client.cursorWordbankPush(), legacySecondCursor))

	// Sync. After fix, the cursor is normalised before comparison so the
	// row's ms SeenAt is correctly recognised as ordered relative to the
	// cursor; maxSeen ends up at the row's ms value. Before fix,
	// `r.SeenAt > maxSeen` (where maxSeen is still the raw legacy
	// "...:NNZ") is FALSE for same-second writes, so maxSeen never
	// advances and the cursor stays at the legacy form forever.
	_, err = client.Sync(ctx)
	require.NoError(t, err)

	final, err := client.meta.GetCursor(ctx, client.cursorWordbankPush())
	require.NoError(t, err)
	require.Contains(t, final, ".", "cursor must be in millisecond layout after sync (got %q)", final)
	require.NotEqual(t, legacySecondCursor, final, "cursor must have advanced past the legacy second-resolution value")
}
