// Command genscreenshot regenerates the README sample screenshots under
// assets/screenshots/ from the committed sample exports in doc/samples/
// (Issue #132). It renders a temporary copy of doc/samples/<lang>/index.html
// with headless Chromium (scrollbars hidden), measures the crop boundaries
// inside the browser instead of hardcoding pixel offsets, crops and scales
// the capture down to the README width, and rejects the result if a border
// artifact — such as the solid dark 1px column that used to sit on the right
// edge of the timeline screenshots — shows up. Run via:
//
//	docker compose run --rm screenshot
//
// which builds tools/genscreenshot/Dockerfile (Go toolchain + Chromium +
// Noto CJK / emoji fonts) and executes this tool inside the container, so no
// host browser or macOS-only image tooling is involved.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// viewportWidth is the CSS viewport width used for every capture. The
	// sample stylesheet centers a max-width 920px body in it, leaving equal
	// white margins on both sides, and the committed screenshots have always
	// been taken at this width.
	viewportWidth = 1100
	// deviceScale doubles the capture resolution; the capture is then scaled
	// down to outputWidth so the committed PNGs stay crisp on the README.
	deviceScale = 2
	// outputWidth is the final width of the committed screenshots.
	outputWidth = 1600
	// timelineMaxHeight caps the timeline crop (CSS px) when the sample has
	// no second date divider to anchor the crop to.
	timelineMaxHeight = 880
	// cropPad is the breathing room (CSS px) added around measured crop
	// boundaries, shrunk when the neighboring block is closer than that.
	cropPad = 12
	// edgeMinLuminance is the minimum luminance every border pixel of a
	// finished screenshot must have. The sample pages paint a white canvas
	// all the way to the viewport edges, so anything darker on a border is a
	// capture artifact like the former (35, 35, 35) column.
	edgeMinLuminance = 180
)

// measureScript is injected before </body> of a temporary page copy. After
// the load event (and again once fonts are ready) it publishes the page
// height and the block geometry needed to pick crop boundaries through the
// document title, which `chromium --dump-dom` then serializes.
const measureScript = `<script>
(function () {
  function box(el) {
    var r = el.getBoundingClientRect();
    return { top: r.top + window.scrollY, bottom: r.bottom + window.scrollY };
  }
  function report() {
    var out = { page: document.documentElement.scrollHeight, blocks: [], threadParentTop: -1 };
    var tl = document.querySelector("main.timeline");
    if (tl) {
      for (var i = 0; i < tl.children.length; i++) {
        var b = box(tl.children[i]);
        out.blocks.push({
          class: tl.children[i].className,
          top: b.top,
          bottom: b.bottom,
          hasFile: !!tl.children[i].querySelector(".file-block")
        });
      }
    }
    var tg = document.querySelector("details.thread-group");
    if (tg) {
      out.thread = box(tg);
      if (tg.previousElementSibling) {
        out.threadParentTop = box(tg.previousElementSibling).top;
      }
    }
    document.title = "GENSCREENSHOT:" + JSON.stringify(out) + ":END";
  }
  window.addEventListener("load", function () {
    report();
    if (document.fonts && document.fonts.ready && document.fonts.ready.then) {
      document.fonts.ready.then(report, report);
    }
  });
})();
</script>`

var metricsPattern = regexp.MustCompile(`GENSCREENSHOT:(\{.*?\}):END`)

type span struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
}

type block struct {
	Class   string `json:"class"`
	HasFile bool   `json:"hasFile"`
	span
}

type pageMetrics struct {
	Page            float64 `json:"page"`
	Blocks          []block `json:"blocks"`
	Thread          *span   `json:"thread"`
	ThreadParentTop float64 `json:"threadParentTop"`
}

func main() {
	samples := flag.String("samples", "doc/samples", "directory holding the committed sample exports (<lang>/index.html)")
	out := flag.String("out", "assets/screenshots", "directory that receives the regenerated sample-*.png files")
	chromeBin := flag.String("chrome", defaultChrome(), "headless Chromium / Chrome binary (defaults to $CHROME_BIN, then \"chromium\")")
	flag.Parse()
	for _, lang := range []string{"ja", "en"} {
		if err := build(lang, *samples, *out, *chromeBin); err != nil {
			fmt.Fprintf(os.Stderr, "genscreenshot: %s: %v\n", lang, err)
			os.Exit(1)
		}
	}
}

