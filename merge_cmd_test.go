package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/store"
)

func seedWordbank(t *testing.T, path string, rows ...store.WordbankRow) {
	t.Helper()
	wb, err := store.OpenSQLiteWordbank(path)
	require.NoError(t, err)
	defer wb.Close()
	ctx := context.Background()
	for _, r := range rows {
		require.NoError(t, wb.Upsert(ctx, r))
	}
}

func seedHistory(t *testing.T, path string, rows ...store.HistoryRow) {
	t.Helper()
	h, err := store.OpenSQLiteHistory(path)
	require.NoError(t, err)
	defer h.Close()
	ctx := context.Background()
	for _, r := range rows {
		require.NoError(t, h.Upsert(ctx, r))
	}
}

func TestRunMerge_WordbankE2E(t *testing.T) {
	tmp := t.TempDir()
	src1 := filepath.Join(tmp, "src1.db")
	src2 := filepath.Join(tmp, "src2.db")
	dst := filepath.Join(tmp, "dst.db")

	seedWordbank(t, src1, store.WordbankRow{
		Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	})
	seedWordbank(t, src2, store.WordbankRow{
		Word: "beta", CreateTime: "2024-02-01T00:00:00Z", UpdateTime: "2024-07-01T00:00:00Z",
	})

	var stdout, stderr bytes.Buffer
	rc := runMerge([]string{"wordbank", "--dst", dst, src1, src2}, &stdout, &stderr)
	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "TOTAL")
	require.Contains(t, stdout.String(), "inserted=2")

	wb, err := store.OpenSQLiteWordbank(dst)
	require.NoError(t, err)
	defer wb.Close()
	rows, err := wb.List(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestRunMerge_HistoryCountIsMAX(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.db")
	dst := filepath.Join(tmp, "dst.db")

	seedHistory(t, dst, store.HistoryRow{
		Word: "doctor", Count: 5, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	})
	seedHistory(t, src, store.HistoryRow{
		Word: "doctor", Count: 9, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	})

	var stdout, stderr bytes.Buffer
	rc := runMerge([]string{"history", "--dst", dst, src}, &stdout, &stderr)
	require.Equal(t, 0, rc, "stderr=%s", stderr.String())

	h, err := store.OpenSQLiteHistory(dst)
	require.NoError(t, err)
	defer h.Close()
	rows, err := h.List(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 9, rows[0].Count, "count must be MAX after merge (ADR D3)")
}

func TestRunMerge_DryRunDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.db")
	dst := filepath.Join(tmp, "dst.db")

	seedWordbank(t, src, store.WordbankRow{
		Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	})

	var stdout, stderr bytes.Buffer
	rc := runMerge([]string{"wordbank", "--dst", dst, "--dry-run", src}, &stdout, &stderr)
	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "dry-run=true")

	wb, err := store.OpenSQLiteWordbank(dst)
	require.NoError(t, err)
	defer wb.Close()
	rows, err := wb.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, rows, "dry-run must not persist any rows")
}

func TestRunMerge_BadArgs(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "dst.db")

	cases := []struct {
		name string
		argv []string
	}{
		{"no kind", []string{}},
		{"bad kind", []string{"galaxy", "--dst", dst, "x.db"}},
		{"no --dst", []string{"wordbank", "src.db"}},
		{"no sources", []string{"wordbank", "--dst", dst}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rc := runMerge(tc.argv, &stdout, &stderr)
			require.NotEqual(t, 0, rc)
			require.True(t, strings.Contains(stderr.String(), "usage") ||
				strings.Contains(stderr.String(), "missing") ||
				strings.Contains(stderr.String(), "no source") ||
				strings.Contains(stderr.String(), "unknown") ||
				strings.Contains(stderr.String(), "flag provided"),
				"stderr should explain the problem; got: %s", stderr.String())
		})
	}
}

func TestRunMerge_DryRunHistoryDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.db")
	dst := filepath.Join(tmp, "dst.db")

	seedHistory(t, src, store.HistoryRow{
		Word: "cherry", Count: 1, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	})

	var stdout, stderr bytes.Buffer
	rc := runMerge([]string{"history", "--dst", dst, "--dry-run", src}, &stdout, &stderr)
	require.Equal(t, 0, rc, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "dry-run=true")

	h, err := store.OpenSQLiteHistory(dst)
	require.NoError(t, err)
	defer h.Close()
	rows, err := h.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, rows, "dry-run must not persist any rows")
}

func TestRunMerge_WordbankBadSource(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "dst.db")

	var stdout, stderr bytes.Buffer
	rc := runMerge([]string{"wordbank", "--dst", dst, "/nonexistent/source.db"}, &stdout, &stderr)
	require.NotEqual(t, 0, rc)
}

func TestRunMerge_HistoryBadSource(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "dst.db")

	var stdout, stderr bytes.Buffer
	rc := runMerge([]string{"history", "--dst", dst, "/nonexistent/source.db"}, &stdout, &stderr)
	require.NotEqual(t, 0, rc)
}
