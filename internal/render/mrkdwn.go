package render

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// TextResolver supplies name and emoji resolution to the mrkdwn converter.
type TextResolver interface {
	// UserName returns the display name for a user ID (or the ID itself).
	UserName(id string) string
	// EmojiHTML returns a safe HTML snippet for a shortcode without colons.
	EmojiHTML(name string) string
}

// Slack API text pre-escapes &, < and > as HTML entities, so a literal "<"
// only ever introduces a mrkdwn construct (doc/design/html-rendering.md).
// The converter extracts constructs and code spans into placeholders first,
// applies formatting to the remaining plain text, then restores placeholders.
// All emitted markup is generated here; Slack text is never passed through
// unescaped (decision log 0026).

var (
	reFenced    = regexp.MustCompile("(?s)```\n?(.*?)```")
	reInline    = regexp.MustCompile("`([^`\n]+)`")
	reConstruct = regexp.MustCompile(`<([^<>]+)>`)
	reEmoji     = regexp.MustCompile(`:([a-z0-9_+'-]+(?:::skin-tone-[2-6])?):`)
	reBold      = regexp.MustCompile(`\*([^*\n]+)\*`)
	reItalic    = regexp.MustCompile(`(^|[^\w])_([^_\n]+)_($|[^\w])`)
	reStrike    = regexp.MustCompile(`~([^~\n]+)~`)
	reUserID    = regexp.MustCompile(`^[UW][A-Z0-9]+$`)
)

type converter struct {
	res          TextResolver
	placeholders []string
}

func (c *converter) stash(html string) string {
	c.placeholders = append(c.placeholders, html)
	return fmt.Sprintf("\x00%d\x00", len(c.placeholders)-1)
}

func (c *converter) restore(s string) string {
	for i, html := range c.placeholders {
		s = strings.Replace(s, fmt.Sprintf("\x00%d\x00", i), html, 1)
	}
	return s
}

// Mrkdwn converts Slack mrkdwn text into safe HTML.
func Mrkdwn(text string, res TextResolver) template.HTML {
	c := &converter{res: res}

	// 1. code blocks and inline code keep their content verbatim
	//    (already entity-escaped by Slack).
	s := reFenced.ReplaceAllStringFunc(text, func(m string) string {
		inner := reFenced.FindStringSubmatch(m)[1]
		return c.stash("<pre><code>" + strings.TrimRight(inner, "\n") + "</code></pre>")
	})
	s = reInline.ReplaceAllStringFunc(s, func(m string) string {
		inner := reInline.FindStringSubmatch(m)[1]
		return c.stash("<code>" + inner + "</code>")
	})

	// 2. <...> constructs: mentions, channel links, special commands, URLs.
	s = reConstruct.ReplaceAllStringFunc(s, func(m string) string {
		inner := reConstruct.FindStringSubmatch(m)[1]
		return c.stash(c.construct(inner))
	})

	// 3. emoji shortcodes.
	s = reEmoji.ReplaceAllStringFunc(s, func(m string) string {
		name := reEmoji.FindStringSubmatch(m)[1]
		return c.stash(c.res.EmojiHTML(name))
	})

	// 4. formatting on the remaining plain text.
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reItalic.ReplaceAllString(s, "$1<em>$2</em>$3")
	s = reStrike.ReplaceAllString(s, "<s>$1</s>")

	// 5. blockquotes (lines starting with "&gt;") and line breaks.
	s = linesToHTML(s)

	return template.HTML(c.restore(s))
}

func (c *converter) construct(inner string) string {
	body, label, hasLabel := strings.Cut(inner, "|")
	switch {
	case strings.HasPrefix(body, "@"):
		id := strings.TrimPrefix(body, "@")
		name := label
		if !hasLabel && reUserID.MatchString(id) {
			name = c.res.UserName(id)
		}
		if name == "" {
			name = id
		}
		return `<span class="mention">@` + name + `</span>`
	case strings.HasPrefix(body, "#"):
		name := label
		if name == "" {
			name = strings.TrimPrefix(body, "#")
		}
		return `<span class="mention">#` + name + `</span>`
	case strings.HasPrefix(body, "!"):
		return c.special(strings.TrimPrefix(body, "!"), label)
	default:
		return anchor(body, label)
	}
}

func (c *converter) special(cmd, label string) string {
	switch {
	case cmd == "here" || cmd == "channel" || cmd == "everyone":
		return `<span class="mention">@` + cmd + `</span>`
	case strings.HasPrefix(cmd, "subteam^"):
		if label == "" {
			label = "@group"
		}
		return `<span class="mention">` + label + `</span>`
	case strings.HasPrefix(cmd, "date^"):
		// render the fallback text per the spec; full <!date> formatting
		// support is out of scope initially.
		if label != "" {
			return label
		}
		return cmd
	default:
		if label != "" {
			return label
		}
		return cmd
	}
}

// anchor renders a link construct; only http/https URLs become anchors.
func anchor(href, label string) string {
	display := label
	if display == "" {
		display = href
	}
	if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
		return display
	}
	return `<a href="` + attrEscape(href) + `">` + display + `</a>`
}

func attrEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// linesToHTML groups consecutive "&gt;"-quoted lines into blockquotes and
// joins everything else with <br>.
func linesToHTML(s string) string {
	lines := strings.Split(s, "\n")
	var out strings.Builder
	var quote []string
	flushQuote := func() {
		if len(quote) == 0 {
			return
		}
		out.WriteString("<blockquote>" + strings.Join(quote, "<br>") + "</blockquote>")
		quote = nil
	}
	for i, line := range lines {
		if q, ok := cutQuote(line); ok {
			quote = append(quote, q)
			continue
		}
		flushQuote()
		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteString("<br>")
		}
	}
	flushQuote()
	return out.String()
}

func cutQuote(line string) (string, bool) {
	if rest, ok := strings.CutPrefix(line, "&gt; "); ok {
		return rest, true
	}
	if rest, ok := strings.CutPrefix(line, "&gt;"); ok {
		return rest, true
	}
	return "", false
}
