package demo

import (
	"bytes"
	"fmt"
	"strings"
)

// All sample images are generated as small SVG documents: they stay crisp in
// screenshots, keep the committed sample light, and never embed real
// photographs or identifiable material.

const svgFont = "'Hiragino Sans','Noto Sans JP','Helvetica Neue',Arial,sans-serif"

func svgAsset(body string) Asset {
	return Asset{ContentType: "image/svg+xml", Body: []byte(body)}
}

// avatarSVG is a rounded square with a single initial, the classic default
// avatar look. All SVG roots carry explicit width/height so they keep their
// intrinsic size when referenced from <img>.
func avatarSVG(initial, bg string) Asset {
	return svgAsset(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96" viewBox="0 0 96 96">`+
		`<rect width="96" height="96" rx="22" fill="%s"/>`+
		`<text x="48" y="52" font-family="%s" font-size="44" font-weight="700" fill="#ffffff" text-anchor="middle" dominant-baseline="central">%s</text>`+
		`</svg>`, bg, svgFont, initial))
}

// workspaceIconSVG is a rounded square with a white star on a two-stop
// gradient.
func workspaceIconSVG(from, to string) Asset {
	return svgAsset(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96" viewBox="0 0 96 96">`+
		`<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`+
		`<rect width="96" height="96" rx="22" fill="url(#g)"/>`+
		`<polygon points="48,20 56,40 78,40 60,53 66,75 48,62 30,75 36,53 18,40 40,40" fill="#ffffff"/>`+
		`</svg>`, from, to))
}

// badgeEmojiSVG is a custom-emoji style rounded badge with short text.
func badgeEmojiSVG(lines []string, bg string, fontSize int) Asset {
	var texts strings.Builder
	step := 64 / (len(lines) + 1)
	for i, line := range lines {
		fmt.Fprintf(&texts,
			`<text x="32" y="%d" font-family="%s" font-size="%d" font-weight="800" fill="#ffffff" text-anchor="middle" dominant-baseline="central">%s</text>`,
			step*(i+1), svgFont, fontSize, line)
	}
	return svgAsset(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">`+
		`<rect width="64" height="64" rx="14" fill="%s"/>%s</svg>`, bg, texts.String()))
}

// cometEmojiSVG is a custom-emoji style comet mark.
func cometEmojiSVG() Asset {
	return svgAsset(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">` +
		`<rect width="64" height="64" rx="14" fill="#1d2b53"/>` +
		`<path d="M8 50 L40 22 L46 32 Z" fill="#ffd43b" opacity="0.7"/>` +
		`<circle cx="44" cy="24" r="10" fill="#ffe066"/>` +
		`</svg>`)
}

// packageArtSVG is the "uploaded design draft" image: night-sky gradient,
// shooting star and product name.
func packageArtSVG(title, subtitle string) Asset {
	return svgAsset(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="800" height="520" viewBox="0 0 800 520">`+
		`<defs><linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#1a1f4b"/><stop offset="1" stop-color="#4c3a8f"/></linearGradient></defs>`+
		`<rect width="800" height="520" fill="url(#sky)"/>`+
		`<circle cx="140" cy="110" r="3" fill="#ffffff"/><circle cx="240" cy="70" r="2" fill="#ffffff"/><circle cx="620" cy="90" r="3" fill="#ffffff"/><circle cx="700" cy="180" r="2" fill="#ffffff"/><circle cx="90" cy="230" r="2" fill="#ffffff"/><circle cx="530" cy="60" r="2" fill="#ffffff"/>`+
		`<path d="M120 300 L560 120 L575 150 Z" fill="#ffd43b" opacity="0.55"/>`+
		`<circle cx="580" cy="130" r="26" fill="#ffe066"/>`+
		`<text x="400" y="380" font-family="%s" font-size="64" font-weight="800" fill="#ffffff" text-anchor="middle">%s</text>`+
		`<text x="400" y="440" font-family="%s" font-size="28" fill="#c5b8ff" text-anchor="middle">%s</text>`+
		`</svg>`, svgFont, title, svgFont, subtitle))
}

// chartSVG is the "uploaded results chart" image: a simple titled bar chart.
func chartSVG(title string, labels []string, values []int, barColor string) Asset {
	const (
		width   = 720
		height  = 420
		baseY   = 350
		chartX  = 80
		chartW  = 600
		maxBarH = 240
	)
	maxVal := 1
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	var bars strings.Builder
	slot := chartW / len(values)
	for i, v := range values {
		barH := maxBarH * v / maxVal
		x := chartX + slot*i + slot/4
		fmt.Fprintf(&bars, `<rect x="%d" y="%d" width="%d" height="%d" rx="6" fill="%s"/>`,
			x, baseY-barH, slot/2, barH, barColor)
		fmt.Fprintf(&bars, `<text x="%d" y="%d" font-family="%s" font-size="20" fill="#495057" text-anchor="middle">%s</text>`,
			x+slot/4, baseY+30, svgFont, labels[i])
		fmt.Fprintf(&bars, `<text x="%d" y="%d" font-family="%s" font-size="20" font-weight="700" fill="#212529" text-anchor="middle">%d</text>`,
			x+slot/4, baseY-barH-12, svgFont, v)
	}
	return svgAsset(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+
		`<rect width="%d" height="%d" fill="#ffffff"/>`+
		`<text x="%d" y="56" font-family="%s" font-size="28" font-weight="700" fill="#212529">%s</text>`+
		`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#adb5bd" stroke-width="2"/>%s</svg>`,
		width, height, width, height, width, height, chartX, svgFont, title,
		chartX, baseY, chartX+chartW, baseY, bars.String()))
}

// ogImageSVG is the unfurl preview image: gradient banner with a headline.
func ogImageSVG(kicker, headline, from, to string) Asset {
	return svgAsset(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="800" height="420" viewBox="0 0 800 420">`+
		`<defs><linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`+
		`<rect width="800" height="420" fill="url(#bg)"/>`+
		`<text x="60" y="140" font-family="%s" font-size="26" font-weight="700" fill="#ffffffb0" letter-spacing="4">%s</text>`+
		`<text x="60" y="215" font-family="%s" font-size="34" font-weight="800" fill="#ffffff">%s</text>`+
		`<rect x="60" y="270" width="120" height="8" rx="4" fill="#ffd43b"/>`+
		`</svg>`, from, to, svgFont, kicker, svgFont, headline))
}

// serviceIconSVG is the small unfurl service icon.
func serviceIconSVG(letter, bg string) Asset {
	return svgAsset(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48">`+
		`<rect width="48" height="48" rx="10" fill="%s"/>`+
		`<text x="24" y="26" font-family="%s" font-size="26" font-weight="800" fill="#ffffff" text-anchor="middle" dominant-baseline="central">%s</text>`+
		`</svg>`, bg, svgFont, letter))
}

// samplePDF builds a minimal one-page PDF with the given ASCII lines, so the
// attachment link in the sample opens as a real document.
func samplePDF(lines []string) Asset {
	var content strings.Builder
	content.WriteString("BT /F1 18 Tf 72 720 Td 28 TL\n")
	for i, line := range lines {
		if i > 0 {
			content.WriteString("T*\n")
		}
		fmt.Fprintf(&content, "(%s) Tj\n", pdfEscape(line))
	}
	content.WriteString("ET")

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return Asset{ContentType: "application/pdf", Body: buf.Bytes()}
}

func pdfEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}
