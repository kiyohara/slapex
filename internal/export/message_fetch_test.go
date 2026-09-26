package export

import (
	"context"
	"slices"
	"testing"

	"github.com/kiyohara/slapex/internal/slack"
)

// TestFetchThreadHoldsExcludedRepliesByTS: fetchThread splits a thread's
// replies by the emoji filters as it fetches them and holds an excluded reply
// by its ts alone. The thread is held until the pages are done, when the
// Messages stage knows whether its parent is on the timeline, so it keeps no
// more than the replies the export may show. Nothing is counted as excluded
// yet: the thread may turn out to be fetched only to judge the parent of a
// broadcast (Issue #206).
func TestFetchThreadHoldsExcludedRepliesByTS(t *testing.T) {
	t.Parallel()

	const threadTS = "1700000001.000000"
	parent := slack.Message{Type: "message", TS: threadTS, ThreadTS: threadTS, User: "U01", Text: "parent", ReplyCount: 3}
	sc := baseScenario()
	sc.Replies[threadTS] = []slack.Message{
		parent,
		{Type: "message", TS: "1700000001.100000", ThreadTS: threadTS, User: "U02", Text: "kept reply"},
		{Type: "message", TS: "1700000001.200000", ThreadTS: threadTS, User: "U02", Text: "excluded reply :shushing_face:"},
		{Type: "message", TS: "1700000001.300000", ThreadTS: threadTS, User: "U01", Text: "another kept reply"},
	}
	fake := newFakeSlackServer(t, &sc)
	t.Cleanup(fake.Close)
	client := slack.New(integrationTestToken, slack.WithBaseURL(fake.URL()+"/api/"))
	filter := newMessageFilter([]string{"shushing_face"}, nil)

	thread, included, err := fetchThread(context.Background(), client, "C123", threadTS, filter)
	if err != nil {
		t.Fatalf("fetchThread() error = %v", err)
	}
	if !included {
		t.Fatal("fetchThread excluded a thread whose parent does not match the filters")
	}
	if got, want := messageTSs(thread.kept), []string{"1700000001.100000", "1700000001.300000"}; !slices.Equal(got, want) {
		t.Fatalf("kept = %v, want %v", got, want)
	}
	if want := []string{"1700000001.200000"}; !slices.Equal(thread.excludedTS, want) {
		t.Fatalf("excludedTS = %v, want %v", thread.excludedTS, want)
	}
	if got := filter.ExcludedCount(); got != 0 {
		t.Fatalf("ExcludedCount = %d, want 0", got)
	}
}

// TestTimelineRepliesKeepsOnlyTimelineThreads: once the pages are done, only
// the threads whose parent is on the timeline keep their replies and have the
// replies the filters excluded counted; a thread held only to judge the parent
// of a broadcast leaves neither (Issue #206). A thread excluded after it was
// fetched is forgotten on the page that drops its broadcast, so it is not held
// until the pages are done.
func TestTimelineRepliesKeepsOnlyTimelineThreads(t *testing.T) {
	t.Parallel()

	const (
		shownTS  = "1700000001.000000" // parent on the timeline
		judgedTS = "1700000002.000000" // parent off the timeline, fetched through its broadcast
		lateTS   = "1700000003.000000" // excluded after its broadcast fetched it
	)
	reply := func(ts, threadTS string) slack.Message {
		return slack.Message{Type: "message", TS: ts, ThreadTS: threadTS, User: "U02", Text: "reply"}
	}
	broadcast := func(ts, threadTS string) slack.Message {
		m := reply(ts, threadTS)
		m.Subtype = "thread_broadcast"
		return m
	}
	timeline := []slack.Message{
		{Type: "message", TS: shownTS, ThreadTS: shownTS, User: "U01", Text: "parent", ReplyCount: 3},
		broadcast("1700000005.000000", judgedTS),
		broadcast("1700000006.000000", lateTS),
	}
	threads := map[string]fetchedThread{
		shownTS: {
			kept:       []slack.Message{reply("1700000001.200000", shownTS), reply("1700000001.100000", shownTS)},
			excludedTS: []string{"1700000001.300000"},
		},
		judgedTS: {kept: []slack.Message{reply("1700000002.100000", judgedTS)}, excludedTS: []string{"1700000002.200000"}},
		lateTS:   {kept: []slack.Message{reply("1700000003.100000", lateTS)}, excludedTS: []string{"1700000003.200000"}},
	}
	filter := newMessageFilter([]string{"shushing_face"}, nil)
	filter.ExcludeThread(lateTS) // its parent's history copy matched on a later page

	timeline = dropExcludedThreads(timeline, threads, filter)
	if got, want := messageTSs(timeline), []string{shownTS, "1700000005.000000"}; !slices.Equal(got, want) {
		t.Fatalf("timeline = %v, want %v", got, want)
	}
	if _, ok := threads[lateTS]; ok {
		t.Fatal("dropExcludedThreads kept holding the replies of the excluded thread")
	}

	replies, repliesTruncated := timelineReplies(timeline, threads, filter)
	if len(replies) != 1 || len(repliesTruncated) != 1 {
		t.Fatalf("timelineReplies returned threads %v / %v, want only %s", replies, repliesTruncated, shownTS)
	}
	if got, want := messageTSs(replies[shownTS]), []string{"1700000001.100000", "1700000001.200000"}; !slices.Equal(got, want) {
		t.Fatalf("replies[%s] = %v, want %v", shownTS, got, want)
	}
	// The broadcast of the excluded thread and the excluded reply of the
	// shown thread.
	if got := filter.ExcludedCount(); got != 2 {
		t.Fatalf("ExcludedCount = %d, want 2", got)
	}
}

func messageTSs(messages []slack.Message) []string {
	var ts []string
	for i := range messages {
		ts = append(ts, messages[i].TS)
	}
	return ts
}
