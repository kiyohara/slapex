package export

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/render"
	"github.com/kiyohara/slapex/internal/slack"
)

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
