package syncserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestServer(t *testing.T) (*SyncServer, *gin.Engine) {
	t.Helper()
	srv, err := New(Config{
		DataDir:  filepath.Join(t.TempDir(), "sync"),
		Username: "alice",
		Password: "s3cret",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Close() })
	r := gin.New()
	srv.Mount(r)
	return srv, r
}

func doJSON(t *testing.T, r *gin.Engine, method, path, user, pass string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		rd = bytes.NewReader(raw)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSyncServer_AuthRequired(t *testing.T) {
	_, r := newTestServer(t)

	w := doJSON(t, r, "GET", "/sync/v1/state", "", "", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.NotEmpty(t, w.Header().Get("WWW-Authenticate"))

	w = doJSON(t, r, "GET", "/sync/v1/state", "alice", "wrong", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = doJSON(t, r, "GET", "/sync/v1/state", "alice", "s3cret", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp stateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, SchemaVersion, resp.SchemaVersion)
	require.NotEmpty(t, resp.ServerNow)
}

func TestSyncServer_WordbankRoundtrip(t *testing.T) {
	_, r := newTestServer(t)

	// Push two items.
	push := wordbankPushRequest{Items: []wordbankItem{
		{Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
		{Word: "beta", CreateTime: "2024-02-01T00:00:00Z", UpdateTime: "2024-07-01T00:00:00Z"},
	}}
	w := doJSON(t, r, "POST", "/sync/v1/wordbank/push", "alice", "s3cret", push)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var pushResp wordbankPushResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pushResp))
	require.Equal(t, 2, pushResp.Applied)

	// Pull everything (since: "").
	w = doJSON(t, r, "POST", "/sync/v1/wordbank/pull", "alice", "s3cret", pullRequest{})
	require.Equal(t, http.StatusOK, w.Code)
	var pullResp wordbankPullResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pullResp))
	require.Len(t, pullResp.Items, 2)

	// Pull from a future timestamp: nothing should come back. ListSince
	// filters by server_seen_at (local-write wall clock), not the
	// user-content update_time, so we use a "now"-relative cutoff.
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	w = doJSON(t, r, "POST", "/sync/v1/wordbank/pull", "alice", "s3cret", pullRequest{
		Since: future,
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pullResp))
	require.Empty(t, pullResp.Items)
}

func TestSyncServer_HistoryCountIsMAX(t *testing.T) {
	_, r := newTestServer(t)

	// Push count=5.
	push := historyPushRequest{Items: []historyItem{
		{Word: "doctor", Count: 5, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
	}}
	w := doJSON(t, r, "POST", "/sync/v1/history/push", "alice", "s3cret", push)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Push count=9 — MAX should win even though update_time is the same.
	push.Items[0].Count = 9
	w = doJSON(t, r, "POST", "/sync/v1/history/push", "alice", "s3cret", push)
	require.Equal(t, http.StatusOK, w.Code)

	// Pull and verify count=9.
	w = doJSON(t, r, "POST", "/sync/v1/history/pull", "alice", "s3cret", pullRequest{})
	require.Equal(t, http.StatusOK, w.Code)
	var pull historyPullResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pull))
	require.Len(t, pull.Items, 1)
	require.Equal(t, 9, pull.Items[0].Count)
}

func TestSyncServer_TombstonePropagatesViaPushPull(t *testing.T) {
	_, r := newTestServer(t)

	// Initial add.
	push := wordbankPushRequest{Items: []wordbankItem{
		{Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
	}}
	w := doJSON(t, r, "POST", "/sync/v1/wordbank/push", "alice", "s3cret", push)
	require.Equal(t, http.StatusOK, w.Code)

	// Tombstone push.
	push.Items[0].UpdateTime = "2024-12-01T00:00:00Z"
	push.Items[0].DeletedAt = "2024-12-01T00:00:00Z"
	w = doJSON(t, r, "POST", "/sync/v1/wordbank/push", "alice", "s3cret", push)
	require.Equal(t, http.StatusOK, w.Code)

	// Verify: a fresh client pulling sees the tombstone.
	w = doJSON(t, r, "POST", "/sync/v1/wordbank/pull", "alice", "s3cret", pullRequest{})
	require.Equal(t, http.StatusOK, w.Code)
	var pull wordbankPullResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pull))
	require.Len(t, pull.Items, 1)
	require.Equal(t, "2024-12-01T00:00:00Z", pull.Items[0].DeletedAt)
}

