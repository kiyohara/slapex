package export

// Integration rendering scenarios for v1-08 (Issue #22). Each case is an
// independent test that builds a minimal fixture on top of the v1-07 fake
// Slack server harness (runExportScenario / exportScenario) and asserts the
// end-to-end HTML / manifest output. The expected behaviour is the confirmed
// display spec in doc/design/html-rendering.md and doc/design/output-format.md
// (subtypes, tombstone, size-limit replacement, 1000-reply cap, emoji).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/slack"
)

// --- shared fixture / helpers ------------------------------------------------

// baseScenario is a minimal valid scenario: one member channel matching the
// "project-alpha" keyword, two resolvable users, no emoji and no assets. Each
// test fills in Messages / Replies / Assets for the path it exercises.
func baseScenario() exportScenario {
	return exportScenario{
		Auth: slack.AuthTest{
			URL:    "https://acme.example.slack.com/",
			Team:   "Acme Workspace",
			TeamID: "TACME123",
			User:   "slapex",
			UserID: "USLAPEX",
			BotID:  "BSLAPEX",
		},
		Channels: []slack.Channel{
			{ID: "C123", Name: "project-alpha", IsMember: true},
		},
		Users: map[string]slack.User{
			"U01": testUser("U01", "alice", "Alice Example", "Alice", ""),
			"U02": testUser("U02", "bob", "Bob Builder", "Bob", ""),
		},
		Emoji:   map[string]string{},
		Assets:  map[string]fakeAsset{},
		Replies: map[string][]slack.Message{},
	}
}

func renderingOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		ChannelKeyword: "project-alpha",
		OutputDir:      t.TempDir(),
		MaxPosts:       1000,
		Days:           90,
		MaxAttachBytes: 1 << 20, // 1MB
		KeepCache:      true,
		ToolVersion:    "test",
	}
}

func readIndexHTML(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	return string(data)
}

func mustContain(t *testing.T, body, marker string) {
	t.Helper()
	if !strings.Contains(body, marker) {
		t.Fatalf("index.html missing marker %q", marker)
	}
}

func mustNotContain(t *testing.T, body, marker string) {
	t.Helper()
	if strings.Contains(body, marker) {
		t.Fatalf("index.html unexpectedly contains marker %q", marker)
	}
}

// manifestEntryFull mirrors the fields of assets_manifest.json the rendering
// cases assert on (status, size, identity).
type manifestEntryFull struct {
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	FileID       string `json:"file_id"`
	OriginalName string `json:"original_name"`
	SizeBytes    int64  `json:"size_bytes"`
}

func readManifestEntries(t *testing.T, dir string) []manifestEntryFull {
	t.Helper()
	var manifest struct {
		Assets []manifestEntryFull `json:"assets"`
	}
	readJSON(t, filepath.Join(dir, ".cache/assets_manifest.json"), &manifest)
	return manifest.Assets
}

func findManifest(entries []manifestEntryFull, match func(manifestEntryFull) bool) (manifestEntryFull, bool) {
	for _, e := range entries {
		if match(e) {
			return e, true
		}
	}
	return manifestEntryFull{}, false
}

// botProfileName / editedAt build the inline anonymous-struct pointers used by
// slack.Message for bot_profile and edited.
func botProfileName(name string) *struct {
	Name string `json:"name"`
} {
	return &struct {
		Name string `json:"name"`
	}{Name: name}
}

func editedAt(ts string) *struct {
	TS string `json:"ts"`
} {
	return &struct {
		TS string `json:"ts"`
	}{TS: ts}
}

// --- case 1: fenced code block with URL and multiple lines -------------------

func TestRunIntegrationFencedCodeBlock(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type: "message",
			TS:   "1700000001.000000",
			User: "U01",
			// Slack stores auto-linked URLs as <...> even inside code blocks.
			Text: "Snippet:\n```\ncurl <https://example.com/api?id=42>\necho ok\n```\ndone",
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, "<pre><code>")
	// URL kept as plain display text, multi-line content preserved (PR #14).
	mustContain(t, body, "curl https://example.com/api?id=42\necho ok")
	mustNotContain(t, body, `<a href="https://example.com/api?id=42"`)
}

// --- case 2: system rows render quietly and supplement missing actors --------

