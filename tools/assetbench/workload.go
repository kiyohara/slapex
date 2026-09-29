package main

// Workloads: the assets a benchmark run downloads, and the origins they come
// from. The presets traced and recent are modeled on the exports measured for
// #272, and heavy on an export with many more uploads; a JSON file of the same
// shape (-print-workload prints one) can describe another export.

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
	// LimitPerSec, when not 0, is a rate limit the origin keeps (Issue
	// #276): a token bucket of LimitBurst requests, at least 1, that
	// refills at LimitPerSec requests a second. RefuseForMS, when not 0, is
	// a passing one: the origin refuses the requests that come in the
	// RefuseForMS from RefuseAfterMS after a run starts. A refused request
	// gets a 429 after the first-byte delay, with a Retry-After of
	// RetryAfterS seconds when that is not 0.
	LimitPerSec   float64 `json:"limit_per_sec,omitempty"`
	LimitBurst    int     `json:"limit_burst,omitempty"`
	RefuseAfterMS int     `json:"refuse_after_ms,omitempty"`
	RefuseForMS   int     `json:"refuse_for_ms,omitempty"`
	RetryAfterS   int     `json:"retry_after_s,omitempty"`
}

// rateLimited reports whether the origin refuses any request.
func (o Origin) rateLimited() bool { return o.LimitPerSec > 0 || o.RefuseForMS > 0 }

// Asset is one download.
type Asset struct {
	Origin int    `json:"origin"` // index into Workload.Origins
	Kind   string `json:"kind"`   // an output.Kind* value
	Bytes  int64  `json:"bytes"`
	// Via, when set, is the origin the page links the asset at, which
	// redirects to Origin.
	Via *int `json:"via,omitempty"`
}

const (
	kib = 1 << 10
	mib = 1 << 20
)

var presets = map[string]func() Workload{
	"traced":  tracedWorkload,
	"heavy":   heavyWorkload,
	"recent":  recentWorkload,
	"small":   smallWorkload,
	"limited": limitedWorkload,
	"spike":   spikeWorkload,
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
		if o.HandshakeMS < 0 || o.FirstByteMS < 0 || o.BytesPerSec < 0 || o.MaxStreams < 0 ||
			o.LimitPerSec < 0 || o.LimitBurst < 0 || o.RefuseAfterMS < 0 || o.RefuseForMS < 0 || o.RetryAfterS < 0 {
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
		case a.Via != nil && (*a.Via < 0 || *a.Via >= len(w.Origins) || *a.Via == a.Origin):
			return fmt.Errorf("asset %d: origin %d cannot redirect to it", i, *a.Via)
		}
	}
	return nil
}

// builder adds origins and assets to a workload.
type builder struct{ w *Workload }

func (b builder) origin(o Origin) int {
	b.w.Origins = append(b.w.Origins, o)
	return len(b.w.Origins) - 1
}

// add adds an asset of kind on origin for each size.
func (b builder) add(origin int, kind string, sizes ...int64) {
	for _, n := range sizes {
		b.w.Assets = append(b.w.Assets, Asset{Origin: origin, Kind: kind, Bytes: n})
	}
}

// redirected adds an asset of kind on origin that the page links at via.
func (b builder) redirected(via, origin int, kind string, n int64) {
	b.w.Assets = append(b.w.Assets, Asset{Origin: origin, Kind: kind, Bytes: n, Via: &via})
}

