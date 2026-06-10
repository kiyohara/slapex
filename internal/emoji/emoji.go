// Package emoji resolves emoji shortcodes to Unicode characters or to local
// image assets, per doc/design/slack-api-usage.md: standard emoji render as
// Unicode text, custom emoji (and aliases) become downloaded image assets,
// unknown shortcodes stay as :name: literals.
package emoji

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed data/emoji.json
var rawStandard []byte

const maxAliasDepth = 10

// Resolution describes how a shortcode should be rendered.
type Resolution struct {
	Unicode  string // non-empty: render as text
	ImageURL string // non-empty: render as image asset (custom emoji URL)
	Literal  string // non-empty: render literally (unknown shortcode)
}

type Resolver struct {
	standard map[string]string
	custom   map[string]string
}

// NewResolver builds a resolver from the embedded standard table and the
// workspace custom emoji map from emoji.list (may be nil).
func NewResolver(custom map[string]string) (*Resolver, error) {
	standard := map[string]string{}
	if err := json.Unmarshal(rawStandard, &standard); err != nil {
		return nil, err
	}
	return &Resolver{standard: standard, custom: custom}, nil
}

// Resolve resolves a shortcode (without colons). Skin tone suffixes are
// folded onto the base emoji per the spec.
func (r *Resolver) Resolve(name string) Resolution {
	if i := strings.Index(name, "::skin-tone-"); i >= 0 {
		name = name[:i]
	}
	for depth := 0; depth < maxAliasDepth; depth++ {
		if url, ok := r.custom[name]; ok {
			if alias, isAlias := strings.CutPrefix(url, "alias:"); isAlias {
				name = alias
				continue
			}
			return Resolution{ImageURL: url}
		}
		if ch, ok := r.standard[name]; ok {
			return Resolution{Unicode: ch}
		}
		break
	}
	return Resolution{Literal: ":" + name + ":"}
}
