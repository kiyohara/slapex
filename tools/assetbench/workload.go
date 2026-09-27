package main

// Workloads: the assets a benchmark run downloads, and the origins they come
// from. The presets are modeled on the exports measured for #272; a JSON file
// of the same shape (-print-workload prints one) can describe another export.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"

	"github.com/kiyohara/slapex/internal/output"
)

// Workload is the assets of one export and the origins they come from.
type Workload struct {
	Name    string   `json:"name,omitempty"`
	Origins []Origin `json:"origins"`
	Assets  []Asset  `json:"assets"`
}

// Origin is one fake origin (one scheme, host and port), served over TLS.
type Origin struct {
	// Class groups the origin in the report: files.slack.com, Slack CDN,
	// gravatar or other.
	Class string `json:"class"`
	// HTTP2 serves HTTP/2, and HTTP/1.1 otherwise.
	HTTP2 bool `json:"http2"`
	// HandshakeMS delays every TLS handshake, standing in for the round
	// trips of TCP and TLS to a distant origin. FirstByteMS delays every
	// response.
	HandshakeMS int `json:"handshake_ms"`
	FirstByteMS int `json:"first_byte_ms"`
	// BytesPerSec is the origin's bandwidth, which all its responses share;
	// 0 is unlimited.
	BytesPerSec int64 `json:"bytes_per_sec"`
	// MaxStreams is the HTTP/2 SETTINGS_MAX_CONCURRENT_STREAMS; 0 leaves
	// Go's default.
	MaxStreams int `json:"max_streams,omitempty"`
}

// Asset is one download.
type Asset struct {
	Origin int    `json:"origin"` // index into Workload.Origins
	Kind   string `json:"kind"`   // an output.Kind* value
	Bytes  int64  `json:"bytes"`
}

const (
	kib = 1 << 10
	mib = 1 << 20
)

var presets = map[string]func() Workload{
	"recent": recentWorkload,
	"small":  smallWorkload,
}

var assetKinds = []string{
	output.KindEmoji, output.KindOGImage, output.KindUploadThumb, output.KindUploadOriginal,
	output.KindAttachment, output.KindAvatar, output.KindServiceIcon, output.KindWorkspaceIcon,
}

// loadWorkload returns the preset of that name, or reads the JSON file.
func loadWorkload(nameOrPath string) (Workload, error) {
	if preset, ok := presets[nameOrPath]; ok {
		return preset(), nil
	}
	data, err := os.ReadFile(nameOrPath)
	if err != nil {
		return Workload{}, fmt.Errorf("workload %q is neither a preset nor a readable file: %w", nameOrPath, err)
	}
	var w Workload
	if err := json.Unmarshal(data, &w); err != nil {
		return Workload{}, fmt.Errorf("workload %s: %w", nameOrPath, err)
	}
	if w.Name == "" {
		w.Name = nameOrPath
	}
	return w, w.validate()
}

func (w Workload) validate() error {
	if len(w.Assets) == 0 {
		return errors.New("the workload has no assets")
	}
	for i, o := range w.Origins {
		if o.HandshakeMS < 0 || o.FirstByteMS < 0 || o.BytesPerSec < 0 || o.MaxStreams < 0 {
			return fmt.Errorf("origin %d has a negative value", i)
		}
	}
	for i, a := range w.Assets {
		switch {
		case a.Origin < 0 || a.Origin >= len(w.Origins):
			return fmt.Errorf("asset %d: there is no origin %d", i, a.Origin)
		case !slices.Contains(assetKinds, a.Kind):
			return fmt.Errorf("asset %d: unknown kind %q", i, a.Kind)
		case a.Bytes < 0:
			return fmt.Errorf("asset %d has a negative size", i)
		}
	}
	return nil
}