// tracedWorkload is modeled on the export traced for #272 before PF-03
// (2026-09-29): 56 assets in 59 requests to 27 origins. files.slack.com has 2
// thumbnails and 2 originals of 3.9 MB together; the Slack CDN 15 on 2 hosts
// (9 avatars and the workspace icon; 5 emoji); gravatar 4 requests, 2 of them
// redirects of avatars to third-party hosts; and 23 third-party origins 36
// requests (one with 4, ten with 2, twelve with 1; 3 of them HTTP/1.1), for 19
// OG images, 14 service icons (one of them redirected) and the 2 avatars. The
// delays follow the trace's averages per class: to connect, 45 ms to
// files.slack.com, 50 and 60 ms to the CDN hosts, 33 ms to gravatar, 100 ms to
// a third-party host (35 to 164 ms); to the first byte, 890 ms from
// files.slack.com, 220 ms (avatars) and 12 ms (emoji) from the CDN, 7 ms from
// gravatar, 150 ms from a third-party host (40 to 299 ms). The sizes of the
// assets of a kind vary around the trace's average; the bandwidths are
// assumptions, 48 MiB/s from files.slack.com after the trace's transfers.
func tracedWorkload() Workload {
	w := Workload{Name: "traced"}
	b := builder{&w}
	// The Slack hosts and gravatar speak HTTP/2 with 128 streams.
	slackOrigin := func(class string, handshakeMS, firstByteMS int, bytesPerSec int64) int {
		return b.origin(Origin{Class: class, HTTP2: true, HandshakeMS: handshakeMS, FirstByteMS: firstByteMS,
			BytesPerSec: bytesPerSec, MaxStreams: 128})
	}

	files := slackOrigin("files.slack.com", 45, 890, 48*mib)
	b.add(files, output.KindUploadThumb, 24*kib, 20*kib)
	b.add(files, output.KindUploadOriginal, 2200*kib, 1790*kib)
	avatars := slackOrigin("Slack CDN", 60, 220, 16*mib)
	b.add(avatars, output.KindAvatar, 6*kib, 4*kib, 7*kib, 5*kib, 8*kib, 5*kib, 6*kib, 4*kib, 6*kib)
	b.add(avatars, output.KindWorkspaceIcon, 6*kib)
	emoji := slackOrigin("Slack CDN", 50, 12, 16*mib)
	b.add(emoji, output.KindEmoji, 4*kib, 5*kib, 3*kib, 6*kib, 4*kib)
	gravatar := slackOrigin("gravatar", 33, 7, 16*mib)
	b.add(gravatar, output.KindAvatar, 7*kib, 8*kib)

	others := make([]int, 23)
	for i := range others {
		o := Origin{Class: "other", HTTP2: i%8 != 5, HandshakeMS: 35 + i*37%130, FirstByteMS: 40 + i*53%260,
			BytesPerSec: int64(2+i%4) * mib}
		if o.HTTP2 {
			o.MaxStreams = 100
		}
		others[i] = b.origin(o)
	}
	b.add(others[0], output.KindOGImage, ogImageSize(0), ogImageSize(1))
	b.add(others[0], output.KindServiceIcon, serviceIconSize(0), serviceIconSize(1))
	for i := 1; i <= 10; i++ {
		b.add(others[i], output.KindOGImage, ogImageSize(i+1))
		b.add(others[i], output.KindServiceIcon, serviceIconSize(i+1))
	}
	for i := 11; i <= 17; i++ {
		b.add(others[i], output.KindOGImage, ogImageSize(i+1))
	}
	b.add(others[18], output.KindServiceIcon, serviceIconSize(12))
	b.redirected(others[19], others[20], output.KindServiceIcon, serviceIconSize(13))
	b.redirected(gravatar, others[21], output.KindAvatar, 5*kib)
	b.redirected(gravatar, others[22], output.KindAvatar, 6*kib)
	shuffle(w.Assets)
	return w
}

// ogImageSize and serviceIconSize are the sizes of the i-th OG image (60 to
// 559 KiB, 300 KB on average in the trace) and service icon (2 to 31 KiB, 16
// KB on average).
func ogImageSize(i int) int64     { return int64(60+i*137%500) * kib }
func serviceIconSize(i int) int64 { return int64(2+i*11%30) * kib }

