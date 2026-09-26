// Messages stage: conversations.history paged back from the end of the fetch
// range until --max-posts messages survive the emoji filters, with the replies
// of each thread fetched as its parent, or with a filter one of its
// broadcasts, arrives (doc/design/slack-api-usage.md).

package export

import (
	"context"
	"fmt"
	"sort"

	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

const maxThreadReplies = 1000 // per-thread reply cap (doc/design/output-format.md)

// fetchedMessages is the result of the Messages stage.
type fetchedMessages struct {
	// timeline holds the kept channel messages in ascending ts order.
	timeline []slack.Message
	// replies holds the kept replies of each fetched thread in ascending ts
	// order, keyed by thread_ts. A thread whose replies were all excluded is
	// absent. A thread fetched through its broadcast stays even when its
	// parent is not on the timeline (Issue #206).
	replies map[string][]slack.Message
	// repliesTruncated marks the threads in replies whose replies stopped at
	// maxThreadReplies.
	repliesTruncated map[string]bool
	// truncated reports that --max-posts cut the timeline while older messages
	// remained in the range.
	truncated bool
	// excluded counts the distinct messages the emoji filters excluded.
	excluded int
}

// fetchMessages runs the Messages phase. Each history page asks for the
// messages --max-posts still lacks and brings in the replies of the threads new
// on that page. A thread whose parent turns out to be excluded takes its
// timeline messages with it, and the next page refills them.
func fetchMessages(ctx context.Context, client *slack.Client, channelID string, fetchRange messageFetchRange, opts Options, p *ui.Printer) (fetchedMessages, error) {
	filter := newMessageFilter(opts.ExcludeBodyEmoji, opts.ExcludeReactionEmoji)
	p.StartPhase("Messages", fmt.Sprintf("fetching %s (--max-posts %d) ...", fetchRange.progressLabel(), opts.MaxPosts))
	var timeline []slack.Message
	replies := map[string][]slack.Message{}
	repliesTruncated := map[string]bool{}
	fetched := fetchedThreads{}
	latest := fetchRange.latestTS()
	truncated := false
	for len(timeline) < opts.MaxPosts {
		batch, more, err := client.History(ctx, channelID, fetchRange.oldestTS(), latest, opts.MaxPosts-len(timeline), filter.Include,
			func(n int) {
				p.UpdatePhase(fmt.Sprintf("fetching %s ... %d fetched", fetchRange.progressLabel(), len(timeline)+n))
			})
		if err != nil {
			return fetchedMessages{}, err
		}
		if len(batch) == 0 {
			break
		}
		latest = oldestMessageTS(batch)
		timeline = append(timeline, batch...)

		threadIDs := unfetchedThreadIDs(batch, fetched, filter.Enabled())
		threadTotal := len(fetched) + len(threadIDs)
		for _, threadTS := range threadIDs {
			fetched[threadTS] = struct{}{}
			p.UpdatePhase(fmt.Sprintf("fetching thread replies ... %d/%d", len(fetched), threadTotal))
			kept, keptTruncated, err := fetchThreadReplies(ctx, client, channelID, threadTS, filter)
			if err != nil {
				return fetchedMessages{}, err
			}
			if len(kept) > 0 {
				replies[threadTS] = kept
				repliesTruncated[threadTS] = keptTruncated
			}
		}
		timeline = dropExcludedThreads(timeline, filter, replies, repliesTruncated)

		if len(timeline) >= opts.MaxPosts {
			truncated = more
			break
		}
		if !more {
			break
		}
	}
	sort.Slice(timeline, func(i, j int) bool { return tsLess(timeline[i].TS, timeline[j].TS) })
	result := fetchedMessages{
		timeline:         timeline,
		replies:          replies,
		repliesTruncated: repliesTruncated,
		truncated:        truncated,
		excluded:         filter.ExcludedCount(),
	}
	status, meta := messagesPhaseMeta(result, opts)
	p.EndPhase(status, "Messages", fmt.Sprintf("%d fetched %s", len(timeline), fetchRange.progressLabel()), meta)
	return result, nil
}

// fetchThreadReplies fetches one thread through conversations.replies. It
// returns the replies that pass the filter, in ascending ts order, and whether
// the replies stopped at maxThreadReplies; it returns none when the thread is
// excluded (messageFilter.IncludeThread).
func fetchThreadReplies(ctx context.Context, client *slack.Client, channelID, threadTS string, filter *messageFilter) ([]slack.Message, bool, error) {
	parent, replies, truncated, err := client.Thread(ctx, channelID, threadTS, maxThreadReplies)
	if err != nil {
		return nil, false, err
	}
	if !filter.IncludeThread(threadTS, parent) {
		return nil, false, nil
	}
	var kept []slack.Message
	for i := range replies {
		if filter.Include(&replies[i]) {
			kept = append(kept, replies[i])
		}
	}
	sort.Slice(kept, func(i, j int) bool { return tsLess(kept[i].TS, kept[j].TS) })
	return kept, truncated, nil
}

// dropExcludedThreads removes from timeline the messages of excluded threads,
// counting each as excluded: broadcasts, and a parent whose thread was
// excluded through its conversations.replies copy. It also forgets the replies
// kept for those threads, since a thread can be excluded after its replies
// were kept: its broadcast fetched it on an earlier page, and its parent's
// history copy arrives on this page carrying an excluded reaction added in
// between.
func dropExcludedThreads(timeline []slack.Message, filter *messageFilter, replies map[string][]slack.Message, repliesTruncated map[string]bool) []slack.Message {
	kept := timeline[:0]
	for i := range timeline {
		threadTS := messageThreadTS(&timeline[i])
		if !filter.ThreadExcluded(threadTS) {
			kept = append(kept, timeline[i])
			continue
		}
		filter.Exclude(&timeline[i])
		delete(replies, threadTS)
		delete(repliesTruncated, threadTS)
	}
	return kept
}

// messagesPhaseMeta returns the status and meta of the line that ends the
// Messages phase: the thread and reply totals, the excluded count once a
// filter has excluded something, and the --max-posts cut, which turns the line
// into a warning.
func messagesPhaseMeta(fetched fetchedMessages, opts Options) (ui.Status, string) {
	status := ui.StatusSuccess
	meta := fmt.Sprintf("threads %d, replies %d", len(fetched.replies), countReplies(fetched.replies))
	if label := excludedMessagesLabel(opts); fetched.excluded > 0 && label != "" {
		meta += fmt.Sprintf(", %s: %d", label, fetched.excluded)
	}
	if fetched.truncated {
		status = ui.StatusWarn
		meta += fmt.Sprintf(", truncated by --max-posts %d", opts.MaxPosts)
	}
	return status, meta
}
