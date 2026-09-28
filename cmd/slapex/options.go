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
	"github.com/kiyohara/slapex/internal/export"
)

var errUsage = errors.New("usage error")

// cliOptions are the options of a command line. The flags set them, and
// validate checks them and fills in those it converts (rawFlags).
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

// exportOptions returns the export options the command line selects. A normal
// run and --demo both start from them, so an option set here reaches both: the
// normal run adds its controlling terminal (PromptTTY), and demo.Run replaces
// what its fixture decides (the channel, resolved without prompting).
func (o *cliOptions) exportOptions() export.Options {
	return export.Options{
		ChannelKeyword:       o.channel,
		OutputDir:            o.outputDir,
		MaxPosts:             o.maxPosts,
		Days:                 o.days,
		Date:                 o.date,
		From:                 o.from,
		To:                   o.to,
		ExcludeBodyEmoji:     o.excludeBodyEmoji,
		ExcludeReactionEmoji: o.excludeReactionEmoji,
		MaxAttachBytes:       o.maxAttachBytes,
		KeepCache:            o.keepCache,
		ReuseCache:           o.reuseCache,
		NoInteractive:        o.noInteractive,
		ToolVersion:          version,
	}
}

// rawFlags hold the text of the flags whose values validate converts: the
// attachment size and the emoji lists.
type rawFlags struct {
	maxAttachmentSize    string
	excludeBodyEmoji     string
	excludeReactionEmoji string
}

// parseCLIArgs parses the command line args, without the program name. On a
// usage error it prints the diagnostics on diagnostics and returns an error
// that is errUsage (errors.Is); --help returns flag.ErrHelp after the usage.
func parseCLIArgs(args []string, diagnostics io.Writer) (*cliOptions, error) {
	opts := &cliOptions{}
	var raw rawFlags
	fs := newFlagSet(opts, &raw, diagnostics)
	// The standard flag package stops parsing at the first non-flag
	// argument, but the spec allows options after the positional channel
	// (cli-interface.md: slapex [channel] [options]). Parse in two passes.
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		opts.channel = fs.Arg(0)
		if err := parseFlags(fs, fs.Args()[1:]); err != nil {
			return nil, err
		}
	}
	if opts.showVersion {
		return &cliOptions{showVersion: true}, nil
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(diagnostics, "slapex: too many arguments: %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return nil, errUsage
	}
	if err := opts.validate(raw, givenFlags(fs)); err != nil {
		fmt.Fprintf(diagnostics, "slapex: %v\n", err)
		return nil, errUsage
	}
	return opts, nil
}

// newFlagSet defines the flags of the command line, bound to opts or, for the
// values validate converts, to raw, and the usage printed on diagnostics for
// --help and with a flag error.
func newFlagSet(opts *cliOptions, raw *rawFlags, diagnostics io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("slapex", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	fs.StringVar(&opts.outputDir, "output", "", "output root directory (default: ./slapex-<yyyymmdd>-<hhmm>)")
	fs.IntVar(&opts.maxPosts, "max-posts", 1000, "maximum number of timeline parent messages (1-10000)")
	fs.IntVar(&opts.days, "days", 30, "fetch messages newer than this many days (1-90)")
	fs.StringVar(&opts.date, "date", "", "fetch timeline messages on the local date containing this date/time")
	fs.StringVar(&opts.from, "from", "", "fetch timeline messages at or after this date/time (requires --to)")
	fs.StringVar(&opts.to, "to", "", "fetch timeline messages before this date/time (requires --from)")
	fs.StringVar(&raw.excludeBodyEmoji, "exclude-body-emoji", "", "exclude messages containing any comma-separated emoji shortcode")
	fs.StringVar(&raw.excludeReactionEmoji, "exclude-reaction-emoji", "", "exclude messages with any comma-separated emoji reaction")
	fs.StringVar(&raw.maxAttachmentSize, "max-attachment-size", "10MB", "per-file save limit for attachments and original images (e.g. 10MB, 512KB, 10485760)")
	fs.BoolVar(&opts.keepCache, "keep-cache", false, "keep the .cache/ directory regardless of the result")
	fs.StringVar(&opts.reuseCache, "reuse-cache", "", "reuse a previously kept cache (path to output directory or .cache/)")
	fs.BoolVar(&opts.noInteractive, "no-interactive", false, "never prompt interactively (channel selection or SLACK_TOKEN entry)")
	fs.BoolVar(&opts.noColor, "no-color", false, "plain progress output: no colors, icons or animations (also via NO_COLOR, CI, TERM=dumb)")
	fs.BoolVar(&opts.demo, "demo", false, "export a bundled fictional sample without a Slack token or Slack App")
	fs.BoolVar(&opts.showVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(diagnostics, "Usage: slapex [channel] [options]\n\n")
		fmt.Fprintf(diagnostics, "Exports Slack channel posts as locally browsable HTML with assets.\n")
		fmt.Fprintf(diagnostics, "The Slack OAuth token is taken from the %s environment variable.\n", slackTokenEnv)
		fmt.Fprintf(diagnostics, "To try it first without a Slack App or token, run: slapex --demo\n\n")
		fmt.Fprintf(diagnostics, "Options:\n")
		fs.PrintDefaults()
	}
	return fs
}

// parseFlags parses the flags in args. fs reports a flag error itself, with
// the usage, and parseFlags marks it a usage error; --help is flag.ErrHelp.
func parseFlags(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(args)
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		err = fmt.Errorf("%w: %v", errUsage, err)
	}
	return err
}

