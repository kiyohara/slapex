// Fetch range resolution: the --days / --date / --from / --to inputs become one
// absolute [start, end) window, the timezone used to display it, and the
// progress / footer labels derived from it (doc/design/cli-interface.md).

package export

import (
	"fmt"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/datetime"
	"github.com/kiyohara/slapex/internal/slack"
)

// rangeMode is the option that set the fetch range, recorded as
// fetch.options.range_mode in metadata.json (doc/design/cache.md). The labels
// and the recorded options handle date and datetime-range and fall back to
// the days form for any other mode.
type rangeMode string

const (
	rangeModeDays          rangeMode = "days"           // --days (the default)
	rangeModeDate          rangeMode = "date"           // --date
	rangeModeDateTimeRange rangeMode = "datetime-range" // --from / --to
)

type messageFetchRange struct {
	mode            rangeMode
	start           time.Time
	end             time.Time
	displayTimezone rangeDisplayTimezone
}

func resolveFetchRange(opts Options, now time.Time) (messageFetchRange, error) {
	return resolveFetchRangeInLocation(opts, now, time.Local)
}

func resolveFetchRangeInLocation(opts Options, now time.Time, loc *time.Location) (messageFetchRange, error) {
	if opts.From != "" || opts.To != "" {
		return resolveDateTimeFetchRange(opts.From, opts.To, loc)
	}
	if opts.Date == "" {
		return messageFetchRange{
			mode:            rangeModeDays,
			start:           now.Add(-time.Duration(opts.Days) * 24 * time.Hour),
			end:             now,
			displayTimezone: environmentRangeDisplayTimezone(loc),
		}, nil
	}
	return resolveDateFetchRange(opts.Date, loc)
}

func resolveDateTimeFetchRange(fromInput, toInput string, loc *time.Location) (messageFetchRange, error) {
	parseLocation := loc
	if parseLocation == nil {
		parseLocation = time.UTC
	}
	start, err := datetime.Parse(fromInput, parseLocation)
	if err != nil {
		return messageFetchRange{}, usagef("invalid from date/time %q", fromInput)
	}
	end, err := datetime.Parse(toInput, parseLocation)
	if err != nil {
		return messageFetchRange{}, usagef("invalid to date/time %q", toInput)
	}
	if !start.Before(end) {
		return messageFetchRange{}, usagef("from date/time must be before to date/time")
	}
	return messageFetchRange{
		mode:            rangeModeDateTimeRange,
		start:           start,
		end:             end,
		displayTimezone: chooseDateTimeRangeDisplayTimezone(fromInput, toInput, loc),
	}, nil
}

func resolveDateFetchRange(input string, loc *time.Location) (messageFetchRange, error) {
	displayTimezone := environmentRangeDisplayTimezone(loc)
	if loc == nil {
		loc = time.UTC
	}
	parsed, err := datetime.Parse(input, loc)
	if err != nil {
		return messageFetchRange{}, usagef("invalid date %q", input)
	}
	localDate := parsed.In(loc)
	start := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, loc)
	return messageFetchRange{
		mode:            rangeModeDate,
		start:           start,
		end:             start.AddDate(0, 0, 1),
		displayTimezone: displayTimezone,
	}, nil
}

type rangeTimezoneSource string

const (
	rangeTimezoneSourceExplicitOffset rangeTimezoneSource = "explicit-offset"
	rangeTimezoneSourceEnvironment    rangeTimezoneSource = "environment"
	rangeTimezoneSourceUTCFallback    rangeTimezoneSource = "utc-fallback"
)

type rangeDisplayTimezone struct {
	location *time.Location
	label    string
	source   rangeTimezoneSource
}

func chooseDateTimeRangeDisplayTimezone(fromInput, toInput string, loc *time.Location) rangeDisplayTimezone {
	fromOffset, fromExplicit := explicitUTCOffset(fromInput)
	toOffset, toExplicit := explicitUTCOffset(toInput)

	if fromExplicit && (!toExplicit || fromOffset == toOffset) {
		return fixedOffsetRangeDisplayTimezone(fromOffset)
	}
	if toExplicit && !fromExplicit {
		return fixedOffsetRangeDisplayTimezone(toOffset)
	}
	return environmentRangeDisplayTimezone(loc)
}