func defaultChrome() string {
	if v := os.Getenv("CHROME_BIN"); v != "" {
		return v
	}
	return "chromium"
}

// build regenerates sample-timeline-<lang>.png and sample-thread-<lang>.png
// from samples/<lang>/. It stages a temporary copy of the sample so the
// committed export is never touched: the timeline page is captured as-is,
// the thread page with <details class="thread-group"> forced open.
func build(lang, samples, out, chromeBin string) error {
	tmp, err := os.MkdirTemp("", "genscreenshot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	stage := filepath.Join(tmp, lang)
	if err := copyDir(filepath.Join(samples, lang), stage); err != nil {
		return err
	}
	index, err := os.ReadFile(filepath.Join(stage, "index.html"))
	if err != nil {
		return err
	}
	pages := []struct {
		file    string
		html    string
		outName string
	}{
		{"timeline.html", injectScript(string(index)), fmt.Sprintf("sample-timeline-%s.png", lang)},
		{"thread.html", injectScript(openThreads(string(index))), fmt.Sprintf("sample-thread-%s.png", lang)},
	}
	if !strings.Contains(pages[1].html, `<details class="thread-group" open>`) {
		return fmt.Errorf("index.html has no <details class=\"thread-group\"> to open for the thread screenshot")
	}

	for i, page := range pages {
		path := filepath.Join(stage, page.file)
		if err := os.WriteFile(path, []byte(page.html), 0o644); err != nil {
			return err
		}
		url := "file://" + filepath.ToSlash(path)
		m, err := measure(chromeBin, url)
		if err != nil {
			return fmt.Errorf("%s: %w", page.file, err)
		}
		var crop span
		if i == 0 {
			bottom, err := timelineCropBottom(m)
			if err != nil {
				return fmt.Errorf("%s: %w", page.file, err)
			}
			crop = span{Top: 0, Bottom: bottom}
		} else {
			crop, err = threadCrop(m)
			if err != nil {
				return fmt.Errorf("%s: %w", page.file, err)
			}
		}
		shot := filepath.Join(tmp, page.file+".png")
		if err := capture(chromeBin, url, m.Page, shot); err != nil {
			return fmt.Errorf("%s: %w", page.file, err)
		}
		img, err := finish(shot, crop)
		if err != nil {
			return fmt.Errorf("%s: %w", page.file, err)
		}
		dst := filepath.Join(out, page.outName)
		if err := writePNG(dst, img); err != nil {
			return err
		}
		b := img.Bounds()
		fmt.Printf("%s: wrote %s (%dx%d, crop y=%.0f..%.0f of %.0f CSS px, border check ok)\n",
			lang, dst, b.Dx(), b.Dy(), crop.Top, crop.Bottom, m.Page)
	}
	return nil
}

// injectScript adds the measurement script to an export page.
func injectScript(html string) string {
	return strings.Replace(html, "</body>", measureScript+"\n</body>", 1)
}

// openThreads forces every collapsed thread open, matching how the thread
// screenshot has always been taken. The exact-match replacement leaves the
// footer's <details class="export-meta"> untouched.
func openThreads(html string) string {
	return strings.ReplaceAll(html, `<details class="thread-group">`, `<details class="thread-group" open>`)
}

// timelineCropBottom returns the crop bottom (CSS px) for the timeline
// screenshot: everything from the top of the page down to the last block of
// the first day, anchored on the second date divider so the crop follows the
// content instead of a hardcoded pixel offset. Without a second divider it
// falls back to the last block that fits within timelineMaxHeight.
func timelineCropBottom(m pageMetrics) (float64, error) {
	dividers := 0
	for i, b := range m.Blocks {
		if !strings.Contains(b.Class, "date-divider") {
			continue
		}
		dividers++
		if dividers == 2 && i > 0 {
			prev := m.Blocks[i-1]
			return prev.Bottom + pad(prev.Bottom, b.Top), nil
		}
	}
	for i := len(m.Blocks) - 1; i >= 0; i-- {
		if m.Blocks[i].Bottom > timelineMaxHeight {
			continue
		}
		bottom := m.Blocks[i].Bottom
		next := m.Page
		if i+1 < len(m.Blocks) {
			next = m.Blocks[i+1].Top
		}
		return bottom + pad(bottom, next), nil
	}
	return 0, fmt.Errorf("no timeline blocks measured (page %.0f CSS px)", m.Page)
}

// threadCrop returns the crop (CSS px) for the thread screenshot: from the
// thread parent message through the opened thread group and, when present,
// on to the first following message with a file attachment. That keeps the
// crop aligned with what the README caption promises for this screenshot
// (thread, code block, bot post, file attachment) while staying anchored on
// content instead of pixel offsets. The breathing room on both sides is
// clamped against the neighboring timeline blocks so the crop never slices
// into their content.
func threadCrop(m pageMetrics) (span, error) {
	if m.Thread == nil || m.ThreadParentTop < 0 {
		return span{}, fmt.Errorf("no thread-group (or no parent message) measured")
	}
	end := m.Thread.Bottom
	for _, b := range m.Blocks {
		if b.Top >= m.Thread.Bottom-0.5 && b.HasFile {
			end = b.Bottom
			break
		}
	}
	prevBottom, nextTop := 0.0, m.Page
	for _, b := range m.Blocks {
		if b.Bottom <= m.ThreadParentTop+0.5 && b.Bottom > prevBottom {
			prevBottom = b.Bottom
		}
		if b.Top >= end-0.5 && b.Top < nextTop {
			nextTop = b.Top
		}
	}
	top := math.Max(0, m.ThreadParentTop-pad(prevBottom, m.ThreadParentTop))
	bottom := math.Min(m.Page, end+pad(end, nextTop))
	return span{Top: top, Bottom: bottom}, nil
}

// pad returns the breathing room to add below a crop boundary at bottom,
// staying clear of the next block starting at next.
func pad(bottom, next float64) float64 {
	p := math.Min(cropPad, (next-bottom)/2)
	return math.Max(0, p)
}

// chromeArgs are the flags shared by the measurement and capture runs.
// --hide-scrollbars keeps the scrollbar out of the viewport so it cannot
// bleed into the right edge of the crop, and --virtual-time-budget lets the
// page finish loading fonts and images before Chromium acts.
func chromeArgs(extra ...string) []string {
	args := []string{
		"--headless",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--hide-scrollbars",
		fmt.Sprintf("--force-device-scale-factor=%d", deviceScale),
		"--virtual-time-budget=15000",
	}
	return append(args, extra...)
}

// measure loads url headlessly and extracts the metrics the injected script
// published through the document title.
func measure(chromeBin, url string) (pageMetrics, error) {
	var m pageMetrics
	out, err := runChrome(chromeBin, chromeArgs(
		fmt.Sprintf("--window-size=%d,900", viewportWidth),
		"--dump-dom",
		url,
	))
	if err != nil {
		return m, err
	}
	match := metricsPattern.FindSubmatch(out)
	if match == nil {
		return m, fmt.Errorf("measurement script output not found in dumped DOM")
	}
	if err := json.Unmarshal(match[1], &m); err != nil {
		return m, fmt.Errorf("parse measurements: %w", err)
	}
	if m.Page <= 0 {
		return m, fmt.Errorf("measured page height %.0f", m.Page)
	}
	return m, nil
}

// capture screenshots the full page at deviceScale into out by sizing the
// window to the measured page height.
func capture(chromeBin, url string, pageHeight float64, out string) error {
	height := int(math.Ceil(pageHeight))
	if height < 900 {
		height = 900
	}
	_, err := runChrome(chromeBin, chromeArgs(
		fmt.Sprintf("--window-size=%d,%d", viewportWidth, height),
		"--screenshot="+out,
		url,
	))
	return err
}

func runChrome(chromeBin string, args []string) ([]byte, error) {
	cmd := exec.Command(chromeBin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w\n%s", chromeBin, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// finish crops the capture to the measured CSS span, scales it down to
// outputWidth and validates its borders.
func finish(shot string, crop span) (*image.RGBA, error) {
	src, err := readPNG(shot)
	if err != nil {
		return nil, err
	}
	rect := image.Rect(
		0, int(math.Round(crop.Top*deviceScale)),
		viewportWidth*deviceScale, int(math.Round(crop.Bottom*deviceScale)),
	).Intersect(src.Bounds())
	if rect.Empty() {
		return nil, fmt.Errorf("crop %v outside capture %v", crop, src.Bounds())
	}
	cropped := src.SubImage(rect).(*image.RGBA)
	height := int(math.Round(float64(rect.Dy()) * outputWidth / float64(rect.Dx())))
	img := scaleDown(cropped, outputWidth, height)
	if err := checkBorders(img); err != nil {
		return nil, err
	}
	return img, nil
}

// scaleDown shrinks src to w x h with an area-average (box) filter, which is
// exact for downscaling and keeps the tool free of external image deps.
func scaleDown(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	xr := float64(sb.Dx()) / float64(w)
	yr := float64(sb.Dy()) / float64(h)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for dy := 0; dy < h; dy++ {
		y0, y1 := float64(dy)*yr, float64(dy+1)*yr
		for dx := 0; dx < w; dx++ {
			x0, x1 := float64(dx)*xr, float64(dx+1)*xr
			var r, g, b, a, area float64
			for sy := int(y0); float64(sy) < y1 && sy < sb.Dy(); sy++ {
				wy := overlap(y0, y1, sy)
				row := src.PixOffset(sb.Min.X, sb.Min.Y+sy)
				for sx := int(x0); float64(sx) < x1 && sx < sb.Dx(); sx++ {
					wgt := overlap(x0, x1, sx) * wy
					o := row + sx*4
					r += float64(src.Pix[o]) * wgt
					g += float64(src.Pix[o+1]) * wgt
					b += float64(src.Pix[o+2]) * wgt
					a += float64(src.Pix[o+3]) * wgt
					area += wgt
				}
			}
			o := dst.PixOffset(dx, dy)
			dst.Pix[o] = round8(r / area)
			dst.Pix[o+1] = round8(g / area)
			dst.Pix[o+2] = round8(b / area)
			dst.Pix[o+3] = round8(a / area)
		}
	}
	return dst
}

// overlap returns how much of the unit source cell i lies inside [a0, a1).
func overlap(a0, a1 float64, i int) float64 {
	lo := math.Max(a0, float64(i))
	hi := math.Min(a1, float64(i+1))
	return math.Max(0, hi-lo)
}

func round8(v float64) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(v))))
}

