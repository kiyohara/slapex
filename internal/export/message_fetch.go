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
	// replies holds the kept replies of the threads whose parent is on the
	// timeline, in ascending ts order, keyed by thread_ts. A thread whose
	// replies were all excluded is absent, and so is a thread fetched only to
	// judge the parent of a broadcast, when that parent is older than the fetch
	// range or fell beyond --max-posts (Issue #206).
	replies map[string][]slack.Message
	// repliesTruncated marks the threads in replies whose replies stopped at
	// maxThreadReplies.
	repliesTruncated map[string]bool
	// truncated reports that --max-posts cut the timeline while older messages
	// remained in the range.
	truncated bool
	// excluded counts the distinct messages the emoji filters excluded: those
	// conversations.history judged, including the ones it examined past the
	// --max-posts cut (slack.Client.History), the timeline messages of excluded
	// threads, and the replies of the threads whose parent is on the timeline.
	excluded int
}

// counts returns the message counts the Messages line, metadata.json and the
// Done summary report. Since replies only holds the threads whose parent is on
// the timeline, its threads and replies are the ones the page shows.
func (f fetchedMessages) counts() exportCounts {
	return exportCounts{
		timeline: len(f.timeline),
		threads:  len(f.replies),
		replies:  countReplies(f.replies),
		excluded: f.excluded,
	}
}

// fetchedThread is a thread's conversations.replies result that the Messages
// stage holds until the pages are done and it knows whether the thread's
// parent is on the timeline: the replies the emoji filters keep, the ts of the
// ones they exclude, and whether the replies stopped at maxThreadReplies. An
// excluded reply is held by its ts alone, since it is only ever counted.
type fetchedThread struct {
	kept       []slack.Message
	excludedTS []string
	truncated  bool
}

// fetchMessages runs the Messages phase. Each history page asks for the
// messages --max-posts still lacks and fetches the threads new on that page. A
// thread whose parent turns out to be excluded takes its timeline messages with
// it, and the next page refills them. Once the pages are done, the threads
// whose parent is on the timeline keep their replies.
func fetchMessages(ctx context.Context, client *slack.Client, channelID string, fetchRange messageFetchRange, opts Options, p *ui.Printer) (fetchedMessages, error) {
	filter := newMessageFilter(opts.ExcludeBodyEmoji, opts.ExcludeReactionEmoji)
	p.StartPhase("Messages", fmt.Sprintf("fetching %s (--max-posts %d) ...", fetchRange.progressLabel(), opts.MaxPosts))
	var timeline []slack.Message
	fetched := fetchedThreads{}
	threads := map[string]fetchedThread{}
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

		threadIDs := unfetchedThreadIDs(batch, fetched, filter)
		threadTotal := len(fetched) + len(threadIDs)
		for _, threadTS := range threadIDs {
			fetched[threadTS] = struct{}{}
			p.UpdatePhase(fmt.Sprintf("fetching thread replies ... %d/%d", len(fetched), threadTotal))
			thread, included, err := fetchThread(ctx, client, channelID, threadTS, filter)
			if err != nil {
				return fetchedMessages{}, err
			}
			if included {
				threads[threadTS] = thread
			}
		}
		timeline = dropExcludedThreads(timeline, threads, filter)

		if len(timeline) >= opts.MaxPosts {
			truncated = more
			break
		}
		if !more {
			break
		}
	}
	sort.Slice(timeline, func(i, j int) bool { return tsLess(timeline[i].TS, timeline[j].TS) })
	replies, repliesTruncated := timelineReplies(timeline, threads, filter)
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

// fetchThread fetches one thread through conversations.replies and reports
// whether it stays in the export (messageFilter.IncludeThread). It splits the
// replies by the emoji filters right away, without counting the excluded ones:
// timelineReplies counts them once it knows the thread's parent is on the
// timeline.
func fetchThread(ctx context.Context, client *slack.Client, channelID, threadTS string, filter *messageFilter) (fetchedThread, bool, error) {
	parent, replies, truncated, err := client.Thread(ctx, channelID, threadTS, maxThreadReplies)
	if err != nil {
		return fetchedThread{}, false, err
	}
	if !filter.IncludeThread(threadTS, parent) {
		return fetchedThread{}, false, nil
	}
	thread := fetchedThread{truncated: truncated}
	for i := range replies {
		if filter.matches(&replies[i]) {
			thread.excludedTS = append(thread.excludedTS, replies[i].TS)
			continue
		}
		thread.kept = append(thread.kept, replies[i])
	}
	return thread, true, nil
}

// dropExcludedThreads removes from timeline the messages of excluded threads,
// counting each as excluded: broadcasts, and a parent whose thread was
// excluded through its conversations.replies copy. It also forgets the replies
// held for those threads, since a thread can be excluded after it was fetched:
// its broadcast fetched it on an earlier page, and its parent's history copy
// arrives on this page carrying an excluded reaction added in between.
func dropExcludedThreads(timeline []slack.Message, threads map[string]fetchedThread, filter *messageFilter) []slack.Message {
	kept := timeline[:0]
	for i := range timeline {
		threadTS := messageThreadTS(&timeline[i])
		if !filter.ThreadExcluded(threadTS) {
			kept = append(kept, timeline[i])
			continue
		}
		filter.Exclude(&timeline[i])
		delete(threads, threadTS)
	}
	return kept
}

// timelineReplies returns the kept replies of the threads whose parent is on
// the timeline, in ascending ts order, and which of those threads stopped at
// maxThreadReplies, and it counts the replies the emoji filters excluded from
// those threads. An excluded thread is left out, and so is a thread whose
// replies were all excluded. So is a thread fetched only to judge the parent
// of a broadcast, when that parent is older than the fetch range or fell
// beyond --max-posts: its replies are neither shown, counted nor resolved, and
// the ones the filters exclude are not counted as excluded (Issue #206).
func timelineReplies(timeline []slack.Message, threads map[string]fetchedThread, filter *messageFilter) (map[string][]slack.Message, map[string]bool) {
	replies := map[string][]slack.Message{}
	repliesTruncated := map[string]bool{}
	for i := range timeline {
		threadTS := timeline[i].TS
		thread, ok := threads[threadTS]
		if !ok || filter.ThreadExcluded(threadTS) {
			continue
		}
		for _, ts := range thread.excludedTS {
			filter.ExcludeReply(ts)
		}
		kept := thread.kept
		if len(kept) == 0 {
			continue
		}
		sort.Slice(kept, func(a, b int) bool { return tsLess(kept[a].TS, kept[b].TS) })
		replies[threadTS] = kept
		repliesTruncated[threadTS] = thread.truncated
	}
	return replies, repliesTruncated
}

// messagesPhaseMeta returns the status and meta of the line that ends the
// Messages phase: the thread and reply totals, the excluded count once a
// filter has excluded something, and the --max-posts cut, which turns the line
// into a warning.
func messagesPhaseMeta(fetched fetchedMessages, opts Options) (ui.Status, string) {
	status := ui.StatusSuccess
	counts := fetched.counts()
	meta := fmt.Sprintf("threads %d, replies %d", counts.threads, counts.replies)
	if label := excludedMessagesLabel(opts); counts.excluded > 0 && label != "" {
		meta += fmt.Sprintf(", %s: %d", label, counts.excluded)
	}
	if fetched.truncated {
		status = ui.StatusWarn
		meta += fmt.Sprintf(", truncated by --max-posts %d", opts.MaxPosts)
	}
	return status, meta
}
