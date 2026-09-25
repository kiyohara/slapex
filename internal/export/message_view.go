// Message view building: converts fetched slack.Message values into the
// render.MessageView tree (author, avatar, body, files, unfurls, reactions,
// thread participants) while saving the assets they reference
// (doc/design/html-rendering.md, doc/design/output-format.md).

package export

import (
	"fmt"
	"html"
	"html/template"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/render"
	"github.com/kiyohara/slapex/internal/slack"
)

var systemSubtypes = map[string]bool{
	"channel_join":      true,
	"channel_leave":     true,
	"channel_topic":     true,
	"channel_purpose":   true,
	"channel_name":      true,
	"channel_archive":   true,
	"channel_unarchive": true,
	"pinned_item":       true,
}

var actorPrefixSystemSubtypes = map[string]bool{
	"channel_topic":   true,
	"channel_purpose": true,
	"channel_name":    true,
}

var normalSubtypes = map[string]bool{
	"":                 true,
	"file_share":       true,
	"thread_broadcast": true,
	"bot_message":      true,
	"me_message":       true,
}

// messageKind is how messageView shows a message, following
// html-rendering.md「メッセージ種別(subtype)の表示」. collectUserIDs reads the
// same kind to scan only the texts that are rendered.
type messageKind int

const (
	messageFull        messageKind = iota // author, body, files, unfurls and reactions
	messageSystem                         // a one-line system row built from the body (systemBody)
	messageTombstone                      // the deleted-message placeholder
	messageUnsupported                    // a system row naming a bodiless unknown subtype
)

func messageKindOf(m *slack.Message) messageKind {
	switch {
	case systemSubtypes[m.Subtype]:
		return messageSystem
	case m.Subtype == "tombstone":
		return messageTombstone
	case !normalSubtypes[m.Subtype] && m.Text == "":
		return messageUnsupported
	default:
		return messageFull
	}
}

type messageViewBuilder struct {
	users              map[string]*slack.User
	avatars            map[string]string
	bots               map[string]*slack.Bot
	botAvatars         map[string]string
	emoji              *emoji.Resolver
	assets             *output.Assets
	maxAttachmentBytes int64
}

// UserName implements render.TextResolver.
func (b *messageViewBuilder) UserName(id string) string {
	if u, ok := b.users[id]; ok {
		return u.DisplayName()
	}
	return id
}

// EmojiHTML implements render.TextResolver.
func (b *messageViewBuilder) EmojiHTML(name string) string {
	literal := html.EscapeString(":" + name + ":")
	r := b.emoji.Resolve(name)
	switch {
	case r.Unicode != "":
		return r.Unicode
	case r.ImageURL != "":
		if rel, ok := b.assets.Save(output.KindEmoji, r.ImageURL, output.AssetMeta{EmojiName: name}); ok {
			return `<img class="emoji" src="` + rel + `" alt="` + literal + `" title="` + literal + `">`
		}
		return literal
	default:
		return literal
	}
}

func (b *messageViewBuilder) messageView(m *slack.Message) *render.MessageView {
	t := tsTime(m.TS)
	v := &render.MessageView{
		TimeLabel: t.Format("2006-01-02 15:04"),
		TimeISO:   t.UTC().Format(time.RFC3339),
		Edited:    m.Edited != nil,
	}

	switch messageKindOf(m) {
	case messageSystem:
		v.IsSystem = true
		v.Body = b.systemBody(m)
		return v
	case messageTombstone:
		v.Author = "(削除)"
		v.AvatarInitial = "?"
		v.Body = render.Safe("(削除されたメッセージ)")
		return v
	case messageUnsupported:
		v.IsSystem = true
		v.Body = render.Safe("(未対応のメッセージ種別: " + html.EscapeString(m.Subtype) + ")")
		return v
	}

	v.Author = b.authorName(m)
	v.AvatarPath = b.authorAvatar(m)
	v.AvatarInitial = initialOf(v.Author)
	v.IsBot = isBotMessage(m, b.users[m.User])
	v.Italic = m.Subtype == "me_message"
	if m.Text != "" {
		v.Body = render.Mrkdwn(m.Text, b)
	}
	b.addFiles(v, m)
	b.addUnfurls(v, m)
	for _, r := range m.Reactions {
		v.Reactions = append(v.Reactions, render.ReactionView{
			Emoji: render.Safe(b.EmojiHTML(r.Name)),
			Count: r.Count,
		})
	}
	return v
}

