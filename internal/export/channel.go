// Channel selection: keyword matching against the channel list, the
// non-interactive candidate listing and the huh selection UI
// (doc/design/usage-flow.md).
package export

import (
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"

	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

const maxSelectable = 10 // interactive selection limit (decision log 0004)

func chooseChannel(channels []slack.Channel, opts Options, wsLine string, p *ui.Printer) (slack.Channel, error) {
	keyword := opts.ChannelKeyword
	var candidates []slack.Channel
	if keyword == "" {
		candidates = channels
	} else {
		for _, ch := range channels {
			if ch.ID == keyword || ch.Name == strings.TrimPrefix(keyword, "#") {
				return ch, nil
			}
		}
		lower := strings.ToLower(strings.TrimPrefix(keyword, "#"))
		for _, ch := range channels {
			if strings.Contains(strings.ToLower(ch.Name), lower) {
				candidates = append(candidates, ch)
			}
		}
	}

	switch {
	case len(candidates) == 0:
		return slack.Channel{}, usagef("no channel matched %q. Check the channel name or ID; for private channels the bot must be a member.", keyword)
	case len(candidates) == 1 && keyword != "":
		return candidates[0], nil
	}

	if len(candidates) > maxSelectable {
		p.StopPhase()
		p.Warnf("%d channels matched. Run again with a more specific channel name or a channel ID.", len(candidates))
		return slack.Channel{}, usagef("too many candidates (%d). Re-run as: slapex <channel-id-or-name>", len(candidates))
	}

	if opts.PromptTTY == nil || opts.NoInteractive {
		p.StopPhase()
		if keyword == "" {
			p.Warnf("No channel specified. Select one of the following channels:")
		} else {
			p.Warnf("Multiple channels matched %q.", keyword)
		}
		p.Plainf("")
		p.Plainf("Workspace: %s", wsLine)
		p.Plainf("")
		p.Plainf("Candidates:")
		for _, ch := range candidates {
			p.Plainf("  %s", channelLine(ch))
		}
		p.Plainf("")
		p.Plainf("Run again with a more specific channel:")
		p.Plainf("")
		p.Plainf("  slapex %s", candidates[0].ID)
		return slack.Channel{}, usagef("channel selection required but interactive selection is unavailable")
	}

	// Stop the live phase line before huh draws its selection UI on the
	// controlling terminal; a running spinner on stderr would fight it.
	p.StopPhase()
	return selectChannel(candidates, opts.PromptTTY)
}

func selectChannel(candidates []slack.Channel, tty *os.File) (slack.Channel, error) {
	choices := make([]huh.Option[int], len(candidates))
	for i, ch := range candidates {
		choices[i] = huh.NewOption(channelLine(ch), i)
	}
	idx := 0
	// Drive the form entirely over the controlling terminal so selection works
	// even when stdout/stderr are redirected or wrapped (e.g. `op run` masking).
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[int]().
			Title("Select a channel").
			Options(choices...).
			Value(&idx),
	)).WithInput(tty).WithOutput(tty)
	if err := form.Run(); err != nil {
		return slack.Channel{}, usagef("channel selection cancelled")
	}
	return candidates[idx], nil
}

func channelLine(ch slack.Channel) string {
	return fmt.Sprintf("#%s (%s)", ch.Name, channelMeta(ch))
}

// channelMeta is the parenthesized channel detail shared by channelLine and
// the Channel phase line (usage-flow.md「処理対象の表示」).
func channelMeta(ch slack.Channel) string {
	visibility := "public"
	if ch.IsPrivate {
		visibility = "private"
	}
	state := "active"
	if ch.IsArchived {
		state = "archived"
	}
	membership := "not member"
	if ch.IsMember {
		membership = "member"
	}
	return fmt.Sprintf("%s, %s, %s, %s", ch.ID, visibility, state, membership)
}

func channelURL(workspaceURL, channelID string) string {
	if workspaceURL == "" || channelID == "" {
		return ""
	}
	return strings.TrimRight(workspaceURL, "/") + "/archives/" + channelID
}