// givenFlags returns the names of the flags the command line gives, also those
// given their default value.
func givenFlags(fs *flag.FlagSet) map[string]bool {
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	return given
}

// validate checks the options the flags set and fills in those it converts
// from raw. given holds the flags the command line gives: a given --date,
// --from or --to replaces the default --days, and a given emoji list turns its
// filter on. The error is the diagnostic of the first problem found.
func (o *cliOptions) validate(raw rawFlags, given map[string]bool) error {
	if o.maxPosts < 1 || o.maxPosts > 10000 {
		return errors.New("--max-posts must be between 1 and 10000")
	}
	if err := o.validateFetchRange(given); err != nil {
		return err
	}
	var err error
	if o.maxAttachBytes, err = parseMaxAttachmentSize(raw.maxAttachmentSize); err != nil {
		return err
	}
	if o.excludeBodyEmoji, err = parseEmojiFilter("exclude-body-emoji", raw.excludeBodyEmoji, given); err != nil {
		return err
	}
	if o.excludeReactionEmoji, err = parseEmojiFilter("exclude-reaction-emoji", raw.excludeReactionEmoji, given); err != nil {
		return err
	}
	return nil
}

// validateFetchRange checks the fetch range: --from with --to, else --date,
// else --days. --from/--to and --date each select the range themselves and
// exclude --days, so days becomes 0 instead of its default.
func (o *cliOptions) validateFetchRange(given map[string]bool) error {
	switch {
	case given["from"] || given["to"]:
		if !given["from"] || !given["to"] {
			return errors.New("--from and --to must be used together")
		}
		if given["date"] {
			return errors.New("--from/--to and --date cannot be used together")
		}
		if given["days"] {
			return errors.New("--from/--to and --days cannot be used together")
		}
		from, err := datetime.Parse(o.from, time.Local)
		if err != nil {
			return fmt.Errorf("invalid --from %q (unsupported date/time format)", o.from)
		}
		to, err := datetime.Parse(o.to, time.Local)
		if err != nil {
			return fmt.Errorf("invalid --to %q (unsupported date/time format)", o.to)
		}
		if !from.Before(to) {
			return errors.New("--from must be before --to")
		}
		o.days = 0
	case given["date"]:
		if _, err := datetime.Parse(o.date, time.Local); err != nil {
			return fmt.Errorf("invalid --date %q (unsupported date/time format)", o.date)
		}
		if given["days"] {
			return errors.New("--date and --days cannot be used together")
		}
		o.days = 0
	case o.days < 1 || o.days > 90:
		return errors.New("--days must be between 1 and 90")
	}
	return nil
}

// parseMaxAttachmentSize parses --max-attachment-size, at least 1KB.
func parseMaxAttachmentSize(value string) (int64, error) {
	n, err := parseSize(value)
	if err != nil || n < 1024 {
		return 0, fmt.Errorf("invalid --max-attachment-size %q (expected e.g. 10MB, 512KB, or a byte count >= 1KB)", value)
	}
	return n, nil
}

// parseEmojiFilter parses the emoji list of the filter flag name. The filter
// is off (nil) unless the flag is given; given, even empty, the list must
// parse (emoji.ParseList), so --name "" is an error rather than no filter.
func parseEmojiFilter(name, value string, given map[string]bool) ([]string, error) {
	if !given[name] {
		return nil, nil
	}
	list, err := emoji.ParseList(value)
	if err != nil {
		return nil, fmt.Errorf("invalid --%s %q: %v", name, value, err)
	}
	return list, nil
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
