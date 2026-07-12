package datetime

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	local := time.FixedZone("local-test", 9*60*60)
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "hyphen date", input: "2026-07-03", want: "2026-07-03T00:00:00+09:00"},
		{name: "slash date", input: "2026/07/03", want: "2026-07-03T00:00:00+09:00"},
		{name: "hour", input: "2026-07-03T09", want: "2026-07-03T09:00:00+09:00"},
		{name: "minute", input: "2026/07/03 09:30", want: "2026-07-03T09:30:00+09:00"},
		{name: "second", input: "2026-07-03T09:30:15", want: "2026-07-03T09:30:15+09:00"},
		{name: "rfc3339", input: "2026-07-03T09:30:15-07:00", want: "2026-07-03T09:30:15-07:00"},
		{name: "rfc3339 nano", input: "2026-07-03T09:30:15.123456789Z", want: "2026-07-03T09:30:15.123456789Z"},
		{name: "invalid date", input: "2026-02-30", wantErr: true},
		{name: "invalid hour", input: "2026-07-03T25:00:00", wantErr: true},
		{name: "invalid minute", input: "2026-07-03T09:60:00", wantErr: true},
		{name: "timezone abbreviation", input: "2026-07-03T09:00:00JST", wantErr: true},
		{name: "natural language", input: "yesterday", wantErr: true},
		{name: "japanese date", input: "2026年07月03日", wantErr: true},
		{name: "two digit year", input: "26-07-03", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input, local)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) succeeded: %s", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			if formatted := got.Format(time.RFC3339Nano); formatted != tt.want {
				t.Fatalf("Parse(%q) = %s, want %s", tt.input, formatted, tt.want)
			}
		})
	}
}
