package export

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/render"
	"github.com/kiyohara/slapex/internal/slack"
)

func TestResolveFetchRangeDateUsesLocalCalendarDay(t *testing.T) {
	r, err := resolveFetchRange(Options{Date: "2026-07-03"}, time.Time{})
	if err != nil {
		t.Fatalf("resolveFetchRange: %v", err)
	}
	wantStart := time.Date(2026, 7, 3, 0, 0, 0, 0, time.Local)
	wantEnd := wantStart.AddDate(0, 0, 1)
	if r.mode != "date" || !r.start.Equal(wantStart) || !r.end.Equal(wantEnd) {
		t.Fatalf("range = %+v, want date [%s, %s)", r, wantStart, wantEnd)
	}
}

func TestMessageFilterCountsExcludedMessageOnce(t *testing.T) {
	filter := newMessageFilter([]string{"shushing_face"}, []string{"speak_no_evil"})
	message := slack.Message{TS: "1700000001.000000", Text: "private :shushing_face:"}
	if filter.Include(&message) || filter.Include(&message) {
		t.Fatal("Include returned true for excluded message")
	}
	if got := filter.ExcludedCount(); got != 1 {
		t.Fatalf("ExcludedCount = %d, want 1", got)
	}
}

func TestMessageFilterMatchesNormalizedReactionName(t *testing.T) {
	filter := newMessageFilter(nil, []string{"+1", "do_not_archive"})
	for _, reaction := range []string{"+1::skin-tone-3", "DO_NOT_ARCHIVE"} {
		message := slack.Message{
			TS:        reaction,
			Reactions: []slack.Reaction{{Name: reaction, Count: 1}},
		}
		if filter.Include(&message) {
			t.Fatalf("Include returned true for reaction %q", reaction)
		}
	}
	if got := filter.ExcludedCount(); got != 2 {
		t.Fatalf("ExcludedCount = %d, want 2", got)
	}
}

func TestResolveFetchRangeDaysUsesAbsoluteStartAndEnd(t *testing.T) {
	now := time.Date(2026, 7, 12, 13, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	r, err := resolveFetchRange(Options{Days: 30}, now)
	if err != nil {
		t.Fatalf("resolveFetchRange: %v", err)
	}
	if want := now.Add(-30 * 24 * time.Hour); !r.start.Equal(want) {
		t.Fatalf("start = %s, want %s", r.start, want)
	}
	if !r.end.Equal(now) {
		t.Fatalf("end = %s, want %s", r.end, now)
	}
}

func TestResolveDateTimeFetchRangePreservesAbsoluteInstants(t *testing.T) {
	local := time.FixedZone("JST", 9*60*60)
	r, err := resolveDateTimeFetchRange("2026/07/03 09:30", "2026-07-03T03:00:00Z", local)
	if err != nil {
		t.Fatalf("resolveDateTimeFetchRange: %v", err)
	}
	if r.mode != "datetime-range" {
		t.Fatalf("mode = %q, want datetime-range", r.mode)
	}
	if got, want := r.start.Format(time.RFC3339), "2026-07-03T09:30:00+09:00"; got != want {
		t.Fatalf("start = %s, want %s", got, want)
	}
	if got, want := r.end.UTC().Format(time.RFC3339), "2026-07-03T03:00:00Z"; got != want {
		t.Fatalf("end = %s, want %s", got, want)
	}
	if got, want := r.progressLabel(), "from 2026-07-03T00:30:00Z (included) to 2026-07-03T03:00:00Z (not included)"; got != want {
		t.Fatalf("progress label = %q, want %q", got, want)
	}
	if got, want := r.footerRangeLabel(), "From 2026-07-03T00:30:00Z (included); to 2026-07-03T03:00:00Z (not included)"; got != want {
		t.Fatalf("footer range = %q, want %q", got, want)
	}
}

func TestResolveDateTimeFetchRangeRejectsEmptyOrReversedRange(t *testing.T) {
	for _, to := range []string{"2026-07-03T09:30", "2026-07-03T09:00"} {
		if _, err := resolveDateTimeFetchRange("2026-07-03T09:30", to, time.Local); err == nil {
			t.Fatalf("resolveDateTimeFetchRange with to=%q succeeded, want error", to)
		}
	}
}

func TestResolveDateFetchRangeNormalizesParsedInstantToLocalDay(t *testing.T) {
	local := time.FixedZone("JST", 9*60*60)
	tests := []struct {
		name      string
		input     string
		wantStart string
	}{
		{name: "loose local input", input: "2026/07/03 09:30", wantStart: "2026-07-03T00:00:00+09:00"},
		{name: "offset input crossing local midnight", input: "2026-07-03T16:30:15-07:00", wantStart: "2026-07-04T00:00:00+09:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := resolveDateFetchRange(tt.input, local)
			if err != nil {
				t.Fatalf("resolveDateFetchRange: %v", err)
			}
			if got := r.start.Format(time.RFC3339); got != tt.wantStart {
				t.Fatalf("start = %s, want %s", got, tt.wantStart)
			}
			if !r.end.Equal(r.start.AddDate(0, 0, 1)) {
				t.Fatalf("end = %s, want next local day", r.end)
			}
		})
	}
}