func explicitUTCOffset(input string) (int, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, input)
	if err != nil {
		return 0, false
	}
	_, offset := parsed.Zone()
	return offset, true
}

func fixedOffsetRangeDisplayTimezone(offset int) rangeDisplayTimezone {
	label := formatUTCOffset(offset)
	return rangeDisplayTimezone{
		location: time.FixedZone(label, offset),
		label:    label,
		source:   rangeTimezoneSourceExplicitOffset,
	}
}

func environmentRangeDisplayTimezone(loc *time.Location) rangeDisplayTimezone {
	if loc == nil {
		return rangeDisplayTimezone{
			location: time.UTC,
			label:    "UTC",
			source:   rangeTimezoneSourceUTCFallback,
		}
	}
	label := loc.String()
	if label == "" || label == "Local" {
		label = "local timezone"
	}
	return rangeDisplayTimezone{
		location: loc,
		label:    label,
		source:   rangeTimezoneSourceEnvironment,
	}
}

// formatUTCOffset labels a fixed offset as UTC±HH:MM
// (doc/design/output-format.md) and a zero offset as UTC. The spec does not
// decide the zero form, and the footer's Exported line shows a zero offset as
// UTC+00:00 (offsetString); Issue #211 kept both as they were.
func formatUTCOffset(offset int) string {
	if offset == 0 {
		return "UTC"
	}
	return "UTC" + offsetString(offset)
}

// offsetString formats an offset in seconds east of UTC as ±HH:MM, dropping
// seconds. The footer's Exported line (buildPage) also uses it, after "UTC",
// and shows a zero offset as UTC+00:00.
func offsetString(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d:%02d", sign, seconds/3600, (seconds%3600)/60)
}

func (r messageFetchRange) oldestTS() string { return slack.FormatTS(r.start.Unix()) }

func (r messageFetchRange) latestTS() string {
	if r.end.IsZero() {
		return ""
	}
	return slack.FormatTS(r.end.Unix())
}

func (r messageFetchRange) progressLabel() string {
	switch r.mode {
	case rangeModeDate:
		return "on " + r.start.Format("2006-01-02") + " (local time)"
	case rangeModeDateTimeRange:
		return "from " + r.start.UTC().Format(time.RFC3339) + " (included) to " + r.end.UTC().Format(time.RFC3339) + " (not included)"
	}
	return "since " + r.start.Format("2006-01-02")
}

func (r messageFetchRange) footerRangeLabel() string {
	displayTimezone := r.displayTimezone
	if displayTimezone.location == nil {
		displayTimezone = environmentRangeDisplayTimezone(nil)
	}
	start := r.start.In(displayTimezone.location).Format(time.RFC3339)
	if r.end.IsZero() {
		return fmt.Sprintf("From %s (included); no end boundary; timezone: %s", start, displayTimezone.label)
	}
	return fmt.Sprintf(
		"From %s (included); to %s (not included); timezone: %s",
		start,
		r.end.In(displayTimezone.location).Format(time.RFC3339),
		displayTimezone.label,
	)
}

func (r messageFetchRange) footerOptionsLabel(opts Options) string {
	limit := fmt.Sprintf("--max-posts %d, --max-attachment-size %s", opts.MaxPosts, humanBytes(opts.MaxAttachBytes))
	if len(opts.ExcludeBodyEmoji) > 0 {
		limit += ", --exclude-body-emoji " + strings.Join(opts.ExcludeBodyEmoji, ",")
	}
	if len(opts.ExcludeReactionEmoji) > 0 {
		limit += ", --exclude-reaction-emoji " + strings.Join(opts.ExcludeReactionEmoji, ",")
	}
	switch r.mode {
	case rangeModeDate:
		return fmt.Sprintf("--date %q, %s", opts.Date, limit)
	case rangeModeDateTimeRange:
		return fmt.Sprintf("--from %q, --to %q, %s", opts.From, opts.To, limit)
	}
	return fmt.Sprintf("--days %d, %s", opts.Days, limit)
}
