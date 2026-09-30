// Prefetch of the asset downloads (Issue #279, PF-07 of #272; decision log
// 0069, doc/design/slack-api-usage.md「取得の並行化」). While the driver is at
// the Messages, Users and Emoji stages, the assets the page is certain to show
// go ahead as soon as they are certain: the workspace icon at the start of
// the Messages phase, the assets of each message the prefetcher confirms
// (prefetchHistoryPage, prefetchThread, and at the end of the Messages phase
// the rest the page shows), and the avatar of each user and bot once
// users.info or bots.info — or the reuse cache — has it. They are planned by
// the render the Assets phase plans with (renderWithAssets), over a planner,
// and sent ahead through output.Assets.Prefetch, which keeps what each
// download gets for the Assets phase's Fetch to take, in plan order: the
// page, the manifest and stderr come out as without the prefetch.

package export

import (
	"sync"

	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// assetPrefetcher plans the assets the page is certain to show and sends
// their downloads ahead. It may be called from the driver and from the
// prefetched requests at once. A nil assetPrefetcher sends nothing.
type assetPrefetcher struct {
	assets             *output.Assets
	maxAttachmentBytes int64

	mu      sync.Mutex
	planner *output.Assets
	sent    int // how much of planner's plan has gone ahead
	views   *messageViewBuilder
	// custom is set once views shows the workspace's custom emoji. Until
	// then, it shows a custom emoji by its name and asks for no image for
	// it, so the messages it plans are kept in early, to plan again with the
	// custom emoji once they are known.
	custom bool
	early  []slack.Message
}

// newAssetPrefetcher returns the assetPrefetcher that sends ahead through
// assets, where the size limit of files is maxAttachmentBytes. With known,
// customEmoji are the workspace's custom emoji (from the reuse cache);
// without, emoji.list has yet to return them (customEmoji). It returns nil
// when it cannot plan: the view builder's emoji table does not load, which
// the driver reports when it gets there.
func newAssetPrefetcher(assets *output.Assets, maxAttachmentBytes int64, customEmoji map[string]string, known bool) *assetPrefetcher {
	resolver, err := emoji.NewResolver(customEmoji)
	if err != nil {
		return nil
	}
	p := &assetPrefetcher{assets: assets, maxAttachmentBytes: maxAttachmentBytes, planner: assets.Planner(), custom: known}
	p.views = p.newViews(resolver)
	return p
}

// newViews returns the view builder that plans the assets of messages with
// resolver: that of renderTimeline, with no user or bot resolved. What a
// message asks for does not depend on them — a user's avatar is asked for
// with the users (avatar) — only on the custom emoji.
func (p *assetPrefetcher) newViews(resolver *emoji.Resolver) *messageViewBuilder {
	return newMessageViewBuilder(p.planner, resolvedUsers{}, resolver, p.maxAttachmentBytes)
}

// workspaceIcon sends the workspace icon of the page header ahead.
func (p *assetPrefetcher) workspaceIcon(teamInfo *slack.TeamInfo) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	saveWorkspaceIcon(p.planner, teamInfo)
	p.sendLocked()
}

// avatar sends ahead the avatar of a resolved user or bot, whose URL is
// srcURL (avatarURL, slack.BotIcons.URL): the page saves one for each
// (newMessageViewBuilder).
func (p *assetPrefetcher) avatar(srcURL string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.planner.Save(output.KindAvatar, srcURL, output.AssetMeta{})
	p.sendLocked()
}

// messages sends ahead the assets that messages, which the page is certain
// to show, ask for: their files, images, URL previews, app icons and custom
// emoji, the last once the custom emoji are known (customEmoji).
func (p *assetPrefetcher) messages(messages []slack.Message) {
	if p == nil || len(messages) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range messages {
		p.views.messageView(&messages[i])
	}
	if !p.custom {
		p.early = append(p.early, messages...)
	}
	p.sendLocked()
}

// customEmoji takes the workspace's custom emoji from emoji.list, and sends
// ahead the images that the messages planned before ask for.
func (p *assetPrefetcher) customEmoji(customEmoji map[string]string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.custom {
		return
	}
	resolver, err := emoji.NewResolver(customEmoji)
	if err != nil {
		return
	}
	p.views, p.custom = p.newViews(resolver), true
	for i := range p.early {
		p.views.messageView(&p.early[i])
	}
	p.early = nil
	p.sendLocked()
}

// sendLocked sends ahead what the planner has planned since the last call.
func (p *assetPrefetcher) sendLocked() {
	plan := p.planner.Plan()
	p.assets.Prefetch(plan[p.sent:])
	p.sent = len(plan)
}

// shownMessages returns the messages the page shows, in the order the page
// renders them: each timeline message, and the replies of its thread after
// it (buildTimeline).
func shownMessages(fetched fetchedMessages) []slack.Message {
	shown := make([]slack.Message, 0, len(fetched.timeline))
	for _, m := range fetched.timeline {
		shown = append(shown, m)
		shown = append(shown, fetched.replies[m.TS]...)
	}
	return shown
}
