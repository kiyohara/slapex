// Command-line options: the flags, their two-pass parse around the channel
// argument, their validation and the usage diagnostics
// (doc/design/cli-interface.md「option 一覧」).

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/datetime"
	"github.com/kiyohara/slapex/internal/emoji"
)

var errUsage = errors.New("usage error")

type cliOptions struct {
	channel              string
	outputDir            string
	maxPosts             int
	days                 int
	date                 string
	from                 string
	to                   string
	excludeBodyEmoji     []string
	excludeReactionEmoji []string
	maxAttachBytes       int64
	keepCache            bool
	reuseCache           string
	noInteractive        bool
	noColor              bool
	demo                 bool
	showVersion          bool
}

func parseCLIArgs(args []string, diagnostics io.Writer) (*cliOptions, error) {
	fs := flag.NewFlagSet("slapex", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	var (
		outputDir            = fs.String("output", "", "output root directory (default: ./slapex-<yyyymmdd>-<hhmm>)")
		maxPosts             = fs.Int("max-posts", 1000, "maximum number of timeline parent messages (1-10000)")
		days                 = fs.Int("days", 30, "fetch messages newer than this many days (1-90)")
		date                 = fs.String("date", "", "fetch timeline messages on the local date containing this date/time")
		from                 = fs.String("from", "", "fetch timeline messages at or after this date/time (requires --to)")
		to                   = fs.String("to", "", "fetch timeline messages before this date/time (requires --from)")
		excludeBodyEmoji     = fs.String("exclude-body-emoji", "", "exclude messages containing any comma-separated emoji shortcode")
		excludeReactionEmoji = fs.String("exclude-reaction-emoji", "", "exclude messages with any comma-separated emoji reaction")
		maxAttach            = fs.String("max-attachment-size", "10MB", "per-file save limit for attachments and original images (e.g. 10MB, 512KB, 10485760)")
		keepCache            = fs.Bool("keep-cache", false, "keep the .cache/ directory regardless of the result")
		reuseCache           = fs.String("reuse-cache", "", "reuse a previously kept cache (path to output directory or .cache/)")
		noInteractive        = fs.Bool("no-interactive", false, "never prompt interactively (channel selection or SLACK_TOKEN entry)")
		noColor              = fs.Bool("no-color", false, "plain progress output: no colors, icons or animations (also via NO_COLOR, CI, TERM=dumb)")
		demoMode             = fs.Bool("demo", false, "export a bundled fictional sample without a Slack token or Slack App")
		showVersion          = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(diagnostics, "Usage: slapex [channel] [options]\n\n")
		fmt.Fprintf(diagnostics, "Exports Slack channel posts as locally browsable HTML with assets.\n")
		fmt.Fprintf(diagnostics, "The Slack OAuth token is taken from the %s environment variable.\n", slackTokenEnv)
		fmt.Fprintf(diagnostics, "To try it first without a Slack App or token, run: slapex --demo\n\n")
		fmt.Fprintf(diagnostics, "Options:\n")
		fs.PrintDefaults()
	}
	// The standard flag package stops parsing at the first non-flag
	// argument, but the spec allows options after the positional channel
	// (cli-interface.md: slapex [channel] [options]). Parse in two passes.
	if err := fs.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			err = fmt.Errorf("%w: %v", errUsage, err)
		}
		return nil, err
	}
	channel := ""
	if fs.NArg() > 0 {
		channel = fs.Arg(0)
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			if !errors.Is(err, flag.ErrHelp) {
				err = fmt.Errorf("%w: %v", errUsage, err)
			}
			return nil, err
		}
	}
	if *showVersion {
		return &cliOptions{showVersion: true}, nil
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(diagnostics, "slapex: too many arguments: %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return nil, errUsage
	}
	if *maxPosts < 1 || *maxPosts > 10000 {
		fmt.Fprintln(diagnostics, "slapex: --max-posts must be between 1 and 10000")
		return nil, errUsage
	}
	daysExplicit := false
	dateExplicit := false
	fromExplicit := false
	toExplicit := false
	excludeBodyEmojiExplicit := false
	excludeReactionEmojiExplicit := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "days":
			daysExplicit = true
		case "date":
			dateExplicit = true
		case "from":
			fromExplicit = true
		case "to":
			toExplicit = true
		case "exclude-body-emoji":
			excludeBodyEmojiExplicit = true
		case "exclude-reaction-emoji":
			excludeReactionEmojiExplicit = true
		}
	})
	if fromExplicit || toExplicit {
		if !fromExplicit || !toExplicit {
			fmt.Fprintln(diagnostics, "slapex: --from and --to must be used together")
			return nil, errUsage
		}
		if dateExplicit {
			fmt.Fprintln(diagnostics, "slapex: --from/--to and --date cannot be used together")
			return nil, errUsage
		}
		if daysExplicit {
			fmt.Fprintln(diagnostics, "slapex: --from/--to and --days cannot be used together")
			return nil, errUsage
		}
		fromTime, err := datetime.Parse(*from, time.Local)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --from %q (unsupported date/time format)\n", *from)
			return nil, errUsage
		}
		toTime, err := datetime.Parse(*to, time.Local)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --to %q (unsupported date/time format)\n", *to)
			return nil, errUsage
		}
		if !fromTime.Before(toTime) {
			fmt.Fprintln(diagnostics, "slapex: --from must be before --to")
			return nil, errUsage
		}
		*days = 0
	} else if dateExplicit {
		if _, err := datetime.Parse(*date, time.Local); err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --date %q (unsupported date/time format)\n", *date)
			return nil, errUsage
		}
		if daysExplicit {
			fmt.Fprintln(diagnostics, "slapex: --date and --days cannot be used together")
			return nil, errUsage
		}
		*days = 0
	}
	if !dateExplicit && !fromExplicit && !toExplicit && (*days < 1 || *days > 90) {
		fmt.Fprintln(diagnostics, "slapex: --days must be between 1 and 90")
		return nil, errUsage
	}
	maxAttachBytes, err := parseSize(*maxAttach)
	if err != nil || maxAttachBytes < 1024 {
		fmt.Fprintf(diagnostics, "slapex: invalid --max-attachment-size %q (expected e.g. 10MB, 512KB, or a byte count >= 1KB)\n", *maxAttach)
		return nil, errUsage
	}
	var excludedBodyEmoji []string
	if excludeBodyEmojiExplicit {
		excludedBodyEmoji, err = emoji.ParseList(*excludeBodyEmoji)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --exclude-body-emoji %q: %v\n", *excludeBodyEmoji, err)
			return nil, errUsage
		}
	}
	var excludedReactionEmoji []string
	if excludeReactionEmojiExplicit {
		excludedReactionEmoji, err = emoji.ParseList(*excludeReactionEmoji)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --exclude-reaction-emoji %q: %v\n", *excludeReactionEmoji, err)
			return nil, errUsage
		}
	}
	return &cliOptions{
		channel:              channel,
		outputDir:            *outputDir,
		maxPosts:             *maxPosts,
		days:                 *days,
		date:                 *date,
		from:                 *from,
		to:                   *to,
		excludeBodyEmoji:     excludedBodyEmoji,
		excludeReactionEmoji: excludedReactionEmoji,
		maxAttachBytes:       maxAttachBytes,
		keepCache:            *keepCache,
		reuseCache:           *reuseCache,
		noInteractive:        *noInteractive,
		noColor:              *noColor,
		demo:                 *demoMode,
		showVersion:          *showVersion,
	}, nil
}

// parseSize parses --max-attachment-size values: a plain byte count or an
// integer with a binary KB / MB / GB suffix (doc/design/cli-interface.md).
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)
	mult := int64(1)
	switch {
	case strings.HasSuffix(upper, "KB"):
		mult, upper = 1<<10, strings.TrimSuffix(upper, "KB")
	case strings.HasSuffix(upper, "MB"):
		mult, upper = 1<<20, strings.TrimSuffix(upper, "MB")
	case strings.HasSuffix(upper, "GB"):
		mult, upper = 1<<30, strings.TrimSuffix(upper, "GB")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(upper), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n * mult, nil
}
