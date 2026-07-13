package emoji

import (
	"fmt"
	"regexp"
	"strings"
)

var shortcodePattern = regexp.MustCompile(`:([a-zA-Z0-9_+'-]+(?:::skin-tone-[2-6])?):`)
var emojiNamePattern = regexp.MustCompile(`^[a-z0-9_+'-]+(?:::skin-tone-[2-6])?$`)

// ParseList parses and normalizes a comma-separated list of Slack emoji names.
// The returned names omit surrounding colons, skin tone suffixes and duplicates.
func ParseList(input string) ([]string, error) {
	parts := strings.Split(input, ",")
	names := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && strings.HasPrefix(part, ":") && strings.HasSuffix(part, ":") {
			part = part[1 : len(part)-1]
		}
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" || !emojiNamePattern.MatchString(part) {
			return nil, fmt.Errorf("invalid emoji name %q", strings.TrimSpace(part))
		}
		part = normalizeName(part)
		if !seen[part] {
			seen[part] = true
			names = append(names, part)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("emoji list is empty")
	}
	return names, nil
}

func normalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if i := strings.Index(name, "::skin-tone-"); i >= 0 {
		name = name[:i]
	}
	return name
}

// NameSet matches normalized Slack emoji names and message body shortcodes.
// MatchesName is intentionally independent of message text so reaction filters
// can reuse the same normalization in a later option.
type NameSet map[string]struct{}

func NewNameSet(names []string) NameSet {
	set := make(NameSet, len(names))
	for _, name := range names {
		set[normalizeName(name)] = struct{}{}
	}
	return set
}

func (s NameSet) MatchesName(name string) bool {
	if len(name) >= 2 && strings.HasPrefix(name, ":") && strings.HasSuffix(name, ":") {
		name = name[1 : len(name)-1]
	}
	_, ok := s[normalizeName(name)]
	return ok
}

func (s NameSet) MatchesText(text string) bool {
	for _, match := range shortcodePattern.FindAllStringSubmatch(text, -1) {
		if s.MatchesName(match[1]) {
			return true
		}
	}
	return false
}