func TestRunIntegrationSystemRows(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Users["U03"] = testUser("U03", "set", "Set User", "set", "")
	sc.Users["U04"] = testUser("U04", "charlie", "Charlie Inviter", "Charlie", "")
	sc.Messages = []slack.Message{
		{
			Type:    "message",
			Subtype: "channel_join",
			TS:      "1700000100.000000",
			User:    "U01",
			Inviter: "U04",
			Text:    "<@U01> has joined the channel",
		},
		{
			Type:    "message",
			Subtype: "channel_topic",
			TS:      "1700000200.000000",
			User:    "U02",
			Text:    "set the channel topic: Launch planning",
		},
		{
			Type:    "message",
			Subtype: "channel_purpose",
			TS:      "1700000300.000000",
			Text:    "set the channel purpose: Planning docs",
		},
		{
			Type:    "message",
			Subtype: "channel_name",
			TS:      "1700000400.000000",
			User:    "U03",
			Text:    "set the channel name: project-beta",
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, `<div class="system-message">`)
	mustContain(t, body, "has joined the channel")
	mustContain(t, body, `has joined the channel <span class="system-context">(invited by <span class="mention">@Charlie</span>)</span>`)
	mustContain(t, body, `<span class="mention">@Bob</span> set the channel topic: Launch planning`)
	mustContain(t, body, "set the channel purpose: Planning docs")
	mustContain(t, body, `<span class="mention">@set</span> set the channel name: project-beta`)
	if got := strings.Count(body, `<span class="mention">@Alice</span>`); got != 1 {
		t.Fatalf("@Alice mention count = %d, want 1 (channel_join must not get a duplicate actor prefix)", got)
	}
	if got := strings.Count(body, `<span class="mention">@Bob</span>`); got != 1 {
		t.Fatalf("@Bob mention count = %d, want 1 (channel_topic gets exactly one actor prefix)", got)
	}
	if got := strings.Count(body, `<span class="mention">@Charlie</span>`); got != 1 {
		t.Fatalf("@Charlie mention count = %d, want 1 (channel_join inviter gets exactly one context suffix)", got)
	}
	if got := strings.Count(body, `<span class="mention">@set</span>`); got != 1 {
		t.Fatalf("@set mention count = %d, want 1 (display name must not suppress actor prefix)", got)
	}
	// System rows carry no avatar and are not rendered as full messages.
	mustNotContain(t, body, `<div class="message">`)
	mustNotContain(t, body, `class="avatar"`)
}

// --- case 3: tombstone parent placeholder with normal thread replies ---------

func TestRunIntegrationTombstoneParent(t *testing.T) {
	t.Parallel()

	const parentTS = "1700000300.000000"
	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type:       "message",
			Subtype:    "tombstone",
			TS:         parentTS,
			ThreadTS:   parentTS,
			ReplyCount: 2,
		},
	}
	sc.Replies = map[string][]slack.Message{
		parentTS: {
			{Type: "message", TS: "1700000301.000000", ThreadTS: parentTS, User: "U02", Text: "First reply after deletion"},
			{Type: "message", TS: "1700000302.000000", ThreadTS: parentTS, User: "U01", Text: "Second reply still here"},
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, `<span class="author">(削除)</span>`)
	mustContain(t, body, "(削除されたメッセージ)")
	mustContain(t, body, `<div class="thread">`)
	assertOrder(t, body,
		"(削除されたメッセージ)",
		"First reply after deletion",
		"Second reply still here",
	)
}

// --- case 4: unknown subtype (with / without text) ---------------------------

func TestRunIntegrationUnknownSubtype(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type:    "message",
			Subtype: "huddle_thread", // unknown, but has text -> normal display
			TS:      "1700000401.000000",
			User:    "U01",
			Text:    "Huddle summary text",
		},
		{
			Type:    "message",
			Subtype: "some_unknown_event", // unknown, no text -> system row
			TS:      "1700000402.000000",
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	// With text: rendered as a normal message body.
	mustContain(t, body, `<div class="message-body">Huddle summary text</div>`)
	// Without text: quiet system row naming the subtype.
	mustContain(t, body, `<span class="system-body">(未対応のメッセージ種別: some_unknown_event)</span>`)
}

// --- case 5: me_message (italic) and bot_message display name ----------------

