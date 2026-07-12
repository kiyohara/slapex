// Package demo holds the anonymized sample fixtures and the in-process fake
// Slack API server that serves them. Each fixture is an entirely fictional
// workspace (users, messages, assets); nothing comes from a real Slack
// workspace and no real credential or network access is involved.
//
// The package is shared by two callers:
//
//   - tools/gensample regenerates the committed sample exports under
//     doc/samples/ and, with -serve, keeps one fixture's fake API running for
//     the README demo GIF recording.
//   - cmd/slapex uses it for the user-facing token-free demo run (slapex
//     --demo, Issue #113): it starts a fixture server in-process and runs the
//     real export pipeline against it, so anyone can see the output without
//     creating a Slack App or issuing a token.
package demo

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// Scenario is one fictional workspace fixture. Asset URLs inside the fixture
// use the "{{base}}" placeholder, replaced with the fake server URL once it is
// listening (same convention as the integration-test harness in
// internal/export).
type Scenario struct {
	Lang        string // output subdirectory name ("ja" / "en")
	ChannelName string // exact channel name passed as the channel keyword

	Auth     slack.AuthTest
	TeamInfo *slack.TeamInfo
	Channels []slack.Channel
	Messages []slack.Message
	Replies  map[string][]slack.Message
	Users    map[string]slack.User
	Emoji    map[string]string
	Assets   map[string]Asset
}

// Asset is one fixture file (image or document) served by the fake API.
type Asset struct {
	ContentType string
	Body        []byte
}

// ReplaceBaseURL rewrites every "{{base}}" placeholder in the fixture's asset
// URLs to baseURL, which must be the externally reachable base URL of the fake
// server serving this scenario.
func (sc *Scenario) ReplaceBaseURL(baseURL string) {
	repl := func(s string) string {
		return strings.ReplaceAll(s, "{{base}}", baseURL)
	}
	for i := range sc.Messages {
		replaceMessageBaseURL(&sc.Messages[i], repl)
	}
	for threadTS, replies := range sc.Replies {
		for i := range replies {
			replaceMessageBaseURL(&replies[i], repl)
		}
		sc.Replies[threadTS] = replies
	}
	for id, u := range sc.Users {
		u.Profile.Image48 = repl(u.Profile.Image48)
		u.Profile.Image72 = repl(u.Profile.Image72)
		sc.Users[id] = u
	}
	if sc.TeamInfo != nil {
		sc.TeamInfo.Icon.Image68 = repl(sc.TeamInfo.Icon.Image68)
	}
	for name, rawURL := range sc.Emoji {
		sc.Emoji[name] = repl(rawURL)
	}
}

func replaceMessageBaseURL(m *slack.Message, repl func(string) string) {
	for i := range m.Files {
		m.Files[i].URLPrivate = repl(m.Files[i].URLPrivate)
		m.Files[i].URLPrivateDownload = repl(m.Files[i].URLPrivateDownload)
		m.Files[i].Thumb360 = repl(m.Files[i].Thumb360)
	}
	for i := range m.Attachments {
		m.Attachments[i].ImageURL = repl(m.Attachments[i].ImageURL)
		m.Attachments[i].ServiceIcon = repl(m.Attachments[i].ServiceIcon)
	}
}

// filterRange mirrors conversations.history bounds for demo mode.
func filterRange(messages []slack.Message, oldest, latest string) []slack.Message {
	lo, loErr := strconv.ParseFloat(strings.TrimSpace(oldest), 64)
	hi, hiErr := strconv.ParseFloat(strings.TrimSpace(latest), 64)
	filtered := make([]slack.Message, 0, len(messages))
	for _, m := range messages {
		ts, err := strconv.ParseFloat(m.TS, 64)
		if err != nil || (loErr == nil && ts < lo) || (hiErr == nil && ts >= hi) {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
}

// --- fixture helpers ---------------------------------------------------------

// at returns a time on day at hh:mm:ss in the local timezone.
func at(day time.Time, hh, mm, ss int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hh, mm, ss, 0, time.Local)
}

// ts renders t as a Slack message ts value.
func ts(t time.Time) string { return fmt.Sprintf("%d.000000", t.Unix()) }

func editedAt(t time.Time) *struct {
	TS string `json:"ts"`
} {
	return &struct {
		TS string `json:"ts"`
	}{TS: ts(t)}
}

func botProfile(name string) *struct {
	Name string `json:"name"`
} {
	return &struct {
		Name string `json:"name"`
	}{Name: name}
}

func sampleUser(id, name, realName, displayName, imageURL string) slack.User {
	u := slack.User{ID: id, Name: name, RealName: realName}
	u.Profile.DisplayName = displayName
	u.Profile.RealName = realName
	u.Profile.Image48 = imageURL
	u.Profile.Image72 = imageURL
	return u
}
