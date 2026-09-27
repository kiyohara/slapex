package export

// Unit tests for the .cache/ files writeCaches writes (doc/design/cache.md).
// Each case writes the files from fixed, fictional inputs and compares them
// with the expected JSON as decoded values: key order does not matter, while a
// null value, a missing key and an empty array or object stay distinct
// (Issue #194).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// cacheTestNow is the export clock of every case; generated_at and executed_at
// record it in UTC.
var cacheTestNow = time.Date(2026, 7, 4, 16, 32, 41, 0, time.FixedZone("JST", 9*60*60))

// cacheTestDownloader serves a PNG signature for every asset URL, so each saved
// asset is sniffed as image/png and all of them share one content-hash file
// name. The URLs in failures fail instead.
type cacheTestDownloader struct{ failures map[string]bool }

func (d cacheTestDownloader) Download(_ context.Context, srcURL string, _ int64, w io.Writer) (int64, string, error) {
	if d.failures[srcURL] {
		return 0, "", errors.New("fake download failure")
	}
	n, err := io.WriteString(w, "\x89PNG\r\n\x1a\n")
	return int64(n), "image/png", err
}

// writeCachesCase is one run's input to writeCaches. The workspace, the channel
// and their labels are the same in every case.
type writeCachesCase struct {
	opts        Options
	fetchRange  messageFetchRange
	counts      exportCounts
	assetTotals assetCounts
	users       map[string]*slack.User
	bots        map[string]*slack.Bot
	emoji       map[string]string
	failures    map[string]bool             // asset URLs whose download fails
	saveAssets  func(assets *output.Assets) // records the manifest entries; nil records none
}

// writeTestCaches writes c's .cache/ files into a new directory and returns it.
func writeTestCaches(t *testing.T, c writeCachesCase) string {
	t.Helper()
	dir := t.TempDir()
	assets := output.NewAssets(context.Background(), cacheTestDownloader{failures: c.failures}, dir, c.opts.MaxAttachBytes)
	if c.saveAssets != nil {
		c.saveAssets(assets)
	}
	auth := &slack.AuthTest{
		URL: "https://acme.example.slack.com/", Team: "Acme Example", TeamID: "T0TEST01",
		User: "alice", UserID: "U0TEST01",
	}
	ch := slack.Channel{ID: "C0TEST01", Name: "project-alpha", IsPrivate: true, IsMember: true}
	if err := writeCaches(dir, cacheTestNow, auth, ch, c.opts, c.fetchRange, "acme-example", "project-alpha_C0TEST01",
		c.counts.timeline, c.counts.threads, c.counts.replies, c.counts.excluded,
		c.assetTotals.saved, c.assetTotals.skipped, c.assetTotals.failed,
		c.users, c.bots, c.emoji, assets); err != nil {
		t.Fatalf("writeCaches: %v", err)
	}
	return dir
}

// assertJSONEqual fails unless got and want decode to the same value. Numbers
// are compared as written, so 1 and 1.0 differ too.
func assertJSONEqual(t *testing.T, name string, got []byte, want string) {
	t.Helper()
	gotValue, err := decodeJSONValue(got)
	if err != nil {
		t.Fatalf("decode %s: %v\n%s", name, err, got)
	}
	wantValue, err := decodeJSONValue([]byte(want))
	if err != nil {
		t.Fatalf("decode the expected %s: %v", name, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("%s =\n%s\nwant\n%s", name, got, want)
	}
}

func decodeJSONValue(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("data after the top-level value")
	}
	return v, nil
}

