// Package datetime parses the explicit date/time forms accepted by slapex CLI
// range options. It intentionally enumerates supported layouts instead of
// accepting natural-language or timezone-abbreviation input.
package datetime

import (
	"fmt"
	"time"
)

var localLayouts = []string{
	"2006-01-02",
	"2006/01/02",
	"2006-01-02T15",
	"2006-01-02 15",
	"2006/01/02T15",
	"2006/01/02 15",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006/01/02T15:04",
	"2006/01/02 15:04",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006/01/02T15:04:05",
	"2006/01/02 15:04:05",
}

// Parse parses RFC3339/RFC3339Nano as an absolute instant, or one of the
// explicitly supported loose layouts in loc. Missing time components are zero.
func Parse(input string, loc *time.Location) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, input); err == nil {
		return parsed, nil
	}
	for _, layout := range localLayouts {
		if parsed, err := time.ParseInLocation(layout, input, loc); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date/time %q", input)
}