func TestRunIntegrationMeAndBotMessage(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type:    "message",
			Subtype: "me_message",
			TS:      "1700000501.000000",
			User:    "U01",
			Text:    "waves hello",
		},
		{
			Type:       "message",
			Subtype:    "bot_message",
			TS:         "1700000502.000000",
			BotID:      "B001",
			Username:   "deploybot",
			BotProfile: botProfileName("Deploy Bot"),
			Text:       "Deployment finished",
		},
		{
			Type:     "message",
			Subtype:  "bot_message",
			TS:       "1700000503.000000",
			BotID:    "B002",
			Username: "Webhook",
			Text:     "Webhook event received",
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	// me_message renders italic.
	mustContain(t, body, `<div class="message-body me-message">waves hello</div>`)
	// bot_profile.name wins over username.
	mustContain(t, body, `<span class="author">Deploy Bot</span>`)
	mustNotContain(t, body, "deploybot")
	// username is used when there is no bot_profile.
	mustContain(t, body, `<span class="author">Webhook</span>`)
}

// --- case 6: edited message shows the quiet (edited) marker ------------------

func TestRunIntegrationEditedMessage(t *testing.T) {
	t.Parallel()

	const parentTS = "1700000601.000000"
	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type:       "message",
			TS:         parentTS,
			ThreadTS:   parentTS,
			User:       "U01",
			Text:       "This line was edited",
			ReplyCount: 1,
			Edited:     editedAt("1700000605.000000"),
		},
		{
			Type: "message",
			TS:   "1700000602.000000",
			User: "U02",
			Text: "This line was not edited",
		},
		{
			Type:   "message",
			TS:     "1700000604.000000",
			User:   "U01",
			Edited: editedAt("1700000606.000000"),
			Attachments: []slack.Attachment{
				{ServiceName: "Example", Title: "Preview without body"},
			},
		},
		{
			Type:    "message",
			Subtype: "me_message",
			TS:      "1700000605.000000",
			User:    "U02",
			Text:    "waves edited",
			Edited:  editedAt("1700000608.000000"),
		},
	}
	sc.Replies = map[string][]slack.Message{
		parentTS: {
			{
				Type:     "message",
				TS:       "1700000603.000000",
				ThreadTS: parentTS,
				User:     "U02",
				Text:     "Reply was edited",
				Edited:   editedAt("1700000607.000000"),
			},
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, `This line was edited <span class="edited">(edited)</span>`)
	mustContain(t, body, `Reply was edited <span class="edited">(edited)</span>`)
	mustContain(t, body, `<div class="message-body me-message">waves edited <span class="edited">(edited)</span></div>`)
	assertOrder(t, body, `Preview without body`, `<div class="edited edited-fallback">(edited)</div>`)
	mustNotContain(t, body, `</span><span class="edited">(edited)</span>`)
	if n := strings.Count(body, "(edited)"); n != 4 {
		t.Fatalf("(edited) marker count = %d, want 4", n)
	}
}

// --- case 7: thread_broadcast appears in timeline and inside the thread ------

func TestRunIntegrationThreadBroadcast(t *testing.T) {
	t.Parallel()

	const parentTS = "1700000700.000000"
	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type:       "message",
			TS:         parentTS,
			ThreadTS:   parentTS,
			User:       "U01",
			Text:       "Parent post",
			ReplyCount: 2,
		},
		{
			// A broadcast reply also surfaces on the channel timeline.
			Type:     "message",
			Subtype:  "thread_broadcast",
			TS:       "1700000702.000000",
			ThreadTS: parentTS,
			User:     "U02",
			Text:     "Broadcast to channel",
		},
	}
	sc.Replies = map[string][]slack.Message{
		parentTS: {
			{Type: "message", TS: "1700000701.000000", ThreadTS: parentTS, User: "U01", Text: "Normal reply"},
			{Type: "message", Subtype: "thread_broadcast", TS: "1700000702.000000", ThreadTS: parentTS, User: "U02", Text: "Broadcast to channel"},
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	// The broadcast renders twice: once as a thread reply and once as a
	// standalone timeline message. (Threads render inline under the parent, so
	// the timeline copy — whose ts is later than the parent's — lands after the
	// thread block rather than before it.)
	if n := strings.Count(body, "Broadcast to channel"); n != 2 {
		t.Fatalf("broadcast text count = %d, want 2 (timeline + thread)", n)
	}
	if n := strings.Count(body, `<div class="thread">`); n != 1 {
		t.Fatalf("thread block count = %d, want 1", n)
	}
	mustContain(t, body, `Thread (2 messages)`)
	// One copy is inside the thread, after the non-broadcast reply.
	assertOrder(t, body, `<div class="thread">`, "Normal reply", "Broadcast to channel")
}

// --- case 8: date divider inserted when the day changes ----------------------

func TestRunIntegrationDateDividers(t *testing.T) {
	t.Parallel()

	// Three days apart so the two messages land on different calendar dates in
	// any timezone; expected dates are derived from tsTime to stay TZ-robust.
	const ts1 = "1700000000.000000"
	const ts2 = "1700259200.000000"
	date1 := tsTime(ts1).Format("2006-01-02")
	date2 := tsTime(ts2).Format("2006-01-02")
	if date1 == date2 {
		t.Fatalf("test setup: expected distinct dates, got %s twice", date1)
	}

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{Type: "message", TS: ts1, User: "U01", Text: "Day one message"},
		{Type: "message", TS: ts2, User: "U02", Text: "Day two message"},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	div1 := fmt.Sprintf(`<div class="date-divider"><span>%s</span></div>`, date1)
	div2 := fmt.Sprintf(`<div class="date-divider"><span>%s</span></div>`, date2)
	assertOrder(t, body, div1, "Day one message", div2, "Day two message")
}

// --- case 9: <!date^...> fallback string is shown in the body ----------------

func TestRunIntegrationDateTokenFallback(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type: "message",
			TS:   "1700000800.000000",
			User: "U01",
			Text: "Meeting on <!date^1700000000^{date_short}|Nov 14, 2023> at noon",
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, "Meeting on Nov 14, 2023 at noon")
	mustNotContain(t, body, "date^")
}

