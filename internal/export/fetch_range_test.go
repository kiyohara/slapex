package export

import (
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

func TestResolveFetchRangeDateUsesLocalCalendarDay(t *testing.T) {
	r, err := resolveFetchRange(Options{Date: "2026-07-03"}, time.Time{})
	if err != nil {
		t.Fatalf("resolveFetchRange: %v", err)
	}
	wantStart := time.Date(2026, 7, 3, 0, 0, 0, 0, time.Local)
	wantEnd := wantStart.AddDate(0, 0, 1)
	if r.mode != "date" || !r.start.Equal(wantStart) || !r.end.Equal(wantEnd) {
		t.Fatalf("range = %+v, want date [%s, %s)", r, wantStart, wantEnd)
	}
}

func TestResolveFetchRangeDaysUsesAbsoluteStartAndEnd(t *testing.T) {
	now := time.Date(2026, 7, 12, 13, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	r, err := resolveFetchRange(Options{Days: 30}, now)
	if err != nil {
		t.Fatalf("resolveFetchRange: %v", err)
	}
	if want := now.Add(-30 * 24 * time.Hour); !r.start.Equal(want) {
		t.Fatalf("start = %s, want %s", r.start, want)
	}
	if !r.end.Equal(now) {
		t.Fatalf("end = %s, want %s", r.end, now)
	}
}

func TestResolveDateTimeFetchRangePreservesAbsoluteInstants(t *testing.T) {
	local := time.FixedZone("JST", 9*60*60)
	r, err := resolveDateTimeFetchRange("2026/07/03 09:30", "2026-07-03T03:00:00Z", local)
	if err != nil {
		t.Fatalf("resolveDateTimeFetchRange: %v", err)
	}
	if r.mode != "datetime-range" {
		t.Fatalf("mode = %q, want datetime-range", r.mode)
	}
	if got, want := r.start.Format(time.RFC3339), "2026-07-03T09:30:00+09:00"; got != want {
		t.Fatalf("start = %s, want %s", got, want)
	}
	if got, want := r.end.UTC().Format(time.RFC3339), "2026-07-03T03:00:00Z"; got != want {
		t.Fatalf("end = %s, want %s", got, want)
	}
	if got, want := r.progressLabel(), "from 2026-07-03T00:30:00Z (included) to 2026-07-03T03:00:00Z (not included)"; got != want {
		t.Fatalf("progress label = %q, want %q", got, want)
	}
	if got, want := r.footerRangeLabel(), "From 2026-07-03T00:30:00Z (included); to 2026-07-03T03:00:00Z (not included); timezone: UTC"; got != want {
		t.Fatalf("footer range = %q, want %q", got, want)
	}
}

// TestResolveDateTimeFetchRangeKeepsFractionOfSecond covers Issue #210: a
// fraction of a second in --from / --to reaches the Slack boundaries, the
// progress and footer labels and metadata.json alike, instead of the
// boundaries alone dropping it.
func TestResolveDateTimeFetchRangeKeepsFractionOfSecond(t *testing.T) {
	local := time.FixedZone("JST", 9*60*60)
	tests := []struct {
		name         string
		from, to     string
		wantOldest   string
		wantLatest   string
		wantProgress string
		wantFooter   string
		wantStart    string // metadata.json fetch.target_range.start
		wantEnd      string // metadata.json fetch.target_range.end
	}{
		{
			name:         "RFC3339Nano inside one second",
			from:         "2026-07-03T09:30:00.2Z",
			to:           "2026-07-03T09:30:00.8Z",
			wantOldest:   "1783071000.200000",
			wantLatest:   "1783071000.800000",
			wantProgress: "from 2026-07-03T09:30:00.2Z (included) to 2026-07-03T09:30:00.8Z (not included)",
			wantFooter:   "From 2026-07-03T09:30:00.2Z (included); to 2026-07-03T09:30:00.8Z (not included); timezone: UTC",
			wantStart:    "2026-07-03T09:30:00.2Z",
			wantEnd:      "2026-07-03T09:30:00.8Z",
		},
		{
			name:         "local datetime with a fraction",
			from:         "2026/07/03 18:30:00.25",
			to:           "2026-07-03 18:30:01",
			wantOldest:   "1783071000.250000",
			wantLatest:   "1783071001.000000",
			wantProgress: "from 2026-07-03T09:30:00.25Z (included) to 2026-07-03T09:30:01Z (not included)",
			wantFooter:   "From 2026-07-03T18:30:00.25+09:00 (included); to 2026-07-03T18:30:01+09:00 (not included); timezone: JST",
			wantStart:    "2026-07-03T09:30:00.25Z",
			wantEnd:      "2026-07-03T09:30:01Z",
		},
		{
			name:         "finer than a microsecond rounds up",
			from:         "2026-07-03T09:30:00.1234561Z",
			to:           "2026-07-03T09:30:00.9999999Z",
			wantOldest:   "1783071000.123457",
			wantLatest:   "1783071001.000000",
			wantProgress: "from 2026-07-03T09:30:00.123457Z (included) to 2026-07-03T09:30:01Z (not included)",
			wantFooter:   "From 2026-07-03T09:30:00.123457Z (included); to 2026-07-03T09:30:01Z (not included); timezone: UTC",
			wantStart:    "2026-07-03T09:30:00.123457Z",
			wantEnd:      "2026-07-03T09:30:01Z",
		},
		{
			name:         "whole microseconds stay",
			from:         "2026-07-03T09:30:00.000001Z",
			to:           "2026-07-03T09:30:00.999999Z",
			wantOldest:   "1783071000.000001",
			wantLatest:   "1783071000.999999",
			wantProgress: "from 2026-07-03T09:30:00.000001Z (included) to 2026-07-03T09:30:00.999999Z (not included)",
			wantFooter:   "From 2026-07-03T09:30:00.000001Z (included); to 2026-07-03T09:30:00.999999Z (not included); timezone: UTC",
			wantStart:    "2026-07-03T09:30:00.000001Z",
			wantEnd:      "2026-07-03T09:30:00.999999Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := resolveDateTimeFetchRange(tt.from, tt.to, local)
			if err != nil {
				t.Fatalf("resolveDateTimeFetchRange: %v", err)
			}
			if got := r.oldestTS(); got != tt.wantOldest {
				t.Errorf("oldestTS = %q, want %q", got, tt.wantOldest)
			}
			if got := r.latestTS(); got != tt.wantLatest {
				t.Errorf("latestTS = %q, want %q", got, tt.wantLatest)
			}
			if got := r.progressLabel(); got != tt.wantProgress {
				t.Errorf("progress label = %q, want %q", got, tt.wantProgress)
			}
			if got := r.footerRangeLabel(); got != tt.wantFooter {
				t.Errorf("footer range = %q, want %q", got, tt.wantFooter)
			}
			target := r.metadataTargetRange()
			if target["start"] != tt.wantStart || target["end"] != tt.wantEnd ||
				target["start_slack_ts"] != tt.wantOldest || target["end_slack_ts"] != tt.wantLatest {
				t.Errorf("metadata target range = %v, want start %q end %q start_slack_ts %q end_slack_ts %q",
					target, tt.wantStart, tt.wantEnd, tt.wantOldest, tt.wantLatest)
			}
		})
	}
}

// TestResolveFetchRangeWholeSecondsKeepTheirOutput pins the outputs a
// whole-second range had before Issue #210 moved the Slack boundaries and the
// labels to the microsecond: the same ts parameters as slack.FormatTS and the
// same RFC3339 labels, for all three range modes.
func TestResolveFetchRangeWholeSecondsKeepTheirOutput(t *testing.T) {
	local := time.FixedZone("JST", 9*60*60)
	tests := []struct {
		name         string
		opts         Options
		now          time.Time
		wantOldest   string
		wantLatest   string
		wantProgress string
		wantFooter   string
	}{
		{
			name:         "from / to",
			opts:         Options{From: "2026-07-03T18:30", To: "2026-07-03T09:45:15Z"},
			wantOldest:   "1783071000.000000",
			wantLatest:   "1783071915.000000",
			wantProgress: "from 2026-07-03T09:30:00Z (included) to 2026-07-03T09:45:15Z (not included)",
			wantFooter:   "From 2026-07-03T09:30:00Z (included); to 2026-07-03T09:45:15Z (not included); timezone: UTC",
		},
		{
			name:         "date with a fraction of a second",
			opts:         Options{Date: "2026-07-03T09:30:00.5Z"},
			wantOldest:   "1783004400.000000",
			wantLatest:   "1783090800.000000",
			wantProgress: "on 2026-07-03 (local time)",
			wantFooter:   "From 2026-07-03T00:00:00+09:00 (included); to 2026-07-04T00:00:00+09:00 (not included); timezone: JST",
		},
		{
			name:         "days",
			opts:         Options{Days: 30},
			now:          time.Date(2026, 7, 3, 18, 30, 0, 0, local),
			wantOldest:   "1780479000.000000",
			wantLatest:   "1783071000.000000",
			wantProgress: "since 2026-06-03",
			wantFooter:   "From 2026-06-03T18:30:00+09:00 (included); to 2026-07-03T18:30:00+09:00 (not included); timezone: JST",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := resolveFetchRangeInLocation(tt.opts, tt.now, local)
			if err != nil {
				t.Fatalf("resolveFetchRangeInLocation: %v", err)
			}
			if got := r.oldestTS(); got != tt.wantOldest || got != slack.FormatTS(r.start.Unix()) {
				t.Errorf("oldestTS = %q, want %q (slack.FormatTS)", got, tt.wantOldest)
			}
			if got := r.latestTS(); got != tt.wantLatest || got != slack.FormatTS(r.end.Unix()) {
				t.Errorf("latestTS = %q, want %q (slack.FormatTS)", got, tt.wantLatest)
			}
			if got := r.progressLabel(); got != tt.wantProgress {
				t.Errorf("progress label = %q, want %q", got, tt.wantProgress)
			}
			if got := r.footerRangeLabel(); got != tt.wantFooter {
				t.Errorf("footer range = %q, want %q", got, tt.wantFooter)
			}
			target := r.metadataTargetRange()
			if target["start"] != r.start.UTC().Format(time.RFC3339) || target["end"] != r.end.UTC().Format(time.RFC3339) {
				t.Errorf("metadata target range = %v, want RFC3339 start / end", target)
			}
		})
	}
}

