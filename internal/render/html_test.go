package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteStyleCSSKeepsTimestampsFromWrapping(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := WriteStyleCSS(dir); err != nil {
		t.Fatalf("WriteStyleCSS: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(data)

	for _, marker := range []string{
		"flex: none;",
		"white-space: nowrap;",
		".system-body { min-width: 0; overflow-wrap: anywhere; }",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("style.css missing marker %q", marker)
		}
	}
}
