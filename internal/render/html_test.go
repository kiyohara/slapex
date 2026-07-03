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

	rootBlock := cssBlock(t, css, ":root")
	assertCSSDeclaration(t, rootBlock, "--thread-summary-fg", "var(--mention-fg)")
	assertCSSDeclaration(t, rootBlock, "--thread-avatar-bg", "#7c3085")
	assertCSSDeclaration(t, rootBlock, "--thread-avatar-more-bg", "rgba(29, 28, 29, 0.72)")

	unfurlBlock := cssBlock(t, css, ".unfurl")
	assertCSSDeclaration(t, unfurlBlock, "padding", "6px 10px")
	assertCSSDeclaration(t, unfurlBlock, "border-left", "4px solid var(--line)")

	unfurlServiceBlock := cssBlock(t, css, ".unfurl-service")
	assertCSSDeclaration(t, unfurlServiceBlock, "font-size", "12px")
	assertCSSDeclaration(t, unfurlServiceBlock, "font-weight", "700")

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
	assertCSSDeclaration(t, threadLabelBlock, "width", "min(360px, 100%)")
	assertCSSDeclaration(t, threadLabelBlock, "background", "var(--thread-chip-bg)")
	assertCSSDeclaration(t, threadLabelBlock, "border", "1px solid var(--thread-chip-line)")
	assertCSSDeclaration(t, threadLabelBlock, "border-radius", "6px")
	assertCSSDeclaration(t, threadLabelBlock, "padding", "3px 12px 3px 0")
	assertCSSDeclaration(t, threadLabelBlock, "font-size", "13px")
	assertCSSDeclaration(t, threadLabelBlock, "margin", "0 0 12px")

	threadLabelHoverBlock := cssBlock(t, css, ".thread-label:hover")
	assertCSSDeclaration(t, threadLabelHoverBlock, "background", "var(--thread-chip-bg-hover)")
	assertCSSDeclaration(t, threadLabelHoverBlock, "border-color", "var(--thread-chip-line-hover)")

	threadLabelCaretBlock := cssBlock(t, css, ".thread-label::after")
	assertCSSDeclaration(t, threadLabelCaretBlock, "content", `"▸"`)
	assertCSSDeclaration(t, threadLabelCaretBlock, "margin-left", "auto")
	assertCSSDeclaration(t, threadLabelCaretBlock, "color", "var(--thread-summary-caret)")

	threadOpenCaretBlock := cssBlock(t, css, ".thread-group[open] .thread-label::after")
	assertCSSDeclaration(t, threadOpenCaretBlock, "content", `"▾"`)

	threadLabelCountBlock := cssBlock(t, css, ".thread-label-count")
	assertCSSDeclaration(t, threadLabelCountBlock, "color", "var(--thread-summary-fg)")

	threadParticipantsBlock := cssBlock(t, css, ".thread-participants")
	assertCSSDeclaration(t, threadParticipantsBlock, "display", "inline-flex")

	threadParticipantBlock := cssBlock(t, css, ".thread-participant")
	assertCSSDeclaration(t, threadParticipantBlock, "width", "20px")
	assertCSSDeclaration(t, threadParticipantBlock, "height", "20px")
	assertCSSDeclaration(t, threadParticipantBlock, "border", "1px solid var(--thread-avatar-border)")
	assertCSSDeclaration(t, threadParticipantBlock, "border-radius", "4px")
	assertCSSDeclaration(t, threadParticipantBlock, "background", "var(--thread-avatar-bg)")
	assertCSSDeclaration(t, threadParticipantBlock, "color", "var(--thread-avatar-fg)")

	threadMoreParticipantBlock := cssBlock(t, css, ".thread-participant-more")
	assertCSSDeclaration(t, threadMoreParticipantBlock, "background", "var(--thread-avatar-more-bg)")
	assertCSSDeclaration(t, threadMoreParticipantBlock, "color", "var(--thread-avatar-fg)")

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

func TestWriteStyleCSSStylesExportHeaderAndFooter(t *testing.T) {
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

	headerBlock := cssBlock(t, css, ".export-header")
	if strings.Contains(headerBlock, "border-bottom") {
		t.Fatalf(".export-header still has border-bottom: %s", headerBlock)
	}

	titleLineBlock := cssBlock(t, css, ".export-title-line")
	assertCSSDeclaration(t, titleLineBlock, "display", "flex")
	assertCSSDeclaration(t, titleLineBlock, "align-items", "center")

	workspaceIconBlock := cssBlock(t, css, ".workspace-icon")
	assertCSSDeclaration(t, workspaceIconBlock, "width", "24px")
	assertCSSDeclaration(t, workspaceIconBlock, "height", "24px")

	channelTitleBlock := cssBlock(t, css, ".export-title .channel-title")
	assertCSSDeclaration(t, channelTitleBlock, "margin-left", "10px")

	channelHashBlock := cssBlock(t, css, ".channel-hash")
	if strings.Contains(channelHashBlock, "color") {
		t.Fatalf(".channel-hash should inherit text color: %s", channelHashBlock)
	}

	titleLinkBlock := cssBlock(t, css, ".title-link")
	assertCSSDeclaration(t, titleLinkBlock, "color", "inherit")
	assertCSSDeclaration(t, titleLinkBlock, "text-decoration", "none")

	titleLinkHoverBlock := cssBlock(t, css, ".title-link:hover,\n.title-link:focus-visible")
	assertCSSDeclaration(t, titleLinkHoverBlock, "text-decoration", "underline")

	footerBlock := cssBlock(t, css, ".export-footer")
	assertCSSDeclaration(t, footerBlock, "justify-content", "space-between")
	assertCSSDeclaration(t, footerBlock, "align-items", "flex-start")

	footerLinkBlock := cssBlock(t, css, ".footer-project-link")
	assertCSSDeclaration(t, footerLinkBlock, "display", "inline-flex")
	assertCSSDeclaration(t, footerLinkBlock, "flex", "none")
	assertCSSDeclaration(t, footerLinkBlock, "text-decoration", "none")

	footerLogoBlock := cssBlock(t, css, ".footer-logo")
	assertCSSDeclaration(t, footerLogoBlock, "width", "16px")
	assertCSSDeclaration(t, footerLogoBlock, "height", "16px")
}

func TestWriteStaticAssetsWritesFooterLogo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := WriteStaticAssets(dir); err != nil {
		t.Fatalf("WriteStaticAssets: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "assets", "slapex-logo.svg"))
	if err != nil {
		t.Fatalf("read slapex-logo.svg: %v", err)
	}
	if !strings.Contains(string(data), "slapex logo") {
		t.Fatalf("slapex-logo.svg does not look like the embedded project logo")
	}
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