// TestResolveFetchRangeDaysCutsNowToWholeSecond: --days counts back from the
// export clock cut to the whole second, so the Slack boundaries, the footer and
// metadata.json agree on the end the footer shows (Issue #210).
func TestResolveFetchRangeDaysCutsNowToWholeSecond(t *testing.T) {
	local := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 7, 3, 18, 30, 0, 750_000_000, local)
	r, err := resolveFetchRangeInLocation(Options{Days: 30}, now, local)
	if err != nil {
		t.Fatalf("resolveFetchRangeInLocation: %v", err)
	}
	wantEnd := time.Date(2026, 7, 3, 18, 30, 0, 0, local)
	if !r.end.Equal(wantEnd) {
		t.Fatalf("end = %s, want %s", r.end.Format(time.RFC3339Nano), wantEnd.Format(time.RFC3339Nano))
	}
	if want := wantEnd.Add(-30 * 24 * time.Hour); !r.start.Equal(want) {
		t.Fatalf("start = %s, want %s", r.start.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
	if got, want := r.latestTS(), "1783071000.000000"; got != want {
		t.Errorf("latestTS = %q, want %q", got, want)
	}
	if got, want := r.oldestTS(), "1780479000.000000"; got != want {
		t.Errorf("oldestTS = %q, want %q", got, want)
	}
	if got, want := r.footerRangeLabel(), "From 2026-06-03T18:30:00+09:00 (included); to 2026-07-03T18:30:00+09:00 (not included); timezone: JST"; got != want {
		t.Errorf("footer range = %q, want %q", got, want)
	}
	if got, want := r.metadataTargetRange()["end"], "2026-07-03T09:30:00Z"; got != want {
		t.Errorf("metadata target_range.end = %v, want %q", got, want)
	}
}

func TestChooseDateTimeRangeDisplayTimezone(t *testing.T) {
	local := time.FixedZone("environment-zone", 8*60*60)
	tests := []struct {
		name       string
		from       string
		to         string
		loc        *time.Location
		wantLabel  string
		wantSource rangeTimezoneSource
		wantOffset int
	}{
		{
			name:       "same explicit offset",
			from:       "2026-07-03T09:00:00+09:00",
			to:         "2026-07-03T10:00:00+09:00",
			loc:        local,
			wantLabel:  "UTC+09:00",
			wantSource: rangeTimezoneSourceExplicitOffset,
			wantOffset: 9 * 60 * 60,
		},
		{
			name:       "only from has explicit offset",
			from:       "2026-07-03T09:00:00-07:00",
			to:         "2026-07-03T18:00:00",
			loc:        local,
			wantLabel:  "UTC-07:00",
			wantSource: rangeTimezoneSourceExplicitOffset,
			wantOffset: -7 * 60 * 60,
		},
		{
			name:       "only to has explicit offset",
			from:       "2026-07-03T09:00:00",
			to:         "2026-07-03T18:00:00Z",
			loc:        local,
			wantLabel:  "UTC",
			wantSource: rangeTimezoneSourceExplicitOffset,
		},
		{
			name:       "different explicit offsets",
			from:       "2026-07-03T09:00:00+09:00",
			to:         "2026-07-03T10:00:00+10:00",
			loc:        local,
			wantLabel:  "environment-zone",
			wantSource: rangeTimezoneSourceEnvironment,
			wantOffset: 8 * 60 * 60,
		},
		{
			name:       "local datetimes",
			from:       "2026-07-03T09:00:00",
			to:         "2026-07-03T18:00:00",
			loc:        local,
			wantLabel:  "environment-zone",
			wantSource: rangeTimezoneSourceEnvironment,
			wantOffset: 8 * 60 * 60,
		},
		{
			name:       "UTC fallback",
			from:       "2026-07-03T09:00:00",
			to:         "2026-07-03T18:00:00",
			wantLabel:  "UTC",
			wantSource: rangeTimezoneSourceUTCFallback,
		},
	}

	instant := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chooseDateTimeRangeDisplayTimezone(tt.from, tt.to, tt.loc)
			if got.label != tt.wantLabel || got.source != tt.wantSource {
				t.Fatalf("display timezone = %+v, want label %q source %q", got, tt.wantLabel, tt.wantSource)
			}
			_, gotOffset := instant.In(got.location).Zone()
			if gotOffset != tt.wantOffset {
				t.Fatalf("offset = %d, want %d", gotOffset, tt.wantOffset)
			}
		})
	}
}

