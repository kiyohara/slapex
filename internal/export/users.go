// Users stage: the users and bots the fetched messages show, resolved through
// users.info / bots.info or taken from the --reuse-cache cache
// (doc/design/slack-api-usage.md, decision log 0054).

package export

import (
	"context"
	"fmt"

	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// resolvedUsers is the result of the Users stage: users by user ID and bots by
// bot ID. An ID whose lookup failed is absent, and the page shows the raw ID
// and an initial for it.
type resolvedUsers struct {
	users map[string]*slack.User
	bots  map[string]*slack.Bot
}

// resolveUsers runs the Users phase for the users and bots the fetched
// messages need (collectUserIDs / collectBotIDs).
func resolveUsers(ctx context.Context, client *slack.Client, fetched fetchedMessages, reuse *reusableCache, p *ui.Printer) resolvedUsers {
	userIDs := collectUserIDs(fetched.timeline, fetched.replies)
	botIDs := collectBotIDs(fetched.timeline, fetched.replies)
	p.StartPhase("Users", fmt.Sprintf("resolving %s ...", resolveTargetsLabel(len(userIDs), len(botIDs))))
	users, reusedUsers := lookupUsers(ctx, client, userIDs, reuse, p)
	bots, reusedBots := lookupBots(ctx, client, botIDs, reuse, p)
	p.EndPhase(ui.StatusSuccess, "Users", resolvedTargetsLabel(len(users), len(bots)),
		reusedTargetsMeta(reusedUsers, reusedBots))
	return resolvedUsers{users: users, bots: bots}
}

// lookupUsers resolves each user ID through users.info, or from the reuse
// cache when it holds the user. It returns the users found and how many of
// them came from the cache. A failed lookup is warned about and skipped.
func lookupUsers(ctx context.Context, client *slack.Client, ids []string, reuse *reusableCache, p *ui.Printer) (map[string]*slack.User, int) {
	users := map[string]*slack.User{}
	reused := 0
	for _, id := range ids {
		if reuse != nil {
			if cu, ok := reuse.users[id]; ok {
				users[id] = cu.toUser(id)
				reused++
				continue
			}
		}
		u, err := client.UserInfo(ctx, id)
		if err != nil {
			p.Warnf("could not resolve user %s: %s", id, err)
			continue
		}
		users[id] = u
	}
	return users, reused
}

// lookupBots resolves each bot ID like lookupUsers, through bots.info. A bot
// message that carries only bot_id has no user to resolve; bots.info supplies
// the app name and icon instead (decision log 0054). A failure is warned about
// and skipped, like an unresolvable user, so the export still completes with
// the bot_id and the initial fallback.
func lookupBots(ctx context.Context, client *slack.Client, ids []string, reuse *reusableCache, p *ui.Printer) (map[string]*slack.Bot, int) {
	bots := map[string]*slack.Bot{}
	reused := 0
	for _, id := range ids {
		if reuse != nil {
			if cb, ok := reuse.bots[id]; ok {
				bots[id] = cb.toBot(id)
				reused++
				continue
			}
		}
		bot, err := client.BotInfo(ctx, id)
		if err != nil {
			p.Warnf("could not resolve bot %s: %s", id, err)
			continue
		}
		bots[id] = bot
	}
	return bots, reused
}

// resolveTargetsLabel / resolvedTargetsLabel / reusedTargetsMeta render the Users
// phase counts. Bots only appear once the channel actually has bot posts to
// resolve, so a channel without them keeps the original wording.
func resolveTargetsLabel(users, bots int) string {
	if bots == 0 {
		return fmt.Sprintf("%d users", users)
	}
	return fmt.Sprintf("%d users, %s", users, botCountLabel(bots))
}

func resolvedTargetsLabel(users, bots int) string {
	if bots == 0 {
		return fmt.Sprintf("%d resolved", users)
	}
	return fmt.Sprintf("%d users, %s resolved", users, botCountLabel(bots))
}

func reusedTargetsMeta(users, bots int) string {
	switch {
	case users > 0 && bots > 0:
		return fmt.Sprintf("%d from cache, users.info / bots.info skipped", users+bots)
	case users > 0:
		return fmt.Sprintf("%d from cache, users.info skipped", users)
	case bots > 0:
		return fmt.Sprintf("%d from cache, bots.info skipped", bots)
	}
	return ""
}

func botCountLabel(n int) string {
	if n == 1 {
		return "1 bot"
	}
	return fmt.Sprintf("%d bots", n)
}
