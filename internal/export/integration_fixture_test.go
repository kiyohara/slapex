package export

// Integration-test fixtures. This file holds the scenario the fake Slack
// server serves (exportScenario and the fault / asset types it embeds), the two
// fixture builders every integration case starts from (happyPathScenario /
// baseScenario) and the small constructors for slack.* values the cases
// compose messages from. Every builder returns a fresh value, so parallel tests
// never share fixture state; there are no package-level fixture variables.
//
// Case-specific fixtures stay next to the cases that use them
// (image48AvatarScenario / botReuseScenario in integration_reuse_test.go).
//
// Related files:
//   - integration_fakeserver_test.go: the fake Slack server serving a scenario
//   - integration_harness_test.go:    runExportScenario / Options builders
//   - integration_assert_test.go:     generic HTML / manifest / log helpers

import (
	"github.com/kiyohara/slapex/internal/slack"
)

// exportScenario is everything the fake Slack server needs to answer one
// export: auth / team identity, the channel list, the history page, per-thread
// replies, resolvable users / bots, custom emoji and downloadable assets.
// "{{base}}" in any URL is replaced with the fake server's URL when the server
// is created (exportScenario.replaceBaseURL).
type exportScenario struct {
	Auth     slack.AuthTest
	TeamInfo *slack.TeamInfo
	Channels []slack.Channel
	Messages []slack.Message
	Replies  map[string][]slack.Message
	Users    map[string]slack.User
	Bots     map[string]slack.Bot
	Emoji    map[string]string
	Assets   map[string]fakeAsset

	// APIFaults / AssetFaults inject error and rate-limit behaviour for the
	// v1-09 error scenarios, keyed by request path (e.g.
	// "/api/conversations.history" or "/files/flaky.pdf"). A nil/empty map
	// keeps the happy behaviour, so v1-07 / v1-08 fixtures are unaffected.
	APIFaults   map[string]*endpointFault
	AssetFaults map[string]*endpointFault
}

type fakeAsset struct {
	ContentType string
	Body        string
	RejectAuth  bool
}

// endpointFault injects error / rate-limit behaviour for one fake server
// endpoint (an API path or an asset path) in the v1-09 error scenarios.
type endpointFault struct {
	// transient responses are emitted one per call, in order, before the
	// endpoint falls through to its normal handler. Used for "429 once then
	// succeed" and "5xx then succeed".
	transient []faultResponse
	// sticky, when non-nil, is returned on every call once transient responses
	// are drained. Used for persistent Slack errors (invalid_auth,
	// missing_scope, not_in_channel), a persistent 429 (retry-limit reached)
	// and a persistent download failure.
	sticky *faultResponse
}

// faultResponse is a single fake response. A non-zero httpStatus (429 or 5xx)
// is written directly, with Retry-After taken from retryAfterSec when > 0. An
// httpStatus of 0 with slackError set yields an {"ok":false,...} body;
// otherwise the endpoint's normal handler runs.
type faultResponse struct {
	httpStatus    int
	retryAfterSec int
	slackError    string
}