// recentWorkload is modeled on the latest export measured for #272: 56 assets
// from 25 origins. files.slack.com has 4, the Slack CDN 14 on 4 hosts (9 on
// one of them), gravatar 5, and 19 third-party origins 33, with 1 to 3 each.
// The Slack hosts and gravatar spoke HTTP/2 with 128 streams and took 30 to
// 56 ms to connect. The sizes, and the latency, bandwidth and HTTP version of
// the third-party origins, are assumptions.
func recentWorkload() Workload {
	w := Workload{Name: "recent"}
	origin := func(o Origin) int {
		w.Origins = append(w.Origins, o)
		return len(w.Origins) - 1
	}
	slackOrigin := func(class string, handshakeMS, firstByteMS int) int {
		return origin(Origin{Class: class, HTTP2: true, HandshakeMS: handshakeMS, FirstByteMS: firstByteMS,
			BytesPerSec: 16 * mib, MaxStreams: 128})
	}
	add := func(origin int, kind string, sizes ...int64) {
		for _, n := range sizes {
			w.Assets = append(w.Assets, Asset{Origin: origin, Kind: kind, Bytes: n})
		}
	}

	files := slackOrigin("files.slack.com", 45, 120)
	add(files, output.KindUploadThumb, 60*kib, 45*kib)
	add(files, output.KindUploadOriginal, 1843*kib)
	add(files, output.KindAttachment, 2662*kib)
	avatars := slackOrigin("Slack CDN", 40, 40)
	add(avatars, output.KindAvatar, slices.Repeat([]int64{12 * kib}, 8)...)
	add(avatars, output.KindWorkspaceIcon, 10*kib)
	emoji := slackOrigin("Slack CDN", 38, 35)
	add(emoji, output.KindEmoji, 5*kib, 5*kib, 5*kib)
	add(slackOrigin("Slack CDN", 36, 35), output.KindAvatar, 8*kib)
	add(slackOrigin("Slack CDN", 42, 40), output.KindAvatar, 12*kib)
	gravatar := origin(Origin{Class: "gravatar", HTTP2: true, HandshakeMS: 44, FirstByteMS: 60,
		BytesPerSec: 8 * mib, MaxStreams: 128})
	add(gravatar, output.KindAvatar, slices.Repeat([]int64{9 * kib}, 5)...)

	// Third-party origins: 4 with 3 assets, 6 with 2 and 9 with 1; every
	// third one speaks HTTP/1.1.
	for i := range 19 {
		o := Origin{Class: "other", HTTP2: i%3 != 2, HandshakeMS: 50 + i*17%60, FirstByteMS: 80 + i*53%240,
			BytesPerSec: int64(2+i%4*2) * mib}
		if o.HTTP2 {
			o.MaxStreams = 100
		}
		id := origin(o)
		n := 1
		switch {
		case i < 4:
			n = 3
		case i < 10:
			n = 2
		}
		for j := range n {
			if j == 1 {
				add(id, output.KindServiceIcon, 4*kib)
			} else {
				add(id, output.KindOGImage, int64(90+(i*7+j*13)%16*10)*kib)
			}
		}
	}
	shuffle(w.Assets)
	return w
}

// smallWorkload is a small export, 8 assets from 5 origins, for a quick run.
func smallWorkload() Workload {
	w := recentWorkload()
	keep := []int{0, 1, 5, 6, 8} // files.slack.com, the avatar CDN, gravatar, two third parties (HTTP/2, HTTP/1.1)
	small := Workload{Name: "small"}
	for _, i := range keep {
		small.Origins = append(small.Origins, w.Origins[i])
	}
	add := func(origin int, kind string, n int64) {
		small.Assets = append(small.Assets, Asset{Origin: origin, Kind: kind, Bytes: n})
	}
	add(0, output.KindUploadThumb, 60*kib)
	add(1, output.KindWorkspaceIcon, 10*kib)
	add(1, output.KindAvatar, 12*kib)
	add(1, output.KindAvatar, 12*kib)
	add(2, output.KindAvatar, 9*kib)
	add(3, output.KindOGImage, 150*kib)
	add(3, output.KindServiceIcon, 4*kib)
	add(4, output.KindOGImage, 120*kib)
	shuffle(small.Assets)
	return small
}

// shuffle mixes the assets as an export meets them, the same way every time.
func shuffle(assets []Asset) {
	r := rand.New(rand.NewPCG(272, 273))
	r.Shuffle(len(assets), func(i, j int) { assets[i], assets[j] = assets[j], assets[i] })
}