func TestResolveFetchRangeDisplayTimezoneByMode(t *testing.T) {
	local := time.FixedZone("environment-zone", 8*60*60)
	tests := []struct {
		name string
		opts Options
	}{
		{
			name: "date ignores raw input offset",
			opts: Options{Date: "2026-07-03T09:00:00-07:00"},
		},
		{
			name: "days uses environment timezone",
			opts: Options{Days: 30},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := resolveFetchRangeInLocation(tt.opts, time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), local)
			if err != nil {
				t.Fatalf("resolveFetchRangeInLocation: %v", err)
			}
			if r.displayTimezone.label != "environment-zone" || r.displayTimezone.source != rangeTimezoneSourceEnvironment {
				t.Fatalf("display timezone = %+v, want environment timezone", r.displayTimezone)
			}
		})
	}
}

func TestFooterRangeLabelPreservesNamedTimezoneDSTOffsets(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load America/New_York: %v", err)
	}
	r := messageFetchRange{
		mode:            "datetime-range",
		start:           time.Date(2026, 3, 8, 6, 30, 0, 0, time.UTC),
		end:             time.Date(2026, 3, 8, 7, 30, 0, 0, time.UTC),
		displayTimezone: environmentRangeDisplayTimezone(newYork),
	}
	if got, want := r.footerRangeLabel(), "From 2026-03-08T01:30:00-05:00 (included); to 2026-03-08T03:30:00-04:00 (not included); timezone: America/New_York"; got != want {
		t.Fatalf("footer range = %q, want %q", got, want)
	}
}

