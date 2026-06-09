package decoder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeString_UTF8(t *testing.T) {
	m := &MDict{encoding: "UTF-8"}
	result := m.decodeString([]byte("hello"))
	require.Equal(t, "hello", result)
}

func TestDecodeString_UTF16(t *testing.T) {
	m := &MDict{encoding: "UTF-16"}
	// Encode "hi" as little-endian UTF-16.
	b := []byte{'h', 0, 'i', 0}
	result := m.decodeString(b)
	require.Equal(t, "hi", result)
}

func TestDecodeString_DefaultEncoding(t *testing.T) {
	m := &MDict{}
	result := m.decodeString([]byte("world"))
	require.Equal(t, "world", result)
}

func TestDecode_InvalidExtension(t *testing.T) {
	m := &MDict{}
	// .txt is not a valid extension.
	err := m.Decode("/tmp/test.txt", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected file ext")
}
