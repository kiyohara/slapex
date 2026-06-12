package output

import (
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", maxLabelLen+10)
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "normalizes to NFC",
			in:   "Cafe\u0301",
			want: "Café",
		},
		{
			name: "replaces forbidden whitespace and control characters",
			in:   "a/b\\c:d*e?f\"g<h>i|j k\tl\nm" + string(rune(0x7f)) + "n",
			want: "a-b-c-d-e-f-g-h-i-j-k-l-m-n",
		},
		{
			name: "compresses and trims separators",
			in:   " / alpha   beta// ",
			want: "alpha-beta",
		},
		{
			name: "truncates to max label length",
			in:   long,
			want: strings.Repeat("a", maxLabelLen),
		},
		{
			name: "keeps unicode letters",
			in:   "開発-連絡",
			want: "開発-連絡",
		},
		{
			name: "returns empty when no usable runes remain",
			in:   " / \t\n| ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Slug(tt.in); got != tt.want {
				t.Fatalf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWorkspaceLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		workspaceURL string
		teamName     string
		teamID       string
		want         string
	}{
		{
			name:         "uses slack subdomain first",
			workspaceURL: "https://example.slack.com/",
			teamName:     "Ignored Team",
			teamID:       "Tignored",
			want:         "example",
		},
		{
			name:         "normalizes team name when domain is unavailable",
			workspaceURL: "",
			teamName:     "開発 Team",
			teamID:       "Tfallback",
			want:         "開発-Team",
		},
		{
			name:         "falls back to team id",
			workspaceURL: "",
			teamName:     " / \t ",
			teamID:       "T123456",
			want:         "T123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := WorkspaceLabel(tt.workspaceURL, tt.teamName, tt.teamID)
			if got != tt.want {
				t.Fatalf("WorkspaceLabel(%q, %q, %q) = %q, want %q", tt.workspaceURL, tt.teamName, tt.teamID, got, tt.want)
			}
		})
	}
}

func TestChannelLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		channelName string
		channelID   string
		want        string
	}{
		{
			name:        "normalizes channel name",
			channelName: "dev / random",
			channelID:   "Cignored",
			want:        "dev-random",
		},
		{
			name:        "falls back to channel id",
			channelName: " / \n ",
			channelID:   "C123456",
			want:        "C123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ChannelLabel(tt.channelName, tt.channelID)
			if got != tt.want {
				t.Fatalf("ChannelLabel(%q, %q) = %q, want %q", tt.channelName, tt.channelID, got, tt.want)
			}
		})
	}
}