// checkBorders fails when any border pixel of the finished screenshot is
// darker than edgeMinLuminance. The sample pages paint a white canvas up to
// the viewport edges and the crops end in whitespace, so a dark border pixel
// means a capture / crop artifact such as the solid (35, 35, 35) right-edge
// column this tool exists to prevent (Issue #132).
func checkBorders(img *image.RGBA) error {
	b := img.Bounds()
	edges := []struct {
		name           string
		x0, y0, x1, y1 int
	}{
		{"top", b.Min.X, b.Min.Y, b.Max.X - 1, b.Min.Y},
		{"bottom", b.Min.X, b.Max.Y - 1, b.Max.X - 1, b.Max.Y - 1},
		{"left", b.Min.X, b.Min.Y, b.Min.X, b.Max.Y - 1},
		{"right", b.Max.X - 1, b.Min.Y, b.Max.X - 1, b.Max.Y - 1},
	}
	for _, e := range edges {
		dark, worst := 0, 255.0
		for y := e.y0; y <= e.y1; y++ {
			for x := e.x0; x <= e.x1; x++ {
				o := img.PixOffset(x, y)
				lum := (299*float64(img.Pix[o]) + 587*float64(img.Pix[o+1]) + 114*float64(img.Pix[o+2])) / 1000
				if lum < edgeMinLuminance {
					dark++
					worst = math.Min(worst, lum)
				}
			}
		}
		if dark > 0 {
			return fmt.Errorf("%s border has %d pixel(s) darker than luminance %d (worst %.0f); capture artifact, refusing to write", e.name, dark, edgeMinLuminance, worst)
		}
	}
	return nil
}

func readPNG(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	return rgba, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// copyDir replaces dst with a copy of the src tree.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