// happyPathScenario is the full-featured fixture: two channels (one matching
// the "project-alpha" keyword), three timeline messages including one thread
// with two replies, an unfurl attachment, a PDF upload, an image upload with a
// thumbnail, reactions, a custom emoji, a workspace icon and avatars for both
// users. Every asset kind the exporter saves appears once, so the happy-path
// assertions (assertOutputFiles / assertHTMLMarkers / assertCacheFiles) can
// check each of them.
func happyPathScenario() exportScenario {
	return exportScenario{
		Auth: slack.AuthTest{
			URL:    "https://acme.example.slack.com/",
			Team:   "Acme Workspace",
			TeamID: "TACME123",
			User:   "slapex",
			UserID: "USLAPEX",
			BotID:  "BSLAPEX",
		},
		TeamInfo: &slack.TeamInfo{
			ID:     "TACME123",
			Name:   "Acme Workspace",
			Domain: "acme",
			Icon: slack.TeamIcon{
				Image68: "{{base}}/files/workspace-icon.png",
			},
		},
		Channels: []slack.Channel{
			{ID: "C999", Name: "random", IsMember: true},
			{ID: "C123", Name: "project-alpha", IsMember: true},
		},
		Messages: []slack.Message{
			{
				Type: "message",
				TS:   "1700000003.000000",
				User: "U02",
				Text: "Final timeline update",
			},
			{
				Type:       "message",
				TS:         "1700000002.000000",
				ThreadTS:   "1700000002.000000",
				User:       "U01",
				Text:       "Starting the launch thread with :party_sloth: and <@U02>",
				ReplyCount: 2,
				Attachments: []slack.Attachment{
					{
						ServiceName: "Example News",
						ServiceIcon: "{{base}}/files/service-example-news.png",
						Title:       "Launch checklist",
						TitleLink:   "https://example.com/launch-checklist",
						Text:        "Read <@U02>'s notes",
						ImageURL:    "{{base}}/files/og-launch.png",
					},
				},
				Reactions: []slack.Reaction{
					{Name: "smile", Count: 3},
					{Name: "party_sloth", Count: 2},
				},
			},
			{
				Type: "message",
				TS:   "1700000001.000000",
				User: "U02",
				Text: "First timeline note",
				Files: []slack.File{
					{
						ID:                 "F-DOC",
						Name:               "runbook.pdf",
						Mimetype:           "application/pdf",
						Size:               18,
						URLPrivateDownload: "{{base}}/files/runbook.pdf",
					},
				},
				Reactions: []slack.Reaction{{Name: "smile", Count: 1}},
			},
		},
		Replies: map[string][]slack.Message{
			"1700000002.000000": {
				{
					Type:       "message",
					TS:         "1700000002.000000",
					ThreadTS:   "1700000002.000000",
					User:       "U01",
					Text:       "Starting the launch thread with :party_sloth: and <@U02>",
					ReplyCount: 2,
				},
				{
					Type:     "message",
					TS:       "1700000002.200000",
					ThreadTS: "1700000002.000000",
					User:     "U01",
					Text:     "Thread is wrapped up :party_sloth:",
				},
				{
					Type:     "message",
					TS:       "1700000002.100000",
					ThreadTS: "1700000002.000000",
					User:     "U02",
					Text:     "Reply with screenshot",
					Files: []slack.File{
						{
							ID:                 "F-IMG",
							Name:               "screenshot.png",
							Mimetype:           "image/png",
							Size:               32,
							URLPrivateDownload: "{{base}}/files/screenshot-original.png",
							Thumb360:           "{{base}}/files/screenshot-thumb.png",
						},
					},
				},
			},
		},
		Users: map[string]slack.User{
			"U01": testUser("U01", "alice", "Alice Example", "Alice", "{{base}}/files/avatar-u01.png"),
			"U02": testUser("U02", "bob", "Bob Builder", "Bob", "{{base}}/files/avatar-u02.png"),
		},
		Emoji: map[string]string{
			"party_sloth": "{{base}}/files/emoji-party-sloth.png",
		},
		Assets: map[string]fakeAsset{
			"/files/avatar-u01.png":           {ContentType: "image/png", Body: "avatar-u01"},
			"/files/avatar-u02.png":           {ContentType: "image/png", Body: "avatar-u02"},
			"/files/workspace-icon.png":       {ContentType: "image/png", Body: "workspace-icon"},
			"/files/emoji-party-sloth.png":    {ContentType: "image/png", Body: "custom-emoji"},
			"/files/service-example-news.png": {ContentType: "image/png", Body: "service-icon", RejectAuth: true},
			"/files/og-launch.png":            {ContentType: "image/png", Body: "og-image"},
			"/files/runbook.pdf":              {ContentType: "application/pdf", Body: "runbook attachment"},
			"/files/screenshot-original.png":  {ContentType: "image/png", Body: "screenshot original"},
			"/files/screenshot-thumb.png":     {ContentType: "image/png", Body: "screenshot thumb"},
		},
	}
}

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

// --- slack.* value constructors ---------------------------------------------

func testUser(id, name, realName, displayName, imageURL string) slack.User {
	u := slack.User{ID: id, Name: name, RealName: realName}
	u.Profile.DisplayName = displayName
	u.Profile.RealName = realName
	u.Profile.Image48 = imageURL
	u.Profile.Image72 = imageURL
	return u
}

// botProfileName / botProfileFull / editedAt build the bot_profile and edited
// values used by slack.Message.
func botProfileName(name string) *slack.BotProfile {
	return &slack.BotProfile{Name: name}
}

// botProfileFull is a bot_profile carrying both the app name and its icons, the
// case that needs no bots.info call at all (decision log 0054).
func botProfileFull(name, iconURL string) *slack.BotProfile {
	return &slack.BotProfile{Name: name, Icons: slack.BotIcons{Image48: iconURL, Image72: iconURL}}
}

func editedAt(ts string) *struct {
	TS string `json:"ts"`
} {
	return &struct {
		TS string `json:"ts"`
	}{TS: ts}
}

// botIcons builds a bots.info / bot_profile icons block from one URL.
func botIcons(url string) slack.BotIcons {
	return slack.BotIcons{Image48: url, Image72: url}
}

func pngAsset(body string) fakeAsset {
	return fakeAsset{ContentType: "image/png", Body: body}
}
