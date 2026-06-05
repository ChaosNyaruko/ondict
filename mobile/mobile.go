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
	"strconv"
	"strings"
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
	"github.com/ChaosNyaruko/ondict/wordbank"
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

	// Open the local wordbank/history stores up-front and install them as
	// the package singletons. Doing this BEFORE any HTTP handler is
	// registered ensures the shim's lazy-init path never races with
	// ConfigureSync — the sync client will reuse these very *sql.DB
	// handles instead of opening competing ones against the same files.
	if err := ensureSharedStores(); err != nil {
		log.Errorf("mobile: open shared stores: %v", err)
	}

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
	syncMu      sync.Mutex
	syncClient  *syncclient.SyncClient
	syncWB      *store.SQLiteWordbank // shared with the package-level wordbank shim
	syncHistory *store.SQLiteHistory  // shared with the package-level history shim

	// storeOnce serialises the first call to ensureSharedStores across all
	// entry points (StartServer, InitSyncOnly/ConfigureSync). Using a
	// sync.Once rather than relying on the syncMu that wraps ConfigureSync
	// prevents a race between StartServer (which calls ensureSharedStores
	// without syncMu) and a concurrent Worker firing InitSyncOnly (which
	// calls it under syncMu) — both goroutines could pass the
	// syncWB != nil check simultaneously and open two competing *sql.DB
	// handles, defeating the shared-store invariant we set up in the
	// previous round. With Once the shared stores are opened exactly once
	// regardless of which caller arrives first.
	storeOnce    sync.Once
	storeOpenErr error // sticky error from the one-time open attempt
)

// ConfigureSync wires up the sync client. Call this once after StartServer
// (it depends on util.SetPaths having already been called). Empty baseURL,
// username, or password disables sync (returns no error).
//
// The same *sql.DB instances are shared with the wordbank/history package
// shims via SetStore so the WebView's writes and the sync client's reads
// both go through one connection pool — opening separate handles to the
// same SQLite file is a recipe for write-lock contention and stale reads.
//
// gomobile-friendly: only primitive types in the signature.
func ConfigureSync(baseURL, username, password string) error {
	syncMu.Lock()
	defer syncMu.Unlock()
	if baseURL == "" || username == "" || password == "" {
		// Disable: drop the sync client but leave the singleton stores in
		// place so existing WebView writes keep working.
		syncClient = nil
		return nil
	}
	if err := ensureSharedStores(); err != nil {
		return err
	}
	cursors := store.NewCursorStore(syncWB.DB())
	c, err := syncclient.New(syncclient.Config{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
	}, syncWB, syncHistory, cursors)
	if err != nil {
		return err
	}
	syncClient = c
	return nil
}

// ensureSharedStores opens the local wordbank.db / history.db ONCE and
// installs the same instances as both:
//   - the wordbank/history package singletons (so the HTTP query handlers
//     and any other call site keep using them via wordbank.Add / etc.)
//   - the inputs to syncclient.New (so the sync push/pull path reads the
//     same writer-pool the HTTP path writes through).
//
// Serialised by storeOnce — safe to call from StartServer and from
// InitSyncOnly/ConfigureSync concurrently.
func ensureSharedStores() error {
	storeOnce.Do(func() {
		wb, err := store.OpenSQLiteWordbank(util.WordBankDB())
		if err != nil {
			storeOpenErr = fmt.Errorf("open local wordbank: %w", err)
			return
		}
		hist, err := store.OpenSQLiteHistory(util.HistoryDB())
		if err != nil {
			_ = wb.Close()
			storeOpenErr = fmt.Errorf("open local history: %w", err)
			return
		}
		wordbank.SetStore(wb)
		history.SetStore(hist)
		syncWB = wb
		syncHistory = hist
	})
	return storeOpenErr
}

// InitSyncOnly bootstraps only the paths and local SQLite stores needed
// for sync, WITHOUT starting the HTTP server or loading dictionaries.
// Call this from contexts that only need to run a sync cycle (e.g. a
// WorkManager background Worker that was launched into a fresh process
// after the OS killed the app — the HTTP server is not running, so
// StartServer has never been called and util.SetPaths / ConfigureSync
// have never fired).
//
// Callers should pass applicationContext.filesDir and .cacheDir just like
// they would to StartServer. After this call succeeds, Sync() is ready
// to use.
//
// gomobile-friendly: only primitive types in the signature.
func InitSyncOnly(configDir, cacheDir, baseURL, username, password string) error {
	// Mirror the path-setup portion of StartServer so that util.WordBankDB()
	// and util.HistoryDB() resolve to the correct on-device locations.
	util.SetPaths(configDir, cacheDir)

	// The rest is identical to ConfigureSync, which also calls ensureSharedStores.
	return ConfigureSync(baseURL, username, password)
}
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

// IsSyncTransient reports whether an error string returned by Sync() (or
// surfaced through SyncManager.syncOnceBlocking) represents a transient
// failure that is worth retrying (network timeout, server-side 5xx) versus
// a permanent failure that will not resolve without user action (4xx: bad
// credentials, wrong URL, etc.).
//
// The syncclient formats HTTP errors as "<path>: HTTP <code>: <body>"
// (internal/syncclient/client.go:postJSON). We parse the code out of that
// string. Anything in the 4xx range (client error) is permanent; 5xx and
// non-HTTP errors (connection refused, timeout) are transient.
//
// Defaults to true (transient) for unrecognised error formats so that an
// unexpected error class doesn't permanently silence the worker.
//
// gomobile-friendly: only primitive types in the signature.
func IsSyncTransient(errMsg string) bool {
	// Look for "HTTP <code>:" pattern in the error message.
	// strconv.Atoi on the extracted token is more robust than regexp for
	// gomobile's limited stdlib cross-compilation targets.
	idx := strings.Index(errMsg, "HTTP ")
	if idx < 0 {
		// No HTTP status in the message — network-level error (timeout,
		// DNS failure, connection refused). Always transient.
		return true
	}
	rest := errMsg[idx+5:] // skip "HTTP "
	end := strings.IndexByte(rest, ':')
	if end < 0 {
		end = len(rest)
	}
	code, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if err != nil {
		return true // can't parse — be optimistic
	}
	// 4xx = client error → permanent (don't retry).
	// 5xx = server error → transient (retry).
	// Anything else → transient.
	return code < 400 || code >= 500
}
