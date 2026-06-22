package render

import (
	"os"
	"path/filepath"
	"regexp"
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

	timeBlock := cssBlock(t, css, ".time")
	assertCSSDeclaration(t, timeBlock, "flex", "none")
	assertCSSDeclaration(t, timeBlock, "white-space", "nowrap")

	systemBodyBlock := cssBlock(t, css, ".system-body")
	assertCSSDeclaration(t, systemBodyBlock, "min-width", "0")
	assertCSSDeclaration(t, systemBodyBlock, "overflow-wrap", "anywhere")

	systemContextBlock := cssBlock(t, css, ".system-context")
	assertCSSDeclaration(t, systemContextBlock, "color", "var(--muted)")
	assertCSSDeclaration(t, systemContextBlock, "font-size", "12px")
}

func TestWriteStyleCSSDistinguishesThreadFromUnfurl(t *testing.T) {
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

	unfurlBlock := cssBlock(t, css, ".unfurl")
	assertCSSDeclaration(t, unfurlBlock, "padding", "6px 10px")
	assertCSSDeclaration(t, unfurlBlock, "border-left", "4px solid var(--line)")

	threadGroupBlock := cssBlock(t, css, ".thread-group")
	assertCSSDeclaration(t, threadGroupBlock, "padding-left", "28px")

	threadGuideBlock := cssBlock(t, css, ".thread-group::before")
	assertCSSDeclaration(t, threadGuideBlock, "border-left", "2px solid var(--thread-line)")

	threadLabelBlock := cssBlock(t, css, ".thread-label")
	assertCSSDeclaration(t, threadLabelBlock, "font-weight", "700")

	threadLabelRuleBlock := cssBlock(t, css, ".thread-label::after")
	assertCSSDeclaration(t, threadLabelRuleBlock, "border-top", "1px solid var(--thread-line)")

	threadBlock := cssBlock(t, css, ".thread")
	assertCSSDeclaration(t, threadBlock, "padding", "0 0 0 8px")

	threadNodeBlock := cssBlock(t, css, ".thread .message::before")
	assertCSSDeclaration(t, threadNodeBlock, "border", "2px solid var(--thread-line)")
}

func cssBlock(t *testing.T, css, selector string) string {
	t.Helper()

	re := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`)
	matches := re.FindStringSubmatch(css)
	if matches == nil {
		t.Fatalf("style.css missing selector %q", selector)
	}
	return matches[1]
}

func assertCSSDeclaration(t *testing.T, block, property, value string) {
	t.Helper()

	re := regexp.MustCompile(`(^|;)\s*` + regexp.QuoteMeta(property) + `\s*:\s*` + regexp.QuoteMeta(value) + `\s*(;|$)`)
	if !re.MatchString(strings.TrimSpace(block)) {
		t.Fatalf("CSS block missing declaration %s: %s", property, value)
	}
}
