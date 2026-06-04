// Package mobile provides an entry point for gomobile to start the ondict
// HTTP server on Android.
package mobile

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/history"
	"github.com/ChaosNyaruko/ondict/internal/httpserver"
	"github.com/ChaosNyaruko/ondict/internal/syncclient"
	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/store"
	"github.com/ChaosNyaruko/ondict/util"
)

// StartServer starts the ondict HTTP server on 127.0.0.1:<port>.
//
// configDir should be the app's private files directory
// (e.g. context.getFilesDir().getAbsolutePath() in Kotlin).
// Dictionary files (.mdx/.mdd) are expected under configDir/dicts/.
//
// cacheDir should be the app's cache directory
// (e.g. context.getCacheDir().getAbsolutePath() in Kotlin).
//
// This function blocks; call it in a goroutine from the Android Activity.
func StartServer(configDir, cacheDir string, port int) {
	t0 := time.Now()

	// Write logs to both a file (persistent) and stderr (visible in adb logcat).
	logFile, err := os.OpenFile(
		filepath.Join(cacheDir, "ondict.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666,
	)
	if err == nil {
		log.SetOutput(io.MultiWriter(logFile, os.Stderr))
	} else {
		log.SetOutput(os.Stderr)
	}
	log.SetLevel(log.DebugLevel)

	// Catch any panic so the goroutine doesn't silently die.
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("StartServer panic: %v", r)
		}
	}()

	util.SetPaths(configDir, cacheDir)
	log.Infof("StartServer: configDir=%s cacheDir=%s port=%d", configDir, cacheDir, port)

	gin.SetMode(gin.ReleaseMode)
	// dumpMDD=false: use on-demand MDD extraction via MddFileHandler instead.
	tLoad := time.Now()
	sources.G.Load(true /* iexact */, false /* dumpMDD */, true /* lazy */)
	log.Infof("[timing] G.Load took %v", time.Since(tLoad))

	r := httpserver.New(httpserver.Options{
		// Phase 7 (ADR D9): record history on mobile so it can be synced
		// to a desktop server. We only attach the SQLite writer; the txt
		// log is desktop-only because Android filesystems don't benefit
		// from the human-readable companion.
		History:         history.NewHistory(history.NewSqlite3Writer()),
		EnableAuth:      false, // no auth on mobile
		ResourceHandler: httpserver.MddFileHandler,
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("mobile: listen %s: %v", addr, err)
	}
	log.Infof("[timing] server ready in %v (from StartServer entry)", time.Since(t0))
	log.Infof("mobile: ondict server listening on %s", addr)
	if err := r.RunListener(l); err != nil {
		log.Fatalf("mobile: server exited: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Sync — gomobile-callable bindings.
//
// The Android app is expected to call ConfigureSync once with the server
// origin + Basic Auth credentials and then call Sync() either on a timer or
// when the user pulls-to-refresh. Each call performs a complete pull-then-
// push cycle for both wordbank and history (see ADR 0001 / Phase 6).
// ---------------------------------------------------------------------------

var (
	syncMu     sync.Mutex
	syncClient *syncclient.SyncClient
)

// ConfigureSync wires up the sync client. Call this once after StartServer
// (it depends on util.SetPaths having already been called). Empty baseURL,
// username, or password disables sync (returns no error).
//
// gomobile-friendly: only primitive types in the signature.
func ConfigureSync(baseURL, username, password string) error {
	syncMu.Lock()
	defer syncMu.Unlock()
	if baseURL == "" || username == "" || password == "" {
		syncClient = nil
		return nil
	}
	wb, err := store.OpenSQLiteWordbank(util.WordBankDB())
	if err != nil {
		return fmt.Errorf("open local wordbank: %w", err)
	}
	hist, err := store.OpenSQLiteHistory(util.HistoryDB())
	if err != nil {
		_ = wb.Close()
		return fmt.Errorf("open local history: %w", err)
	}
	cursors := store.NewCursorStore(wb.DB())
	c, err := syncclient.New(syncclient.Config{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
	}, wb, hist, cursors)
	if err != nil {
		_ = wb.Close()
		_ = hist.Close()
		return err
	}
	syncClient = c
	return nil
}

// Sync runs one pull-then-push cycle. Returns a human-readable summary
// string on success, suitable for showing in a Toast or logging in adb.
// Returns the underlying error string verbatim on failure (gomobile cannot
// marshal Go errors directly; we return it as a string).
func Sync() (string, error) {
	syncMu.Lock()
	c := syncClient
	syncMu.Unlock()
	if c == nil {
		return "", fmt.Errorf("sync not configured: call ConfigureSync first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stats, err := c.Sync(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"sync ok: wordbank pull(ins=%d upd=%d) push=%d; history pull(ins=%d upd=%d) push=%d",
		stats.WordbankPullApplied.Inserted, stats.WordbankPullApplied.Updated, stats.WordbankPushed,
		stats.HistoryPullApplied.Inserted, stats.HistoryPullApplied.Updated, stats.HistoryPushed,
	), nil
}