func TestChooseChannel(t *testing.T) {
	channels := testChannels(
		"C001", "general",
		"C002", "engineering",
		"C003", "engineering-notify",
		"G004", "engineering-private",
	)

	tests := []struct {
		name     string
		channels []slack.Channel
		opts     Options
		wantID   string
		wantErr  bool
		wantLogs []string
	}{
		{
			name:   "channel id match",
			opts:   Options{ChannelKeyword: "C002"},
			wantID: "C002",
		},
		{
			name:   "channel name exact match",
			opts:   Options{ChannelKeyword: "#general"},
			wantID: "C001",
		},
		{
			name:   "single partial match",
			opts:   Options{ChannelKeyword: "notify"},
			wantID: "C003",
		},
		{
			name:    "multiple partial matches non interactive",
			opts:    Options{ChannelKeyword: "engin"},
			wantErr: true,
			wantLogs: []string{
				`Multiple channels matched "engin".`,
				"Workspace: Example Workspace",
				"Candidates:",
				"#engineering (C002, public, active, member)",
				"#engineering-notify (C003, public, active, member)",
				"#engineering-private (G004, public, active, member)",
				"Run again with a more specific channel:",
				"slapex C002",
			},
		},
		{
			name:    "multiple partial matches no interactive",
			opts:    Options{ChannelKeyword: "engin", PromptTTY: os.Stdin, NoInteractive: true},
			wantErr: true,
			wantLogs: []string{
				`Multiple channels matched "engin".`,
				"Candidates:",
				"slapex C002",
			},
		},
		{
			name:     "too many partial matches",
			channels: numberedChannels(11, "eng"),
			opts:     Options{ChannelKeyword: "eng", PromptTTY: os.Stdin},
			wantErr:  true,
			wantLogs: []string{
				"11 channels matched. Run again with a more specific channel name or a channel ID.",
			},
		},
		{
			name:    "no match",
			opts:    Options{ChannelKeyword: "missing"},
			wantErr: true,
		},
		{
			name:    "empty keyword non interactive",
			opts:    Options{},
			wantErr: true,
			wantLogs: []string{
				"No channel specified. Select one of the following channels:",
				"Candidates:",
				"#general (C001, public, active, member)",
				"slapex C001",
			},
		},
		{
			name:     "empty keyword single candidate non interactive",
			channels: testChannels("C001", "general"),
			opts:     Options{},
			wantErr:  true,
			wantLogs: []string{
				"No channel specified. Select one of the following channels:",
				"Candidates:",
				"#general (C001, public, active, member)",
				"slapex C001",
			},
		},
		{
			name:     "empty keyword too many candidates",
			channels: numberedChannels(11, "channel"),
			opts:     Options{},
			wantErr:  true,
			wantLogs: []string{
				"11 channels matched. Run again with a more specific channel name or a channel ID.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testChannels := channels
			if tt.channels != nil {
				testChannels = tt.channels
			}
			var logs []string
			got, err := chooseChannel(testChannels, tt.opts, "Example Workspace", testPrinter(func(line string) {
				logs = append(logs, line)
			}))
			if tt.wantErr {
				var usage *UsageError
				if !errors.As(err, &usage) {
					t.Fatalf("chooseChannel() error = %v, want UsageError", err)
				}
				for _, want := range tt.wantLogs {
					if !containsLog(logs, want) {
						t.Fatalf("chooseChannel() logs = %q, want entry containing %q", logs, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("chooseChannel() returned error: %v", err)
			}
			if got.ID != tt.wantID {
				t.Fatalf("chooseChannel() ID = %q, want %q", got.ID, tt.wantID)
			}
			if len(logs) != 0 {
				t.Fatalf("chooseChannel() logs = %q, want none", logs)
			}
		})
	}
}

func TestThreadParticipants(t *testing.T) {
	t.Parallel()

	replies := []*render.MessageView{
		{Author: "Alice", AvatarPath: "avatars/alice.png", AvatarInitial: "A"},
		{Author: "Bob", AvatarPath: "avatars/bob.png", AvatarInitial: "B"},
		{Author: "Alice", AvatarPath: "avatars/alice.png", AvatarInitial: "A"},
		{Author: "Carol", AvatarPath: "avatars/carol.png", AvatarInitial: "C"},
		{Author: "Dave", AvatarPath: "avatars/dave.png", AvatarInitial: "D"},
		{IsSystem: true, Author: "system", AvatarInitial: "?"},
	}

	got, extra := threadParticipants(replies)
	if extra != 1 {
		t.Fatalf("extra = %d, want 1", extra)
	}
	if len(got) != 3 {
		t.Fatalf("participants len = %d, want 3", len(got))
	}
	for i, want := range []string{"Alice", "Bob", "Carol"} {
		if got[i].Author != want {
			t.Fatalf("participants[%d].Author = %q, want %q", i, got[i].Author, want)
		}
	}
}

func TestChannelLine(t *testing.T) {
	tests := []struct {
		name string
		ch   slack.Channel
		want string
	}{
		{
			name: "public active member",
			ch:   slack.Channel{ID: "C001", Name: "general", IsMember: true},
			want: "#general (C001, public, active, member)",
		},
		{
			name: "private archived not member",
			ch:   slack.Channel{ID: "G002", Name: "secret", IsPrivate: true, IsArchived: true},
			want: "#secret (G002, private, archived, not member)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := channelLine(tt.ch); got != tt.want {
				t.Fatalf("channelLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChannelURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		workspaceURL string
		channelID    string
		want         string
	}{
		{
			name:         "builds archive URL",
			workspaceURL: "https://acme.example.slack.com/",
			channelID:    "C123",
			want:         "https://acme.example.slack.com/archives/C123",
		},
		{
			name:         "workspace URL without trailing slash",
			workspaceURL: "https://acme.example.slack.com",
			channelID:    "C123",
			want:         "https://acme.example.slack.com/archives/C123",
		},
		{
			name:         "missing workspace URL",
			workspaceURL: "",
			channelID:    "C123",
			want:         "",
		},
		{
			name:         "missing channel ID",
			workspaceURL: "https://acme.example.slack.com/",
			channelID:    "",
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := channelURL(tt.workspaceURL, tt.channelID); got != tt.want {
				t.Fatalf("channelURL(%q, %q) = %q, want %q", tt.workspaceURL, tt.channelID, got, tt.want)
			}
		})
	}
}

func testChannels(pairs ...string) []slack.Channel {
	channels := make([]slack.Channel, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		channels = append(channels, slack.Channel{ID: pairs[i], Name: pairs[i+1], IsMember: true})
	}
	return channels
}

func numberedChannels(n int, prefix string) []slack.Channel {
	channels := make([]slack.Channel, 0, n)
	for i := 1; i <= n; i++ {
		channels = append(channels, slack.Channel{
			ID:       fmt.Sprintf("C%03d", i),
			Name:     fmt.Sprintf("%s-%02d", prefix, i),
			IsMember: true,
		})
	}
	return channels
}

func containsLog(logs []string, want string) bool {
	for _, log := range logs {
		if strings.Contains(log, want) {
			return true
		}
	}
	return false
}