// --- case 10a: oversize non-image attachment is replaced + manifest skip -----

func TestRunIntegrationOversizeAttachment(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type: "message",
			TS:   "1700000900.000000",
			User: "U01",
			Text: "Here is the archive",
			Files: []slack.File{
				{
					ID:                 "F-ZIP",
					Name:               "big-archive.zip",
					Mimetype:           "application/zip",
					Size:               5000,
					URLPrivateDownload: "{{base}}/files/big-archive.zip",
				},
			},
		},
	}
	opts := renderingOptions(t)
	opts.MaxAttachBytes = 100

	got := runExportScenario(t, sc, opts)
	body := readIndexHTML(t, got.OutputDir)

	// Replacement message includes file name (link text), Slack file ID,
	// original size and the limit (output-format.md). Asserting the file ID
	// keeps the display honest if it is ever dropped.
	mustContain(t, body, `<span class="file-link unavailable">📄 big-archive.zip</span>`)
	mustContain(t, body, "サイズオーバーのため保存されませんでした。(file ID: F-ZIP, 5000B, 上限 100B)")

	entry, ok := findManifest(readManifestEntries(t, got.OutputDir), func(e manifestEntryFull) bool {
		return e.Kind == "attachment" && e.FileID == "F-ZIP"
	})
	if !ok {
		t.Fatalf("manifest missing attachment entry for F-ZIP")
	}
	if entry.Status != "skipped_size" {
		t.Fatalf("attachment status = %q, want skipped_size", entry.Status)
	}
	if entry.OriginalName != "big-archive.zip" || entry.SizeBytes != 5000 {
		t.Fatalf("attachment manifest = %+v, want big-archive.zip / 5000", entry)
	}
}

// --- case 10b: oversize image original keeps thumbnail + note ----------------

func TestRunIntegrationOversizeImageOriginal(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type: "message",
			TS:   "1700001000.000000",
			User: "U01",
			Text: "Big screenshot",
			Files: []slack.File{
				{
					ID:                 "F-BIGIMG",
					Name:               "huge-photo.png",
					Mimetype:           "image/png",
					Size:               5000,
					URLPrivateDownload: "{{base}}/files/huge-original.png",
					Thumb360:           "{{base}}/files/huge-thumb.png",
				},
			},
		},
	}
	// Only the thumbnail is fetchable; the original exceeds the limit and is
	// never downloaded.
	sc.Assets["/files/huge-thumb.png"] = fakeAsset{ContentType: "image/png", Body: "huge-thumb"}
	opts := renderingOptions(t)
	opts.MaxAttachBytes = 100

	got := runExportScenario(t, sc, opts)
	body := readIndexHTML(t, got.OutputDir)

	// Thumbnail still shown, original-not-saved note present, no original link.
	mustContain(t, body, `<img class="upload-thumb"`)
	mustContain(t, body, "original はサイズ上限超過のため保存されませんでした。(huge-photo.png: 5000B, 上限 100B)")
	mustNotContain(t, body, "assets/uploads/originals/")

	entries := readManifestEntries(t, got.OutputDir)
	orig, ok := findManifest(entries, func(e manifestEntryFull) bool {
		return e.Kind == "upload_original" && e.FileID == "F-BIGIMG"
	})
	if !ok || orig.Status != "skipped_size" {
		t.Fatalf("upload_original entry = %+v (ok=%v), want skipped_size", orig, ok)
	}
	if _, ok := findManifest(entries, func(e manifestEntryFull) bool {
		return e.Kind == "upload_thumb" && e.Status == "saved"
	}); !ok {
		t.Fatalf("manifest missing saved upload_thumb entry: %+v", entries)
	}
}

