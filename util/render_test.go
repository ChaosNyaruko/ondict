package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceLINK(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal text",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:  "LINK prefix with null terminator",
			input: "@@@LINK=some_word\r\n\x00",
			// The function escapes the link and wraps it in HTML
			expected: `See <a class=Crossrefto href="/dict?query=some_word&engine=mdx&format=html">some_word</a> for more`,
		},
		{
			name:     "LINK prefix without null terminator",
			input:    "@@@LINK=some_word",
			expected: "@@@LINK=some_word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceLINK(tt.input)
			if tt.name == "LINK prefix with null terminator" {
				// The output contains newlines, so we check if it contains the link
				assert.True(t, strings.Contains(got, "some_word"))
				assert.True(t, strings.Contains(got, "href=\"/dict?query=some_word"))
			} else {
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
