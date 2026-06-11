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

	// 1. code blocks and inline code render as plain text: Slack embeds
	//    constructs even inside code (auto-linked URLs, mentions), so they
	//    are reduced to display text and leftover angle brackets escaped.
	s := reFenced.ReplaceAllStringFunc(text, func(m string) string {
		inner := reFenced.FindStringSubmatch(m)[1]
		return c.stash("<pre><code>" + c.codeText(strings.TrimRight(inner, "\n")) + "</code></pre>")
	})
	s = reInline.ReplaceAllStringFunc(s, func(m string) string {
		inner := reInline.FindStringSubmatch(m)[1]
		return c.stash("<code>" + c.codeText(inner) + "</code>")
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

// codeText renders code span / code block content. Slack auto-links URLs
// (and can embed mentions) even inside code, so constructs are reduced to
// their display text; remaining raw angle brackets are escaped so browsers
// never parse code content as markup. User-typed < and > arrive entity-
// escaped from Slack, so this never double-escapes.
func (c *converter) codeText(s string) string {
	s = reConstruct.ReplaceAllStringFunc(s, func(m string) string {
		body, label, hasLabel := strings.Cut(reConstruct.FindStringSubmatch(m)[1], "|")
		text, _ := c.constructText(body, label, hasLabel)
		return text
	})
	s = strings.ReplaceAll(s, "<", "&lt;")
	return strings.ReplaceAll(s, ">", "&gt;")
}

func (c *converter) construct(inner string) string {
	body, label, hasLabel := strings.Cut(inner, "|")
	text, mention := c.constructText(body, label, hasLabel)
	switch {
	case mention:
		return `<span class="mention">` + text + `</span>`
	case strings.HasPrefix(body, "!"):
		return text
	default:
		return anchor(body, text)
	}
}

// constructText resolves the display text of a <...> construct already split
// at "|"; mention reports whether it is highlighted outside code content.
func (c *converter) constructText(body, label string, hasLabel bool) (text string, mention bool) {
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
		return "@" + name, true
	case strings.HasPrefix(body, "#"):
		name := label
		if name == "" {
			name = strings.TrimPrefix(body, "#")
		}
		return "#" + name, true
	case strings.HasPrefix(body, "!"):
		return c.special(strings.TrimPrefix(body, "!"), label)
	default:
		if label != "" {
			return label, false
		}
		return body, false
	}
}

func (c *converter) special(cmd, label string) (text string, mention bool) {
	switch {
	case cmd == "here" || cmd == "channel" || cmd == "everyone":
		return "@" + cmd, true
	case strings.HasPrefix(cmd, "subteam^"):
		if label == "" {
			label = "@group"
		}
		return label, true
	case strings.HasPrefix(cmd, "date^"):
		// render the fallback text per the spec; full <!date> formatting
		// support is out of scope initially.
		if label != "" {
			return label, false
		}
		return cmd, false
	default:
		if label != "" {
			return label, false
		}
		return cmd, false
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
