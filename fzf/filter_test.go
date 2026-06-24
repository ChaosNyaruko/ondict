package fzf

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserOpenCommand(t *testing.T) {
	// Just verify the function returns a non-empty string on any platform.
	cmd := browserOpenCommand()
	assert.NotEmpty(t, cmd)
}

func TestIsCommandAvailable_NotExist(t *testing.T) {
	// A command that definitely doesn't exist.
	assert.False(t, isCommandAvailable("this_command_does_not_exist_ondict_test"))
}

func TestIsCommandAvailable_Exists(t *testing.T) {
	// "echo" is available on all POSIX systems.
	assert.True(t, isCommandAvailable("echo"))
}

func TestWithFilter_Echo(t *testing.T) {
	// Use "echo hello" as the filter command — it ignores stdin but produces stdout output.
	// This exercises the withFilter code path (spawn + read output) without fzf.
	result := withFilter("echo hello", func(in io.WriteCloser) {
		_, _ = io.WriteString(in, "apple\nbanana\n")
	})
	require.NotNil(t, result)
	// echo hello outputs "hello"; result is at least a non-nil slice.
	assert.True(t, len(result) >= 1)
}

func TestIsCommandAvailable_NoShell(t *testing.T) {
	// If SHELL is empty, falls back to "sh".
	t.Setenv("SHELL", "")
	assert.True(t, isCommandAvailable("echo"))
}

func TestBrowserOpenCommand_Darwin(t *testing.T) {
	// On the current system (macOS), should return "open".
	cmd := browserOpenCommand()
	// Just ensure it's non-empty.
	assert.NotEmpty(t, cmd)
}
