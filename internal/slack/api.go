package slack

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// AuthTest is the auth.test result used to resolve the workspace.
type AuthTest struct {
	URL    string `json:"url"`
	Team   string `json:"team"`
	TeamID string `json:"team_id"`
	User   string `json:"user"`
	UserID string `json:"user_id"`
	BotID  string `json:"bot_id"`
}

func (c *Client) AuthTest(ctx context.Context) (*AuthTest, error) {
	var out AuthTest
	if _, err := c.call(ctx, "auth.test", url.Values{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TeamInfo is the team.info result used for workspace display details.
type TeamInfo struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Domain string   `json:"domain"`
	Icon   TeamIcon `json:"icon"`
}

// TeamIcon is the subset of team.info icon URLs slapex may render.
type TeamIcon struct {
	Image34      string `json:"image_34"`
	Image44      string `json:"image_44"`
	Image68      string `json:"image_68"`
	Image88      string `json:"image_88"`
	Image102     string `json:"image_102"`
	Image132     string `json:"image_132"`
	Image230     string `json:"image_230"`
	ImageDefault bool   `json:"image_default"`
}

func (c *Client) TeamInfo(ctx context.Context) (*TeamInfo, error) {
	var out struct {
		Team TeamInfo `json:"team"`
	}
	if _, err := c.call(ctx, "team.info", url.Values{}, &out); err != nil {
		return nil, err
	}
	return &out.Team, nil
}

// Channel is the subset of the conversations.list item slapex uses.
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	IsMember   bool   `json:"is_member"`
}

func (c *Client) ListChannels(ctx context.Context) ([]Channel, error) {
	var channels []Channel
	cursor := ""
	for {
		params := url.Values{
			"types":            {"public_channel,private_channel"},
			"exclude_archived": {"false"},
			"limit":            {strconv.Itoa(pageLimit)},
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		var page struct {
			Channels []Channel `json:"channels"`
		}
		next, err := c.call(ctx, "conversations.list", params, &page)
		if err != nil {
			return nil, err
		}
		channels = append(channels, page.Channels...)
		if next == "" {
			return channels, nil
		}
		cursor = next
	}
}

// Message is the subset of a Slack message slapex renders.
type Message struct {
	Type       string `json:"type"`
	Subtype    string `json:"subtype"`
	TS         string `json:"ts"`
	ThreadTS   string `json:"thread_ts"`
	User       string `json:"user"`
	Inviter    string `json:"inviter"`
	BotID      string `json:"bot_id"`
	Username   string `json:"username"`
	BotProfile *struct {
		Name string `json:"name"`
	} `json:"bot_profile"`
	Text       string `json:"text"`
	ReplyCount int    `json:"reply_count"`
	Edited     *struct {
		TS string `json:"ts"`
	} `json:"edited"`
	Files       []File       `json:"files"`
	Attachments []Attachment `json:"attachments"`
	Reactions   []Reaction   `json:"reactions"`
}

// IsThreadParent reports whether the message starts a thread on the timeline.
func (m *Message) IsThreadParent() bool {
	return m.ThreadTS != "" && m.ThreadTS == m.TS && m.ReplyCount > 0
}

// File is the subset of the Slack file object slapex uses.
type File struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Mimetype           string `json:"mimetype"`
	Size               int64  `json:"size"`
	Mode               string `json:"mode"` // "tombstone" for deleted files
	IsExternal         bool   `json:"is_external"`
	URLPrivate         string `json:"url_private"`
	URLPrivateDownload string `json:"url_private_download"`
	Permalink          string `json:"permalink"`
	Thumb360           string `json:"thumb_360"`
	Thumb480           string `json:"thumb_480"`
	Thumb160           string `json:"thumb_160"`
	Thumb64            string `json:"thumb_64"`
}

// DownloadURL returns the preferred URL for fetching the original content.
func (f *File) DownloadURL() string {
	if f.URLPrivateDownload != "" {
		return f.URLPrivateDownload
	}
	return f.URLPrivate
}

// ThumbURL returns the preferred thumbnail for inline display.
func (f *File) ThumbURL() string {
	for _, u := range []string{f.Thumb480, f.Thumb360, f.Thumb160, f.Thumb64} {
		if u != "" {
			return u
		}
	}
	return ""
}

// Attachment is the subset of legacy attachments (URL unfurls) slapex uses.
type Attachment struct {
	ServiceName string `json:"service_name"`
	ServiceIcon string `json:"service_icon"`
	Title       string `json:"title"`
	TitleLink   string `json:"title_link"`
	Text        string `json:"text"`
	ImageURL    string `json:"image_url"`
	ThumbURL    string `json:"thumb_url"`
}

// PreviewImageURL returns the unfurl preview image, if any.
func (a *Attachment) PreviewImageURL() string {
	if a.ImageURL != "" {
		return a.ImageURL
	}
	return a.ThumbURL
}

type Reaction struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// History fetches timeline messages in [oldest, latest), up to maxMessages.
// It reports whether the fetch stopped because maxMessages was reached.
func (c *Client) History(ctx context.Context, channelID, oldest, latest string, maxMessages int, progress func(fetched int)) ([]Message, bool, error) {
	var messages []Message
	cursor := ""
	for {
		params := url.Values{
			"channel":   {channelID},
			"oldest":    {oldest},
			"inclusive": {"true"},
			"limit":     {strconv.Itoa(pageLimit)},
		}
		if latest != "" {
			params.Set("latest", latest)
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		var page struct {
			Messages []Message `json:"messages"`
		}
		next, err := c.call(ctx, "conversations.history", params, &page)
		if err != nil {
			return nil, false, err
		}
		for _, m := range page.Messages {
			if !timestampInRange(m.TS, oldest, latest) {
				continue
			}
			if len(messages) >= maxMessages {
				return messages, true, nil
			}
			messages = append(messages, m)
		}
		if progress != nil {
			progress(len(messages))
		}
		if next == "" {
			return messages, false, nil
		}
		cursor = next
	}
}

func timestampInRange(ts, oldest, latest string) bool {
	// An unbounded caller continues to rely on conversations.history for the
	// lower-bound filtering. A bounded range is checked again locally so an
	// inclusive API response cannot leak its exact end boundary into the export.
	if latest == "" {
		return true
	}
	value, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return false
	}
	if oldest != "" {
		start, err := strconv.ParseFloat(oldest, 64)
		if err == nil && value < start {
			return false
		}
	}
	if latest != "" {
		end, err := strconv.ParseFloat(latest, 64)
		if err == nil && value >= end {
			return false
		}
	}
	return true
}

// Replies fetches thread replies (excluding the parent message itself), up to
// maxReplies. It reports whether the thread was truncated at the limit.
func (c *Client) Replies(ctx context.Context, channelID, threadTS string, maxReplies int) ([]Message, bool, error) {
	var replies []Message
	cursor := ""
	for {
		params := url.Values{
			"channel": {channelID},
			"ts":      {threadTS},
			"limit":   {strconv.Itoa(pageLimit)},
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		var page struct {
			Messages []Message `json:"messages"`
		}
		next, err := c.call(ctx, "conversations.replies", params, &page)
		if err != nil {
			return nil, false, err
		}
		for _, m := range page.Messages {
			if m.TS == threadTS {
				continue // the parent appears in the replies response
			}
			if len(replies) >= maxReplies {
				return replies, true, nil
			}
			replies = append(replies, m)
		}
		if next == "" {
			return replies, false, nil
		}
		cursor = next
	}
}

// User is the subset of users.info slapex uses for display names and avatars.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RealName string `json:"real_name"`
	Deleted  bool   `json:"deleted"`
	IsBot    bool   `json:"is_bot"`
	Profile  struct {
		DisplayName string `json:"display_name"`
		RealName    string `json:"real_name"`
		Image48     string `json:"image_48"`
		Image72     string `json:"image_72"`
	} `json:"profile"`
}

// DisplayName resolves the name to show, falling back to the user ID.
func (u *User) DisplayName() string {
	for _, name := range []string{u.Profile.DisplayName, u.Profile.RealName, u.RealName, u.Name} {
		if name != "" {
			return name
		}
	}
	return u.ID
}

func (c *Client) UserInfo(ctx context.Context, userID string) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	if _, err := c.call(ctx, "users.info", url.Values{"user": {userID}}, &out); err != nil {
		return nil, err
	}
	return &out.User, nil
}

// EmojiList returns the workspace custom emoji map (name -> URL or alias:name).
func (c *Client) EmojiList(ctx context.Context) (map[string]string, error) {
	var out struct {
		Emoji map[string]string `json:"emoji"`
	}
	if _, err := c.call(ctx, "emoji.list", url.Values{}, &out); err != nil {
		return nil, err
	}
	return out.Emoji, nil
}

// FormatTS renders a time as a Slack ts parameter value.
func FormatTS(sec int64) string {
	return fmt.Sprintf("%d.000000", sec)
}
