package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ChaosNyaruko/ondict/store"
	"github.com/ChaosNyaruko/ondict/syncmerge"
)

// runMerge implements the `ondict merge <wordbank|history>` subcommand. It
// is dispatched directly from main() before flag.Parse() so the subcommand's
// flag set is independent of the top-level flags.
//
// CLI shape (see ADR 0001 / D8):
//
//	ondict merge wordbank --dst out.db src1.db src2.db [--dry-run]
//	ondict merge history  --dst out.db src1.db src2.db [--dry-run]
//
// On success, summary stats are printed to stdout. The destination database
// is created (and migrated to the latest schema) if it does not already
// exist; existing destinations are merged into in place.
func runMerge(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, mergeUsage)
		return 2
	}
	kind := args[0]
	rest := args[1:]

	fs := flag.NewFlagSet("merge "+kind, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dst := fs.String("dst", "", "destination DB file (created if absent)")
	dryRun := fs.Bool("dry-run", false, "open the sources but do not write to the destination")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	if *dst == "" {
		fmt.Fprintln(stderr, "missing required flag --dst")
		fmt.Fprintln(stderr, mergeUsage)
		return 2
	}
	srcs := fs.Args()
	if len(srcs) == 0 {
		fmt.Fprintln(stderr, "no source DB files supplied")
		fmt.Fprintln(stderr, mergeUsage)
		return 2
	}

	ctx := context.Background()
	switch kind {
	case "wordbank":
		return runMergeWordbank(ctx, *dst, srcs, *dryRun, stdout, stderr)
	case "history":
		return runMergeHistory(ctx, *dst, srcs, *dryRun, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown merge kind %q (want wordbank or history)\n", kind)
		fmt.Fprintln(stderr, mergeUsage)
		return 2
	}
}

const mergeUsage = `usage:
  ondict merge wordbank --dst out.db src1.db [src2.db ...] [--dry-run]
  ondict merge history  --dst out.db src1.db [src2.db ...] [--dry-run]`

func runMergeWordbank(ctx context.Context, dstPath string, srcPaths []string, dryRun bool, stdout, stderr io.Writer) int {
	dstStore, err := openMergeWordbankDst(dstPath, dryRun)
	if err != nil {
		fmt.Fprintf(stderr, "open destination %q: %v\n", dstPath, err)
		return 1
	}
	defer dstStore.Close()

	var total syncmerge.Stats
	for _, sp := range srcPaths {
		src, err := store.OpenSQLiteWordbank(sp)
		if err != nil {
			fmt.Fprintf(stderr, "open source %q: %v\n", sp, err)
			return 1
		}
		stats, err := syncmerge.MergeWordbank(ctx, dstStore, src)
		_ = src.Close()
		if err != nil {
			fmt.Fprintf(stderr, "merge wordbank %q: %v\n", sp, err)
			return 1
		}
		fmt.Fprintf(stdout, "[wordbank] %s: inserted=%d updated=%d unchanged=%d tombstones=%d\n",
			sp, stats.Inserted, stats.Updated, stats.Unchanged, stats.TombstonesPropagated)
		total = total.Add(stats)
	}
	fmt.Fprintf(stdout, "[wordbank] TOTAL  -> dst=%s inserted=%d updated=%d unchanged=%d tombstones=%d (dry-run=%v)\n",
		dstPath, total.Inserted, total.Updated, total.Unchanged, total.TombstonesPropagated, dryRun)
	return 0
}

func runMergeHistory(ctx context.Context, dstPath string, srcPaths []string, dryRun bool, stdout, stderr io.Writer) int {
	dstStore, err := openMergeHistoryDst(dstPath, dryRun)
	if err != nil {
		fmt.Fprintf(stderr, "open destination %q: %v\n", dstPath, err)
		return 1
	}
	defer dstStore.Close()

	var total syncmerge.Stats
	for _, sp := range srcPaths {
		src, err := store.OpenSQLiteHistory(sp)
		if err != nil {
			fmt.Fprintf(stderr, "open source %q: %v\n", sp, err)
			return 1
		}
		stats, err := syncmerge.MergeHistory(ctx, dstStore, src)
		_ = src.Close()
		if err != nil {
			fmt.Fprintf(stderr, "merge history %q: %v\n", sp, err)
			return 1
		}
		fmt.Fprintf(stdout, "[history]  %s: inserted=%d updated=%d unchanged=%d tombstones=%d\n",
			sp, stats.Inserted, stats.Updated, stats.Unchanged, stats.TombstonesPropagated)
		total = total.Add(stats)
	}
	fmt.Fprintf(stdout, "[history]  TOTAL  -> dst=%s inserted=%d updated=%d unchanged=%d tombstones=%d (dry-run=%v)\n",
		dstPath, total.Inserted, total.Updated, total.Unchanged, total.TombstonesPropagated, dryRun)
	return 0
}

// openMergeWordbankDst returns a store.WordbankStore. In dry-run mode it
// returns a wrapper that drops every Upsert; the caller still sees
// per-source stats so they can preview the operation.
func openMergeWordbankDst(path string, dryRun bool) (store.WordbankStore, error) {
	st, err := store.OpenSQLiteWordbank(path)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return &readOnlyWordbank{WordbankStore: st}, nil
	}
	return st, nil
}

func openMergeHistoryDst(path string, dryRun bool) (store.HistoryStore, error) {
	st, err := store.OpenSQLiteHistory(path)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return &readOnlyHistory{HistoryStore: st}, nil
	}
	return st, nil
}

// readOnlyWordbank rejects every write so a --dry-run merge can still
// compute Inserted/Updated/Unchanged stats from the merge engine without
// mutating the destination DB.
type readOnlyWordbank struct{ store.WordbankStore }

func (*readOnlyWordbank) Upsert(_ context.Context, _ store.WordbankRow) error { return nil }

type readOnlyHistory struct{ store.HistoryStore }

func (*readOnlyHistory) Upsert(_ context.Context, _ store.HistoryRow) error { return nil }

// silence unused-import warning if the file is ever compiled without main_test.
var _ = os.Args
