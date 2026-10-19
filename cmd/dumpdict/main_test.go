package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectDictPaths_FileAddsPairedMDD(t *testing.T) {
	dir := t.TempDir()
	mdxPath := filepath.Join(dir, "dict.mdx")
	mddPath := filepath.Join(dir, "dict.mdd")
	require.NoError(t, os.WriteFile(mdxPath, []byte("mdx"), 0o644))
	require.NoError(t, os.WriteFile(mddPath, []byte("mdd"), 0o644))

	paths, err := collectDictPaths([]string{mdxPath}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{mdxPath}, paths.mdx)
	require.Equal(t, []string{mddPath}, paths.mdd)
}

func TestCollectDictPaths_DirectoryFindsMDXAndMDD(t *testing.T) {
	dir := t.TempDir()
	mdxPath := filepath.Join(dir, "dict.mdx")
	mddPath := filepath.Join(dir, "dict.mdd")
	nestedMDDPath := filepath.Join(dir, "nested", "media.mdd")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedMDDPath), 0o755))
	require.NoError(t, os.WriteFile(mdxPath, []byte("mdx"), 0o644))
	require.NoError(t, os.WriteFile(mddPath, []byte("mdd"), 0o644))
	require.NoError(t, os.WriteFile(nestedMDDPath, []byte("mdd"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("txt"), 0o644))

	paths, err := collectDictPaths(nil, []string{dir})
	require.NoError(t, err)
	require.Equal(t, []string{mdxPath}, paths.mdx)
	require.ElementsMatch(t, []string{mddPath, nestedMDDPath}, paths.mdd)
}

func TestCollectDictPaths_AllowsExplicitMDD(t *testing.T) {
	mddPath := filepath.Join(t.TempDir(), "dict.mdd")
	require.NoError(t, os.WriteFile(mddPath, []byte("mdd"), 0o644))

	paths, err := collectDictPaths([]string{mddPath}, nil)
	require.NoError(t, err)
	require.Empty(t, paths.mdx)
	require.Equal(t, []string{mddPath}, paths.mdd)
}

func TestCollectDictPaths_RejectsUnsupportedExplicitFile(t *testing.T) {
	_, err := collectDictPaths([]string{"dict.txt"}, nil)
	require.Error(t, err)
}

func TestDumpMDDFileMissingPath(t *testing.T) {
	err := dumpMDDFile(filepath.Join(t.TempDir(), "missing.mdd"))
	require.Error(t, err)
}