// heavyWorkload is an export with many more uploads than the traced one, to
// weigh the lane limits of a busy origin and the limit on large downloads:
// 96 thumbnails (20 to 76 KiB), 24 originals (0.5 to 8 MiB) and 8
// attachments (1 to 6 MiB) from files.slack.com; 40 avatars and the
// workspace icon, and 20 emoji, from the Slack CDN; 6 avatars from gravatar;
// and an OG image and a service icon from each of 12 third-party origins. The
// origins are the traced workload's, but for a slower link to
// files.slack.com, 10 MiB/s, over which the large downloads share the
// bandwidth. The sizes and the bandwidth are assumptions.
func heavyWorkload() Workload {
	t := tracedWorkload()
	w := Workload{Name: "heavy"}
	b := builder{&w}
	files := b.origin(t.Origins[0])
	w.Origins[files].BytesPerSec = 10 * mib
	avatars := b.origin(t.Origins[1])
	emoji := b.origin(t.Origins[2])
	gravatar := b.origin(t.Origins[3])
	for i := range 96 {
		b.add(files, output.KindUploadThumb, int64(20+(i*29)%57)*kib)
	}
	for i := range 24 {
		b.add(files, output.KindUploadOriginal, int64(512+(i*1013)%7680)*kib)
	}
	for i := range 8 {
		b.add(files, output.KindAttachment, int64(1024+(i*677)%5120)*kib)
	}
	for i := range 40 {
		b.add(avatars, output.KindAvatar, int64(4+i%5)*kib)
	}
	b.add(avatars, output.KindWorkspaceIcon, 6*kib)
	for i := range 20 {
		b.add(emoji, output.KindEmoji, int64(3+i%4)*kib)
	}
	b.add(gravatar, output.KindAvatar, 7*kib, 8*kib, 6*kib, 7*kib, 9*kib, 5*kib)
	for i := range 12 {
		id := b.origin(t.Origins[4+i])
		b.add(id, output.KindOGImage, ogImageSize(i))
		b.add(id, output.KindServiceIcon, serviceIconSize(i))
	}
	shuffle(w.Assets)
	return w
}

// recentWorkload is modeled on the export measured for #272 when it was filed
// (2026-09-27), before the trace: 56 assets from 25 origins. files.slack.com
// has 4, the Slack CDN 14 on 4 hosts (9 on one of them), gravatar 5, and 19
// third-party origins 33, with 1 to 3 each. The Slack hosts and gravatar spoke
// HTTP/2 with 128 streams and took 30 to 56 ms to connect. The sizes, and the
// latency, bandwidth and HTTP version of the third-party origins, are
// assumptions.
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

// limitedWorkload is 160 thumbnails (20 to 76 KiB) from a files.slack.com
// that keeps a rate limit (Issue #276): 8 requests a second, with a burst of
// 8, over which it answers 429 with Retry-After: 1. It is the traced
// workload's files.slack.com otherwise, whose 890 ms to the first byte lets
// a lane of 16 downloads send about 18 requests a second. Slack does not
// publish a rate limit of its file host: the limit and the Retry-After are
// assumptions, to weigh how a lane gives its places back after a 429
// (lane.Limits.RestoreAfter).
func limitedWorkload() Workload {
	w := thumbnailWorkload("limited")
	w.Origins[0].LimitPerSec, w.Origins[0].LimitBurst, w.Origins[0].RetryAfterS = 8, 8, 1
	return w
}

// spikeWorkload is the thumbnails of limitedWorkload from a files.slack.com
// that keeps no rate limit but for a passing one: it refuses the requests
// that come from 1.5 s to 3.5 s into the run with a 429 of Retry-After: 2. A
// lane of 16 downloads sends its third 16 then.
func spikeWorkload() Workload {
	w := thumbnailWorkload("spike")
	w.Origins[0].RefuseAfterMS, w.Origins[0].RefuseForMS, w.Origins[0].RetryAfterS = 1500, 2000, 2
	return w
}

// thumbnailWorkload is 160 thumbnails from the traced workload's
// files.slack.com.
func thumbnailWorkload(name string) Workload {
	w := Workload{Name: name}
	b := builder{&w}
	files := b.origin(tracedWorkload().Origins[0])
	for i := range 160 {
		b.add(files, output.KindUploadThumb, int64(20+(i*29)%57)*kib)
	}
	return w
}

// shuffle mixes the assets as an export meets them, the same way every time.
func shuffle(assets []Asset) {
	r := rand.New(rand.NewPCG(272, 273))
	r.Shuffle(len(assets), func(i, j int) { assets[i], assets[j] = assets[j], assets[i] })
}
