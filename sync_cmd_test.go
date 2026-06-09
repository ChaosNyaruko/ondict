package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunSync_MissingBaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	rc := runSync([]string{}, &stdout, &stderr)
	require.NotEqual(t, 0, rc)
}

func TestRunSync_MissingCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONDICT_SYNC_USER", "")
	t.Setenv("ONDICT_SYNC_PASSWORD", "")
	var stdout, stderr bytes.Buffer
	rc := runSync([]string{"--base-url", "http://localhost:9999"}, &stdout, &stderr)
	require.NotEqual(t, 0, rc)
}

func TestRunSync_InvalidFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	rc := runSync([]string{"--unknown-flag"}, &stdout, &stderr)
	require.NotEqual(t, 0, rc)
}

func TestRunSync_WithCredentials_ServerNotReachable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONDICT_SYNC_USER", "alice")
	t.Setenv("ONDICT_SYNC_PASSWORD", "secret")
	var stdout, stderr bytes.Buffer
	// base-url points to a non-existent server → fails with a network error.
	rc := runSync([]string{
		"--base-url", "http://127.0.0.1:19999",
		"--user", "alice",
		"--timeout", "1s",
	}, &stdout, &stderr)
	require.NotEqual(t, 0, rc)
}

func TestRunSync_OnlyUser_NoPassword(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONDICT_SYNC_USER", "")
	t.Setenv("ONDICT_SYNC_PASSWORD", "")
	var stdout, stderr bytes.Buffer
	// --user is provided but password env is empty → still fails.
	rc := runSync([]string{"--base-url", "http://localhost", "--user", "alice"}, &stdout, &stderr)
	require.NotEqual(t, 0, rc)
}
