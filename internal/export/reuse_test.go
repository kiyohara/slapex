package export

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveReuseCacheDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T) (path string, want string)
	}{
		{
			name: "cache directory",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				cacheDir := filepath.Join(dir, ".cache")
				writeTestFile(t, filepath.Join(cacheDir, "metadata.json"))
				return cacheDir, cacheDir
			},
		},
		{
			name: "output directory containing cache",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				cacheDir := filepath.Join(dir, ".cache")
				writeTestFile(t, filepath.Join(cacheDir, "metadata.json"))
				return dir, cacheDir
			},
		},
		{
			name: "missing cache falls back to original path",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				return dir, dir
			},
		},
		{
			name: "metadata directory is not treated as cache metadata",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				if err := os.Mkdir(filepath.Join(dir, "metadata.json"), 0o755); err != nil {
					t.Fatalf("mkdir metadata placeholder: %v", err)
				}
				cacheDir := filepath.Join(dir, ".cache")
				writeTestFile(t, filepath.Join(cacheDir, "metadata.json"))
				return dir, cacheDir
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path, want := tt.setup(t)
			if got := resolveReuseCacheDir(path); got != want {
				t.Fatalf("resolveReuseCacheDir(%q) = %q, want %q", path, got, want)
			}
		})
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