// --- case 11: asset download failure (404) is partial, export still succeeds -

func TestRunIntegrationAssetDownloadFailure(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type: "message",
			TS:   "1700001100.000000",
			User: "U01",
			Text: "Report attached",
			Files: []slack.File{
				{
					ID:                 "F-404",
					Name:               "report.pdf",
					Mimetype:           "application/pdf",
					Size:               50, // under the limit: failure is the 404, not the size
					URLPrivateDownload: "{{base}}/files/missing-report.pdf",
				},
			},
		},
	}
	// The download URL is intentionally not registered as an asset -> 404.

	// runExportScenario fatals if Run returns an error, so reaching the
	// assertions proves the export as a whole succeeded despite the failure.
	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, `<span class="file-link unavailable">📄 report.pdf</span>`)
	mustContain(t, body, "取得に失敗しました。")

	entry, ok := findManifest(readManifestEntries(t, got.OutputDir), func(e manifestEntryFull) bool {
		return e.Kind == "attachment" && e.FileID == "F-404"
	})
	if !ok || entry.Status != "failed" {
		t.Fatalf("attachment entry = %+v (ok=%v), want status failed", entry, ok)
	}
}

// --- case 12: thread replies over 1000 are truncated with a notice ----------

func TestRunIntegrationRepliesTruncated(t *testing.T) {
	t.Parallel()

	const parentTS = "1700001200.000000"
	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type:       "message",
			TS:         parentTS,
			ThreadTS:   parentTS,
			User:       "U01",
			Text:       "Big thread",
			ReplyCount: 1001,
		},
	}
	replies := make([]slack.Message, 0, 1001)
	for i := 1; i <= 1001; i++ {
		replies = append(replies, slack.Message{
			Type:     "message",
			TS:       fmt.Sprintf("1700001200.%06d", i),
			ThreadTS: parentTS,
			User:     "U01",
			Text:     fmt.Sprintf("reply %d", i),
		})
	}
	sc.Replies = map[string][]slack.Message{parentTS: replies}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	// All replies up to the 1000 cap are kept, including the boundary value
	// reply 1000; only the 1001st is dropped. (reply 1000 present guards
	// against an off-by-one that cuts at 999.)
	mustContain(t, body, `<div class="message-body">reply 1</div>`)
	mustContain(t, body, `<div class="message-body">reply 1000</div>`)
	mustNotContain(t, body, "reply 1001")

	threadIdx := strings.Index(body, `<div class="thread">`)
	noticeIdx := strings.Index(body, `<div class="notice">取り扱える件数の上限に達しました。</div>`)
	if threadIdx < 0 || noticeIdx < 0 || noticeIdx < threadIdx {
		t.Fatalf("truncation notice should render inside the thread: thread=%d notice=%d", threadIdx, noticeIdx)
	}
	mustContain(t, body, `Thread (1000+ messages)`)
}

// --- case 13: standard emoji -> Unicode, unknown shortcode stays literal -----

func TestRunIntegrationEmojiRendering(t *testing.T) {
	t.Parallel()

	res, err := emoji.NewResolver(nil)
	if err != nil {
		t.Fatalf("build emoji resolver: %v", err)
	}
	smile := res.Resolve("smile").Unicode
	if smile == "" {
		t.Fatalf("test setup: :smile: has no Unicode mapping")
	}

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type: "message",
			TS:   "1700001300.000000",
			User: "U01",
			Text: "Status :smile: and :definitely_not_a_real_emoji: done",
		},
	}

	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	// Standard emoji rendered as Unicode text directly (no image element).
	mustContain(t, body, smile)
	mustNotContain(t, body, `<img class="emoji"`)
	// Unknown shortcode (not standard, not custom) kept literally.
	mustContain(t, body, ":definitely_not_a_real_emoji:")
}
