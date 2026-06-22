// Package render builds the static index.html / style.css output
// (doc/design/html-rendering.md): Slack-like appearance, no JavaScript,
// styles in a separate stylesheet.
package render

import (
	"embed"
	"html/template"
	"io"
	"os"
	"path/filepath"
)

//go:embed templates/index.html.tmpl
var templateFS embed.FS

//go:embed templates/style.css
var styleCSS []byte

// PageData is the root template input.
type PageData struct {
	WorkspaceName string
	ChannelName   string
	WorkspaceLine string
	ChannelLine   string
	ExportedLine  string
	RangeLine     string
	Items         []TimelineItem
	Truncated     bool // --max-posts reached
}

// TimelineItem is either a date divider or a message.
type TimelineItem struct {
	IsDateDivider bool
	Date          string
	Message       *MessageView
}

// MessageView is one rendered message (timeline or thread reply).
type MessageView struct {
	IsSystem                bool
	Author                  string
	AvatarPath              string
	AvatarInitial           string
	TimeLabel               string
	TimeISO                 string
	Edited                  bool
	Italic                  bool
	Body                    template.HTML
	Images                  []ImageView
	FilesList               []FileView
	Unfurls                 []UnfurlView
	Reactions               []ReactionView
	Replies                 []*MessageView
	RepliesTruncated        bool
	ThreadParticipants      []ThreadParticipantView
	ThreadExtraParticipants int
}

// ThreadParticipantView is one compact avatar shown in a thread summary.
type ThreadParticipantView struct {
	Author        string
	AvatarPath    string
	AvatarInitial string
}

// ImageView is an uploaded image: thumbnail inline, original on click.
type ImageView struct {
	ThumbPath    string
	OriginalPath string // empty: original not saved
	Name         string
	Note         string // shown when the original was skipped
}

// FileView is a non-image attachment (saved, skipped or unavailable).
type FileView struct {
	Name string
	Path string // empty: not saved
	Note string
}

// UnfurlView is a legacy attachment / URL preview.
type UnfurlView struct {
	Service         string
	ServiceIconPath string
	Title           string
	TitleHref       string
	Text            template.HTML
	ImagePath       string
}

// ReactionView is one reaction pill.
type ReactionView struct {
	Emoji template.HTML
	Count int
}

// Safe marks markup assembled by this program's own renderer as safe HTML.
// Never pass raw Slack text here (decision log 0026).
func Safe(s string) template.HTML { return template.HTML(s) }

var page = template.Must(template.ParseFS(templateFS, "templates/index.html.tmpl"))

// WriteHTML renders index.html into w.
func WriteHTML(w io.Writer, data *PageData) error {
	return page.Execute(w, data)
}

// WriteStyleCSS writes the bundled stylesheet next to index.html.
func WriteStyleCSS(outDir string) error {
	return os.WriteFile(filepath.Join(outDir, "style.css"), styleCSS, 0o644)
}
