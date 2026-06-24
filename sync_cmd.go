package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ChaosNyaruko/ondict/internal/syncclient"
	"github.com/ChaosNyaruko/ondict/store"
	"github.com/ChaosNyaruko/ondict/util"
)

// runSync implements the `ondict sync` subcommand — a one-shot client-side
// sync run intended for cron jobs or manual desktop refreshes. Mobile uses
// the gomobile bindings in package mobile instead; the protocol and ADR
// references are the same (see docs/adr/0001-wordbank-history-sync.md).
//
//	ondict sync --base-url https://sync.example.com [--user alice]
//	            [--loop 10m]                # keep running, sync every interval
//	            [--gc-tombstones-after 90d] # purge tombstones older than that
//
// Credentials default to the ONDICT_SYNC_USER / ONDICT_SYNC_PASSWORD env
// vars (matching the server side). --user overrides the env username; the
// password is always read from ONDICT_SYNC_PASSWORD so it never enters
// shell history.
func runSync(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "", "sync server origin (required), e.g. https://sync.example.com")
	user := fs.String("user", "", "username (defaults to $ONDICT_SYNC_USER)")
	timeout := fs.Duration("timeout", 10*time.Minute, "overall timeout for one sync round trip (default generous for first-time full sync)")
	loop := fs.Duration("loop", 0, "if >0, run as a daemon, syncing on this interval")
	gcAfter := fs.Duration("gc-tombstones-after", 0, "purge tombstones older than this each cycle (0 disables)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *baseURL == "" {
		fmt.Fprintln(stderr, "missing required flag --base-url")
		return 2
	}
	if *user == "" {
		*user = os.Getenv("ONDICT_SYNC_USER")
	}
	pass := os.Getenv("ONDICT_SYNC_PASSWORD")
	if *user == "" || pass == "" {
		fmt.Fprintln(stderr, "username/password missing: set --user and ONDICT_SYNC_PASSWORD env var")
		return 2
	}

	wb, err := store.OpenSQLiteWordbank(util.WordBankDB())
	if err != nil {
		fmt.Fprintf(stderr, "open local wordbank: %v\n", err)
		return 1
	}
	defer wb.Close()
	hist, err := store.OpenSQLiteHistory(util.HistoryDB())
	if err != nil {
		fmt.Fprintf(stderr, "open local history: %v\n", err)
		return 1
	}
	defer hist.Close()
	cursors := store.NewCursorStore(wb.DB())

	client, err := syncclient.New(syncclient.Config{
		BaseURL:  *baseURL,
		Username: *user,
		Password: pass,
	}, wb, hist, cursors)
	if err != nil {
		fmt.Fprintf(stderr, "configure sync client: %v\n", err)
		return 1
	}

	once := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		stats, err := client.Sync(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout,
			"sync ok: wordbank pull(ins=%d upd=%d unc=%d) push=%d; history pull(ins=%d upd=%d unc=%d) push=%d\n",
			stats.WordbankPullApplied.Inserted, stats.WordbankPullApplied.Updated, stats.WordbankPullApplied.Unchanged, stats.WordbankPushed,
			stats.HistoryPullApplied.Inserted, stats.HistoryPullApplied.Updated, stats.HistoryPullApplied.Unchanged, stats.HistoryPushed,
		)
		if *gcAfter > 0 {
			before := time.Now().UTC().Add(-*gcAfter)
			wbGC, _ := wb.GCTombstones(ctx, before)
			hGC, _ := hist.GCTombstones(ctx, before)
			if wbGC > 0 || hGC > 0 {
				fmt.Fprintf(stdout, "gc: wordbank=%d history=%d (before=%s)\n", wbGC, hGC, before.Format(time.RFC3339))
			}
		}
		return nil
	}

	if *loop <= 0 {
		if err := once(); err != nil {
			fmt.Fprintf(stderr, "sync failed: %v\n", err)
			return 1
		}
		return 0
	}

	// Daemon mode: run once immediately then on every tick. Errors are
	// logged and the loop continues; the operator restarts the process to
	// recover from persistent failures.
	t := time.NewTicker(*loop)
	defer t.Stop()
	if err := once(); err != nil {
		fmt.Fprintf(stderr, "sync failed: %v\n", err)
	}
	for range t.C {
		if err := once(); err != nil {
			fmt.Fprintf(stderr, "sync failed: %v\n", err)
		}
	}
	return 0
}