func TestSyncServer_PerUserIsolation(t *testing.T) {
	srv, err := New(Config{
		DataDir:  filepath.Join(t.TempDir(), "sync"),
		Username: "alice",
		Password: "s3cret",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Close() })

	// Manually seed a different user's DB to prove isolation.
	bobDir := filepath.Join(srv.cfg.DataDir, "bob")
	require.NoError(t, os.MkdirAll(bobDir, 0o755))
	otherWB, err := store.OpenSQLiteWordbank(filepath.Join(bobDir, "wordbank.db"))
	require.NoError(t, err)
	require.NoError(t, otherWB.Upsert(context.Background(), store.WordbankRow{
		Word: "bobs-word", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-01-01T00:00:00Z",
	}))
	require.NoError(t, otherWB.Close())

	// Alice cannot see Bob's data.
	r := gin.New()
	srv.Mount(r)
	w := doJSON(t, r, "POST", "/sync/v1/wordbank/pull", "alice", "s3cret", pullRequest{})
	require.Equal(t, http.StatusOK, w.Code)
	var pull wordbankPullResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pull))
	require.Empty(t, pull.Items)
}

func TestSyncServer_RejectsBadConfig(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)

	_, err = New(Config{DataDir: t.TempDir()})
	require.Error(t, err, "must require Username/Password")
}

func TestSyncServer_RejectsBadSinceTimestamp(t *testing.T) {
	_, r := newTestServer(t)
	w := doJSON(t, r, "POST", "/sync/v1/wordbank/pull", "alice", "s3cret", pullRequest{Since: "not-a-time"})
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// Regression for code-review P1b: pull must capture its cutoff BEFORE
// reading the rows, so a row written between scan-end and server_now-stamp
// surfaces on the next pull rather than being silently skipped.
//
// The race the reviewer flagged is internal to a single pull (scan, then
// nowMs), but the externally observable invariant is: any push that
// commits AFTER server_now must be visible on the NEXT pull whose Since
// equals that server_now. We verify the invariant directly by separating
// the operations with enough wall-clock delta (>1ms) to be unambiguous in
// our millisecond-resolution timestamp space.
func TestSyncServer_PullCaptureCutoffBeforeQuery(t *testing.T) {
	_, r := newTestServer(t)

	// Step 1: baseline pull, capture server_now.
	w := doJSON(t, r, "POST", "/sync/v1/wordbank/pull", "alice", "s3cret", pullRequest{})
	require.Equal(t, http.StatusOK, w.Code)
	var base wordbankPullResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &base))

	// Ensure the next write lands at a strictly-greater millisecond.
	time.Sleep(2 * time.Millisecond)

	// Step 2: push a row.
	push := wordbankPushRequest{Items: []wordbankItem{
		{Word: "raceword", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
	}}
	w = doJSON(t, r, "POST", "/sync/v1/wordbank/push", "alice", "s3cret", push)
	require.Equal(t, http.StatusOK, w.Code)

	// Step 3: pull with the baseline cursor — must include raceword.
	w = doJSON(t, r, "POST", "/sync/v1/wordbank/pull", "alice", "s3cret", pullRequest{Since: base.ServerNow})
	require.Equal(t, http.StatusOK, w.Code)
	var delta wordbankPullResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &delta))
	found := false
	for _, it := range delta.Items {
		if it.Word == "raceword" {
			found = true
			break
		}
	}
	require.True(t, found, "row pushed after a pull must be visible on the next pull (P1 race)")
}