// TestUTCOffsetFormats pins the two footer forms of an offset: the Range
// timezone label (formatUTCOffset) shows a zero offset as UTC, while the
// Exported line puts offsetString after "UTC" and shows it as UTC+00:00.
func TestUTCOffsetFormats(t *testing.T) {
	tests := []struct {
		offset        int
		wantOffset    string // offsetString
		wantUTCOffset string // formatUTCOffset
	}{
		{offset: 0, wantOffset: "+00:00", wantUTCOffset: "UTC"},
		{offset: 9 * 60 * 60, wantOffset: "+09:00", wantUTCOffset: "UTC+09:00"},
		{offset: -7 * 60 * 60, wantOffset: "-07:00", wantUTCOffset: "UTC-07:00"},
		{offset: 5*60*60 + 45*60, wantOffset: "+05:45", wantUTCOffset: "UTC+05:45"},
		{offset: -(3*60*60 + 30*60), wantOffset: "-03:30", wantUTCOffset: "UTC-03:30"},
		{offset: 9*60*60 + 18*60 + 59, wantOffset: "+09:18", wantUTCOffset: "UTC+09:18"}, // seconds dropped
	}
	for _, tt := range tests {
		if got := offsetString(tt.offset); got != tt.wantOffset {
			t.Errorf("offsetString(%d) = %q, want %q", tt.offset, got, tt.wantOffset)
		}
		if got := formatUTCOffset(tt.offset); got != tt.wantUTCOffset {
			t.Errorf("formatUTCOffset(%d) = %q, want %q", tt.offset, got, tt.wantUTCOffset)
		}
	}
}

