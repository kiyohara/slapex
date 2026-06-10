package output

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxLabelLen = 64

// forbidden characters per doc/design/output-format.md (0029): path
// separators, Windows-reserved characters, whitespace and control characters.
func isForbidden(r rune) bool {
	switch r {
	case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
		return true
	}
	return unicode.IsSpace(r) || unicode.IsControl(r)
}

// Slug normalizes a display name into a directory label per the rules in
// doc/design/output-format.md. It returns "" when nothing usable remains;
// callers fall back to IDs in that case.
func Slug(name string) string {
	name = norm.NFC.String(name)
	var sb strings.Builder
	for _, r := range name {
		if isForbidden(r) {
			sb.WriteRune('-')
		} else {
			sb.WriteRune(r)
		}
	}
	collapsed := strings.Join(strings.FieldsFunc(sb.String(), func(r rune) bool { return r == '-' }), "-")
	runes := []rune(collapsed)
	if len(runes) > maxLabelLen {
		runes = runes[:maxLabelLen]
		collapsed = strings.Trim(string(runes), "-")
	}
	return collapsed
}

// WorkspaceLabel picks the directory label for a workspace: domain part of
// the workspace URL first, then the normalized team name, then the team ID.
func WorkspaceLabel(workspaceURL, teamName, teamID string) string {
	if domain := subdomain(workspaceURL); domain != "" {
		return domain
	}
	if s := Slug(teamName); s != "" {
		return s
	}
	return teamID
}

// ChannelLabel picks the directory label for a channel, falling back to the
// channel ID when normalization leaves nothing.
func ChannelLabel(channelName, channelID string) string {
	if s := Slug(channelName); s != "" {
		return s
	}
	return channelID
}

// subdomain extracts "example" from "https://example.slack.com/".
func subdomain(workspaceURL string) string {
	host := workspaceURL
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	if i := strings.Index(host, "."); i > 0 {
		return host[:i]
	}
	return ""
}