func (b *messageViewBuilder) systemBody(m *slack.Message) template.HTML {
	body := render.Mrkdwn(m.Text, b)
	if suffix, ok := b.channelJoinInviterSuffix(m); ok {
		body += suffix
	}
	name, ok := b.userDisplayName(m.User)
	if !ok || !actorPrefixSystemSubtypes[m.Subtype] || systemTextStartsWithActor(m.Text, m.User, name) {
		return body
	}
	prefix := `<span class="mention">` + html.EscapeString("@"+name) + `</span> `
	return render.Safe(prefix) + body
}

func (b *messageViewBuilder) channelJoinInviterSuffix(m *slack.Message) (template.HTML, bool) {
	if m.Subtype != "channel_join" || m.Inviter == "" || m.Inviter == m.User || systemTextMentionsUser(m.Text, m.Inviter) {
		return "", false
	}
	name, ok := b.userDisplayName(m.Inviter)
	if !ok {
		return "", false
	}
	suffix := ` <span class="system-context">(invited by <span class="mention">` + html.EscapeString("@"+name) + `</span>)</span>`
	return render.Safe(suffix), true
}

// authorName resolves the displayed poster name. For a bot message the inline
// bot_profile / username overrides come first, then the bots.info name, and only
// then the raw bot_id (decision log 0054).
func (b *messageViewBuilder) authorName(m *slack.Message) string {
	if m.User != "" {
		return b.UserName(m.User)
	}
	if m.BotProfile != nil && m.BotProfile.Name != "" {
		return m.BotProfile.Name
	}
	if m.Username != "" {
		return m.Username
	}
	if bot, ok := b.bots[m.BotID]; ok && bot.Name != "" {
		return bot.Name
	}
	if m.BotID != "" {
		return m.BotID
	}
	return "(unknown)"
}

// authorAvatar resolves the saved avatar for a message: the poster's users.info
// image, else the inline bot_profile app icon, else the bots.info app icon. An
// empty result leaves the initial fallback in place (decision log 0035 / 0054).
// Both bot icons are saved on demand; output.Assets deduplicates by source URL,
// so repeated posts from the same app download once.
func (b *messageViewBuilder) authorAvatar(m *slack.Message) string {
	if m.User != "" {
		return b.avatars[m.User]
	}
	if m.BotProfile != nil {
		if rel, ok := b.assets.Save(output.KindAvatar, m.BotProfile.Icons.URL(), output.AssetMeta{}); ok {
			return rel
		}
	}
	return b.botAvatars[m.BotID]
}

// isBotMessage reports whether a message was posted by an app rather than a
// person, so the renderer can show the APP chip (decision log 0054). u is the
// resolved poster, if any: a bot user posting through chat.postMessage carries a
// normal user ID and is only recognizable by users.info is_bot. Slackbot is not
// an app and reports is_bot false, so it is correctly left out.
func isBotMessage(m *slack.Message, u *slack.User) bool {
	if m.BotID != "" || m.BotProfile != nil {
		return true
	}
	return u != nil && u.IsBot
}

func (b *messageViewBuilder) userDisplayName(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	u, ok := b.users[id]
	if !ok {
		return "", false
	}
	return u.DisplayName(), true
}