func TestResolveDateTimeFetchRangeRejectsEmptyOrReversedRange(t *testing.T) {
	for _, to := range []string{"2026-07-03T09:30", "2026-07-03T09:00"} {
		if _, err := resolveDateTimeFetchRange("2026-07-03T09:30", to, time.Local); err == nil {
			t.Fatalf("resolveDateTimeFetchRange with to=%q succeeded, want error", to)
		}
	}
	for _, tt := range []struct{ name, from, to string }{
		{name: "same fraction", from: "2026-07-03T09:30:00.5Z", to: "2026-07-03T09:30:00.5Z"},
		{name: "reversed within a second", from: "2026-07-03T09:30:00.8Z", to: "2026-07-03T09:30:00.2Z"},
		// Both round up to 09:30:00.000001, so no ts value lies between them.
		{name: "empty after rounding up", from: "2026-07-03T09:30:00.0000001Z", to: "2026-07-03T09:30:00.0000009Z"},
	} {
		if _, err := resolveDateTimeFetchRange(tt.from, tt.to, time.UTC); err == nil {
			t.Fatalf("%s: resolveDateTimeFetchRange(%q, %q) succeeded, want error", tt.name, tt.from, tt.to)
		}
	}
}

func TestCeilMicrosecond(t *testing.T) {
	base := time.Date(2026, 7, 3, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		nsec int
		want int
	}{
		{nsec: 0, want: 0},
		{nsec: 1, want: 1_000},
		{nsec: 999, want: 1_000},
		{nsec: 1_000, want: 1_000},
		{nsec: 123_456_001, want: 123_457_000},
		{nsec: 999_999_001, want: 1_000_000_000},
	}
	for _, tt := range tests {
		got := ceilMicrosecond(base.Add(time.Duration(tt.nsec)))
		if want := base.Add(time.Duration(tt.want)); !got.Equal(want) {
			t.Errorf("ceilMicrosecond(+%dns) = %s, want %s", tt.nsec, got.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
		}
	}
	// Before the Unix epoch the rounding still goes up, toward the later
	// instant.
	early := time.Unix(-2, 250_000_001)
	if got, want := ceilMicrosecond(early), time.Unix(-2, 250_001_000); !got.Equal(want) {
		t.Errorf("ceilMicrosecond(%s) = %s, want %s", early.UTC().Format(time.RFC3339Nano), got.UTC().Format(time.RFC3339Nano), want.UTC().Format(time.RFC3339Nano))
	}
}

func TestResolveDateFetchRangeNormalizesParsedInstantToLocalDay(t *testing.T) {
	local := time.FixedZone("JST", 9*60*60)
	tests := []struct {
		name      string
		input     string
		wantStart string
	}{
		{name: "loose local input", input: "2026/07/03 09:30", wantStart: "2026-07-03T00:00:00+09:00"},
		{name: "offset input crossing local midnight", input: "2026-07-03T16:30:15-07:00", wantStart: "2026-07-04T00:00:00+09:00"},
		{name: "fraction of a second", input: "2026-07-03T16:30:15.5-07:00", wantStart: "2026-07-04T00:00:00+09:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := resolveDateFetchRange(tt.input, local)
			if err != nil {
				t.Fatalf("resolveDateFetchRange: %v", err)
			}
			if got := r.start.Format(time.RFC3339); got != tt.wantStart {
				t.Fatalf("start = %s, want %s", got, tt.wantStart)
			}
			if !r.end.Equal(r.start.AddDate(0, 0, 1)) {
				t.Fatalf("end = %s, want next local day", r.end)
			}
		})
	}
}
