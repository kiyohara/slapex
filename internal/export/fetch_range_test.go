package export

import (
	"testing"
	"time"
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

func TestResolveDateTimeFetchRangeRejectsEmptyOrReversedRange(t *testing.T) {
	for _, to := range []string{"2026-07-03T09:30", "2026-07-03T09:00"} {
		if _, err := resolveDateTimeFetchRange("2026-07-03T09:30", to, time.Local); err == nil {
			t.Fatalf("resolveDateTimeFetchRange with to=%q succeeded, want error", to)
		}
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