func TestWriteCachesPayloads(t *testing.T) {
	t.Parallel()

	const (
		workspace = `{"team_id": "T0TEST01", "name": "Acme Example", "url": "https://acme.example.slack.com/", "domain": "acme.example.slack.com"}`
		channel   = `{"id": "C0TEST01", "name": "project-alpha", "is_private": true, "is_archived": false, "is_member": true}`
		labels    = `{"workspace_label": "acme-example", "channel_label": "project-alpha_C0TEST01", "workspace_name": "Acme Example", "channel_name": "project-alpha"}`
		authTest  = `{"url": "https://acme.example.slack.com/", "team": "Acme Example", "team_id": "T0TEST01", "user": "alice", "user_id": "U0TEST01", "bot_id": ""}`
		savedPNG  = `4c4b6a3be1314ab86138bef4314dde022e600960d8689a2c8f8631802d20dab6.png`
	)

	tests := []struct {
		name string
		in   writeCachesCase
		want map[string]string // file name -> expected JSON
	}{
		{
			// Every optional field is set: the emoji filters, users and bots
			// with and without the omitempty fields, and saved, skipped and
			// failed assets. Each count differs, so a count recorded under
			// another key fails.
			name: "days range with every optional field",
			in: writeCachesCase{
				opts: Options{
					Days: 30, MaxPosts: 1000, MaxAttachBytes: 10 << 20,
					ExcludeBodyEmoji:     []string{"speak_no_evil"},
					ExcludeReactionEmoji: []string{"do_not_archive", "shushing_face"},
					ToolVersion:          "v9.9.9-test",
				},
				fetchRange:  messageFetchRange{mode: "days", start: cacheTestNow.AddDate(0, 0, -30), end: cacheTestNow},
				counts:      exportCounts{timeline: 11, threads: 4, replies: 7, excluded: 5},
				assetTotals: assetCounts{saved: 3, skipped: 2, failed: 1},
				users: map[string]*slack.User{
					"U0TEST01": func() *slack.User {
						u := &slack.User{ID: "U0TEST01", Name: "alice", RealName: "Alice Example"}
						u.Profile.DisplayName = "Alice"
						u.Profile.Image48 = "https://assets.example.test/alice-48.png"
						u.Profile.Image72 = "https://assets.example.test/alice-72.png"
						return u
					}(),
					"U0TEST02": func() *slack.User {
						u := &slack.User{ID: "U0TEST02", Name: "bob"}
						u.Profile.Image48 = "https://assets.example.test/bob-48.png"
						return u
					}(),
					"U0TEST03": func() *slack.User {
						u := &slack.User{ID: "U0TEST03", Name: "deploybot", IsBot: true}
						u.Profile.DisplayName = "Deploy Bot"
						return u
					}(),
				},
				bots: map[string]*slack.Bot{
					"B0TEST01": {ID: "B0TEST01", Name: "CI Notifier", Icons: slack.BotIcons{Image72: "https://assets.example.test/ci-72.png"}},
					"B0TEST02": {ID: "B0TEST02", Name: "Legacy Hook"},
				},
				emoji: map[string]string{
					"party_sloth": "https://emoji.example.test/party-sloth.png",
					"sloth":       "alias:party_sloth",
				},
				failures: map[string]bool{"https://emoji.example.test/broken.png": true},
				saveAssets: func(assets *output.Assets) {
					assets.Save(output.KindWorkspaceIcon, "https://assets.example.test/workspace-68.png", output.AssetMeta{})
					assets.Save(output.KindAvatar, "https://assets.example.test/alice-72.png", output.AssetMeta{})
					assets.Save(output.KindUploadOriginal, "https://files.example.test/F0TEST01/diagram.png", output.AssetMeta{
						FileID: "F0TEST01", OriginalName: "diagram.png", Mimetype: "image/png", SizeBytes: 2048,
					})
					assets.SkipTooLarge(output.KindUploadOriginal, "https://files.example.test/F0TEST02/video.mp4", output.AssetMeta{
						FileID: "F0TEST02", OriginalName: "video.mp4", Mimetype: "video/mp4", SizeBytes: 50 << 20,
					})
					assets.SkipTooLarge(output.KindAttachment, "https://files.example.test/F0TEST03/archive.zip", output.AssetMeta{
						FileID: "F0TEST03", OriginalName: "archive.zip", Mimetype: "application/zip", SizeBytes: 20 << 20,
					})
					assets.Save(output.KindEmoji, "https://emoji.example.test/broken.png", output.AssetMeta{EmojiName: "broken_emoji"})
				},
			},
			want: map[string]string{
				"metadata.json": `{
					"schema_version": 1,
					"generated_at": "2026-07-04T07:32:41Z",
					"tool_version": "v9.9.9-test",
					"workspace": ` + workspace + `,
					"channel": ` + channel + `,
					"fetch": {
						"days": 30,
						"max_posts": 1000,
						"max_attachment_size_bytes": 10485760,
						"oldest_ts": "1780558361.000000",
						"latest_ts": "1783150361.000000",
						"executed_at": "2026-07-04T07:32:41Z",
						"target_range": {
							"start": "2026-06-04T07:32:41Z",
							"end": "2026-07-04T07:32:41Z",
							"start_slack_ts": "1780558361.000000",
							"end_slack_ts": "1783150361.000000"
						},
						"options": {
							"range_mode": "days",
							"days": 30,
							"max_posts": 1000,
							"max_attachment_size_bytes": 10485760,
							"exclude_body_emoji": ["speak_no_evil"],
							"exclude_reaction_emoji": ["do_not_archive", "shushing_face"]
						}
					},
					"labels": ` + labels + `,
					"counts": {
						"timeline_messages": 11,
						"threads": 4,
						"replies": 7,
						"excluded_messages": 5,
						"assets_saved": 3,
						"assets_skipped": 2,
						"assets_failed": 1
					}
				}`,
				"assets_manifest.json": `{
					"schema_version": 1,
					"generated_at": "2026-07-04T07:32:41Z",
					"assets": [
						{
							"kind": "workspace_icon",
							"source_url": "https://assets.example.test/workspace-68.png",
							"local_path": "assets/workspace-icons/` + savedPNG + `",
							"mimetype": "image/png",
							"size_bytes": 8,
							"status": "saved"
						},
						{
							"kind": "avatar",
							"source_url": "https://assets.example.test/alice-72.png",
							"local_path": "assets/avatars/` + savedPNG + `",
							"mimetype": "image/png",
							"size_bytes": 8,
							"status": "saved"
						},
						{
							"kind": "upload_original",
							"source_url": "https://files.example.test/F0TEST01/diagram.png",
							"local_path": "assets/uploads/originals/` + savedPNG + `",
							"file_id": "F0TEST01",
							"original_name": "diagram.png",
							"mimetype": "image/png",
							"size_bytes": 2048,
							"status": "saved"
						},
						{
							"kind": "upload_original",
							"source_url": "https://files.example.test/F0TEST02/video.mp4",
							"file_id": "F0TEST02",
							"original_name": "video.mp4",
							"mimetype": "video/mp4",
							"size_bytes": 52428800,
							"status": "skipped_size"
						},
						{
							"kind": "attachment",
							"source_url": "https://files.example.test/F0TEST03/archive.zip",
							"file_id": "F0TEST03",
							"original_name": "archive.zip",
							"mimetype": "application/zip",
							"size_bytes": 20971520,
							"status": "skipped_size"
						},
						{
							"kind": "emoji",
							"source_url": "https://emoji.example.test/broken.png",
							"emoji_name": "broken_emoji",
							"status": "failed",
							"error": "fake download failure"
						}
					]
				}`,
				"slack_api_cache.json": `{
					"schema_version": 1,
					"generated_at": "2026-07-04T07:32:41Z",
					"users": {
						"U0TEST01": {"display_name": "Alice", "real_name": "Alice Example", "avatar_url": "https://assets.example.test/alice-72.png"},
						"U0TEST02": {"display_name": "bob", "avatar_url": "https://assets.example.test/bob-48.png"},
						"U0TEST03": {"display_name": "Deploy Bot", "is_bot": true}
					},
					"bots": {
						"B0TEST01": {"name": "CI Notifier", "avatar_url": "https://assets.example.test/ci-72.png"},
						"B0TEST02": {"name": "Legacy Hook"}
					},
					"emoji": {
						"party_sloth": "https://emoji.example.test/party-sloth.png",
						"sloth": "alias:party_sloth"
					},
					"workspace": ` + authTest + `,
					"channel": ` + channel + `
				}`,
			},
		},
		{
			// Nothing optional is set: the options carry no emoji filter and
			// no days, the flat days is still recorded, every count is 0, no
			// asset is recorded (assets is null), users and bots are empty
			// objects and nil emoji is null.
			name: "date range without optional fields",
			in: writeCachesCase{
				opts: Options{Days: 90, Date: "2026-07-03", MaxPosts: 500, MaxAttachBytes: 1 << 20, ToolVersion: "dev"},
				fetchRange: messageFetchRange{
					mode:  "date",
					start: time.Date(2026, 7, 3, 0, 0, 0, 0, cacheTestNow.Location()),
					end:   time.Date(2026, 7, 4, 0, 0, 0, 0, cacheTestNow.Location()),
				},
				users: map[string]*slack.User{},
				bots:  map[string]*slack.Bot{},
			},
			want: map[string]string{
				"metadata.json": `{
					"schema_version": 1,
					"generated_at": "2026-07-04T07:32:41Z",
					"tool_version": "dev",
					"workspace": ` + workspace + `,
					"channel": ` + channel + `,
					"fetch": {
						"days": 90,
						"max_posts": 500,
						"max_attachment_size_bytes": 1048576,
						"oldest_ts": "1783004400.000000",
						"latest_ts": "1783090800.000000",
						"executed_at": "2026-07-04T07:32:41Z",
						"target_range": {
							"start": "2026-07-02T15:00:00Z",
							"end": "2026-07-03T15:00:00Z",
							"start_slack_ts": "1783004400.000000",
							"end_slack_ts": "1783090800.000000"
						},
						"options": {
							"range_mode": "date",
							"date": "2026-07-03",
							"max_posts": 500,
							"max_attachment_size_bytes": 1048576
						}
					},
					"labels": ` + labels + `,
					"counts": {
						"timeline_messages": 0,
						"threads": 0,
						"replies": 0,
						"excluded_messages": 0,
						"assets_saved": 0,
						"assets_skipped": 0,
						"assets_failed": 0
					}
				}`,
				"assets_manifest.json": `{
					"schema_version": 1,
					"generated_at": "2026-07-04T07:32:41Z",
					"assets": null
				}`,
				"slack_api_cache.json": `{
					"schema_version": 1,
					"generated_at": "2026-07-04T07:32:41Z",
					"users": {},
					"bots": {},
					"emoji": null,
					"workspace": ` + authTest + `,
					"channel": ` + channel + `
				}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := writeTestCaches(t, tt.in)
			entries, err := os.ReadDir(filepath.Join(dir, ".cache"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(tt.want) {
				t.Fatalf(".cache/ holds %d files, want %d", len(entries), len(tt.want))
			}
			for name, want := range tt.want {
				got, err := os.ReadFile(filepath.Join(dir, ".cache", name))
				if err != nil {
					t.Fatal(err)
				}
				assertJSONEqual(t, name, got, want)
			}
		})
	}
}

// TestMetadataFetchRangeFields covers the fetch range shapes the cases above do
// not: --from / --to, whose options carry from and to but neither days nor
// date, and a range without an end, whose end and end_slack_ts are null
// (doc/design/cache.md).
func TestMetadataFetchRangeFields(t *testing.T) {
	t.Parallel()

	opts := Options{Days: 90, From: "2026-07-03T09:30", To: "2026-07-03T10:00:00+09:00", MaxPosts: 1000, MaxAttachBytes: 10 << 20}
	tests := []struct {
		name string
		r    messageFetchRange
		want string
	}{
		{
			name: "datetime range",
			r: messageFetchRange{
				mode:  "datetime-range",
				start: time.Date(2026, 7, 3, 9, 30, 0, 0, cacheTestNow.Location()),
				end:   time.Date(2026, 7, 3, 10, 0, 0, 0, cacheTestNow.Location()),
			},
			want: `{
				"target_range": {
					"start": "2026-07-03T00:30:00Z",
					"end": "2026-07-03T01:00:00Z",
					"start_slack_ts": "1783038600.000000",
					"end_slack_ts": "1783040400.000000"
				},
				"options": {
					"range_mode": "datetime-range",
					"from": "2026-07-03T09:30",
					"to": "2026-07-03T10:00:00+09:00",
					"max_posts": 1000,
					"max_attachment_size_bytes": 10485760
				}
			}`,
		},
		{
			name: "range without an end",
			r:    messageFetchRange{mode: "days", start: cacheTestNow.AddDate(0, 0, -90)},
			want: `{
				"target_range": {
					"start": "2026-04-05T07:32:41Z",
					"end": null,
					"start_slack_ts": "1775374361.000000",
					"end_slack_ts": null
				},
				"options": {
					"range_mode": "days",
					"days": 90,
					"max_posts": 1000,
					"max_attachment_size_bytes": 10485760
				}
			}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(map[string]any{
				"target_range": tt.r.metadataTargetRange(),
				"options":      tt.r.metadataOptions(opts),
			})
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, "fetch", got, tt.want)
		})
	}
}