func systemTextStartsWithActor(text, userID, displayName string) bool {
	for _, prefix := range []string{
		"<@" + userID + ">",
		"<@" + userID + "|",
		"@" + displayName,
	} {
		if prefix != "" && strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func systemTextMentionsUser(text, userID string) bool {
	return userID != "" && (strings.Contains(text, "<@"+userID+">") || strings.Contains(text, "<@"+userID+"|"))
}

func (b *messageViewBuilder) addFiles(v *render.MessageView, m *slack.Message) {
	for i := range m.Files {
		f := &m.Files[i]
		switch {
		case f.Mode == "tombstone":
			v.FilesList = append(v.FilesList, render.FileView{Name: "(削除されたファイル)"})
		case strings.HasPrefix(f.Mimetype, "image/"):
			b.addImage(v, f)
		default:
			b.addAttachmentFile(v, f)
		}
	}
}

func (b *messageViewBuilder) addImage(v *render.MessageView, f *slack.File) {
	meta := output.AssetMeta{FileID: f.ID, OriginalName: f.Name, Mimetype: f.Mimetype, SizeBytes: f.Size}
	thumbURL := f.ThumbURL()
	if thumbURL == "" && f.IsExternal {
		v.FilesList = append(v.FilesList, render.FileView{Name: f.Name, Note: "(外部サービス連携の画像のため保存対象外)"})
		return
	}
	img := render.ImageView{Name: f.Name}
	if thumbURL != "" {
		// Slack's mimetype and size describe the original, not the thumbnail:
		// leave them out so the thumbnail's manifest entry takes both from its
		// own download (Issue #208).
		thumbMeta := output.AssetMeta{FileID: f.ID, OriginalName: f.Name}
		img.ThumbPath, _ = b.assets.Save(output.KindUploadThumb, thumbURL, thumbMeta)
	}
	// oversize marks an original the size limit kept out, and origSize is the
	// size shown for it (see oversizeFileNote).
	oversize, origSize := false, int64(0)
	switch {
	case b.maxAttachmentBytes > 0 && f.Size > b.maxAttachmentBytes:
		b.assets.SkipTooLarge(output.KindUploadOriginal, f.DownloadURL(), meta)
		oversize, origSize = true, f.Size
	case f.DownloadURL() != "":
		if rel, ok := b.assets.Save(output.KindUploadOriginal, f.DownloadURL(), meta); ok {
			img.OriginalPath = rel
		} else if b.assets.Status(f.DownloadURL()) == output.StatusSkippedSize {
			// file.size was absent or understated, so the pre-check let the
			// original through and the download hit the limit (Issue #203).
			oversize = true
		} else {
			img.Note = "original の取得に失敗しました。"
		}
	}
	if oversize {
		img.Note = b.oversizeOriginalNote(f.Name, origSize)
	}
	if img.ThumbPath == "" && img.OriginalPath != "" {
		img.ThumbPath = img.OriginalPath
	}
	if img.ThumbPath == "" {
		note := "画像の取得に失敗しました。"
		if oversize {
			// No thumbnail to keep the note under: show the image like an
			// oversize attachment, so its size and the limit still show
			// (html-rendering.md「画像と添付ファイルの表示」).
			note = b.oversizeFileNote(f.ID, origSize)
		}
		v.FilesList = append(v.FilesList, render.FileView{Name: f.Name, Note: note})
		return
	}
	v.Images = append(v.Images, img)
}

func (b *messageViewBuilder) addAttachmentFile(v *render.MessageView, f *slack.File) {
	meta := output.AssetMeta{FileID: f.ID, OriginalName: f.Name, Mimetype: f.Mimetype, SizeBytes: f.Size}
	name := f.Name
	if name == "" {
		name = f.ID
	}
	switch {
	case f.IsExternal || f.DownloadURL() == "":
		v.FilesList = append(v.FilesList, render.FileView{Name: name, Note: "(外部サービス連携のファイルのため保存対象外)"})
	case b.maxAttachmentBytes > 0 && f.Size > b.maxAttachmentBytes:
		b.assets.SkipTooLarge(output.KindAttachment, f.DownloadURL(), meta)
		v.FilesList = append(v.FilesList, render.FileView{Name: name, Note: b.oversizeFileNote(f.ID, f.Size)})
	default:
		if rel, ok := b.assets.Save(output.KindAttachment, f.DownloadURL(), meta); ok {
			v.FilesList = append(v.FilesList, render.FileView{Name: name, Path: rel})
		} else if b.assets.Status(f.DownloadURL()) == output.StatusSkippedSize {
			// file.size was absent or understated, so the pre-check let the
			// file through and the download hit the limit (Issue #203).
			v.FilesList = append(v.FilesList, render.FileView{Name: name, Note: b.oversizeFileNote(f.ID, 0)})
		} else {
			v.FilesList = append(v.FilesList, render.FileView{Name: name, Note: "取得に失敗しました。"})
		}
	}
}

// oversizeFileNote is the note shown in place of a file the size limit kept
// out: the Slack file ID when there is one, the original size and the limit,
// next to the file name in FileView.Name (output-format.md「添付ファイルの
// サイズ制限」). size is Slack's file.size when the pre-check caught the file,
// or 0 when the download itself hit the limit: file.size was absent or
// understated there, so the real size is unknown and is left out.
func (b *messageViewBuilder) oversizeFileNote(fileID string, size int64) string {
	detail := "上限 " + humanBytes(b.maxAttachmentBytes)
	if size > 0 {
		detail = humanBytes(size) + ", " + detail
	}
	if fileID != "" {
		detail = "file ID: " + fileID + ", " + detail
	}
	return "サイズオーバーのため保存されませんでした。(" + detail + ")"
}

// oversizeOriginalNote is the note under an image thumbnail whose original the
// size limit kept out. size follows oversizeFileNote: 0 leaves the unknown
// size out.
func (b *messageViewBuilder) oversizeOriginalNote(name string, size int64) string {
	if size > 0 {
		return fmt.Sprintf("original はサイズ上限超過のため保存されませんでした。(%s: %s, 上限 %s)",
			name, humanBytes(size), humanBytes(b.maxAttachmentBytes))
	}
	return fmt.Sprintf("original はサイズ上限超過のため保存されませんでした。(%s, 上限 %s)",
		name, humanBytes(b.maxAttachmentBytes))
}

func (b *messageViewBuilder) addUnfurls(v *render.MessageView, m *slack.Message) {
	for i := range m.Attachments {
		a := &m.Attachments[i]
		uv := render.UnfurlView{Service: a.ServiceName, Title: a.Title}
		if strings.HasPrefix(a.TitleLink, "http://") || strings.HasPrefix(a.TitleLink, "https://") {
			uv.TitleHref = a.TitleLink
		}
		if a.Text != "" {
			// collectUserIDs scans the same text for mentions; a field newly
			// passed to Mrkdwn needs the same scan there.
			uv.Text = render.Mrkdwn(a.Text, b)
		}
		if a.ServiceIcon != "" {
			uv.ServiceIconPath, _ = b.assets.Save(output.KindServiceIcon, a.ServiceIcon, output.AssetMeta{})
		}
		if src := a.PreviewImageURL(); src != "" {
			uv.ImagePath, _ = b.assets.Save(output.KindOGImage, src, output.AssetMeta{})
		}
		if uv.Service == "" && uv.ServiceIconPath == "" && uv.Title == "" && uv.Text == "" && uv.ImagePath == "" {
			continue
		}
		v.Unfurls = append(v.Unfurls, uv)
	}
}

func threadParticipants(replies []*render.MessageView) ([]render.ThreadParticipantView, int) {
	const maxParticipants = 3

	seen := map[string]bool{}
	var participants []render.ThreadParticipantView
	uniqueCount := 0
	for _, reply := range replies {
		if reply == nil || reply.IsSystem || reply.Author == "" {
			continue
		}
		key := reply.Author + "\x00" + reply.AvatarPath
		if seen[key] {
			continue
		}
		seen[key] = true
		uniqueCount++
		if len(participants) < maxParticipants {
			participants = append(participants, render.ThreadParticipantView{
				Author:        reply.Author,
				AvatarPath:    reply.AvatarPath,
				AvatarInitial: reply.AvatarInitial,
			})
		}
	}
	if uniqueCount <= maxParticipants {
		return participants, 0
	}
	return participants, uniqueCount - maxParticipants
}

func initialOf(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}
