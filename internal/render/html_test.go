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

	editedBlock := cssBlock(t, css, ".edited")
	assertCSSDeclaration(t, editedBlock, "font-style", "normal")
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
	assertCSSDeclaration(t, threadGroupBlock, "margin", "8px 0 0 48px")
	assertCSSDeclaration(t, threadGroupBlock, "padding-left", "0")

	threadGuideBlock := cssBlock(t, css, ".thread-group::before")
	assertCSSDeclaration(t, threadGuideBlock, "display", "none")
	assertCSSDeclaration(t, threadGuideBlock, "border-left", "2px solid var(--thread-line)")

	threadOpenGuideBlock := cssBlock(t, css, ".thread-group[open]::before")
	assertCSSDeclaration(t, threadOpenGuideBlock, "display", "block")

	threadLabelBlock := cssBlock(t, css, ".thread-label")
	assertCSSDeclaration(t, threadLabelBlock, "font-weight", "700")
	assertCSSDeclaration(t, threadLabelBlock, "cursor", "pointer")
	assertCSSDeclaration(t, threadLabelBlock, "list-style", "none")
	assertCSSDeclaration(t, threadLabelBlock, "display", "inline-flex")
	assertCSSDeclaration(t, threadLabelBlock, "width", "fit-content")
	assertCSSDeclaration(t, threadLabelBlock, "background", "var(--thread-chip-bg)")
	assertCSSDeclaration(t, threadLabelBlock, "border", "1px solid var(--thread-chip-line)")
	assertCSSDeclaration(t, threadLabelBlock, "border-radius", "12px")
	assertCSSDeclaration(t, threadLabelBlock, "font-size", "13px")
	assertCSSDeclaration(t, threadLabelBlock, "margin", "0 0 12px")

	threadLabelHoverBlock := cssBlock(t, css, ".thread-label:hover")
	assertCSSDeclaration(t, threadLabelHoverBlock, "background", "var(--thread-chip-bg-hover)")
	assertCSSDeclaration(t, threadLabelHoverBlock, "border-color", "var(--thread-chip-line-hover)")

	threadBlock := cssBlock(t, css, ".thread")
	assertCSSDeclaration(t, threadBlock, "padding", "0 0 0 36px")

	threadNodeBlock := cssBlock(t, css, ".thread .message::before")
	assertCSSDeclaration(t, threadNodeBlock, "border", "2px solid var(--thread-line)")
}

func TestWriteStyleCSSStylesNativeDisclosureControls(t *testing.T) {
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

	exportSummaryBlock := cssBlock(t, css, ".export-meta summary")
	assertCSSDeclaration(t, exportSummaryBlock, "cursor", "pointer")
	assertCSSDeclaration(t, exportSummaryBlock, "list-style", "none")

	exportSummaryFocusBlock := cssBlock(t, css, ".export-meta summary:focus-visible")
	assertCSSDeclaration(t, exportSummaryFocusBlock, "outline", "2px solid var(--mention-fg)")

	threadLabelFocusBlock := cssBlock(t, css, ".thread-label:focus-visible")
	assertCSSDeclaration(t, threadLabelFocusBlock, "outline", "2px solid var(--mention-fg)")
}

func TestWriteStyleCSSLimitsImageLinkHitArea(t *testing.T) {
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

	imageLinkBlock := cssBlock(t, css, ".image-block a")
	assertCSSDeclaration(t, imageLinkBlock, "display", "inline-block")

	thumbBlock := cssBlock(t, css, ".upload-thumb")
	assertCSSDeclaration(t, thumbBlock, "display", "block")
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
