// Command genemoji generates internal/emoji/data/emoji.json, the embedded
// shortcode -> Unicode table for standard emoji (doc/design/slack-api-usage.md).
//
// The source dataset is iamcal/emoji-data (MIT), the dataset Slack itself is
// known to use. Run via:
//
//	docker compose run --rm dev go run ./tools/genemoji
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const sourceURL = "https://raw.githubusercontent.com/iamcal/emoji-data/master/emoji.json"

type entry struct {
	Unified    string   `json:"unified"`
	ShortName  string   `json:"short_name"`
	ShortNames []string `json:"short_names"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genemoji:", err)
		os.Exit(1)
	}
}

func run() error {
	resp, err := http.Get(sourceURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: HTTP %d", sourceURL, resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var entries []entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return err
	}

	table := map[string]string{}
	for _, e := range entries {
		ch, err := unifiedToString(e.Unified)
		if err != nil {
			return fmt.Errorf("entry %s: %w", e.ShortName, err)
		}
		names := e.ShortNames
		if len(names) == 0 {
			names = []string{e.ShortName}
		}
		for _, name := range names {
			if name != "" {
				table[name] = ch
			}
		}
	}

	out, err := json.MarshalIndent(table, "", "")
	if err != nil {
		return err
	}
	path := "internal/emoji/data/emoji.json"
	if err := os.MkdirAll("internal/emoji/data", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "genemoji: wrote %d shortcodes to %s\n", len(table), path)
	return nil
}

// unifiedToString converts "1F1EF-1F1F5" style codepoint lists to a string.
func unifiedToString(unified string) (string, error) {
	var sb strings.Builder
	for _, part := range strings.Split(unified, "-") {
		cp, err := strconv.ParseInt(part, 16, 32)
		if err != nil {
			return "", err
		}
		sb.WriteRune(rune(cp))
	}
	return sb.String(), nil
}
