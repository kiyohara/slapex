package demo

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// TestScenariosRenderEndToEnd runs both bundled fixtures through the real
// export pipeline against an in-process fake server, the same path slapex
// --demo takes. It guards that the fixtures stay renderable and that no
// {{base}} placeholder leaks into the output.
func TestScenariosRenderEndToEnd(t *testing.T) {
	now := time.Now()
	for _, sc := range []*Scenario{ScenarioJA(now), ScenarioEN(now)} {
		t.Run(sc.Lang, func(t *testing.T) {
			srv := NewServer(sc)
			defer srv.Close()

			client := slack.New(FakeToken, slack.WithBaseURL(srv.APIBaseURL()), slack.WithSleeper(NoPacing))
			dir, err := export.Run(context.Background(), client, export.Options{
				ChannelKeyword: sc.ChannelName,
				OutputDir:      t.TempDir(),
				MaxPosts:       1000,
				Days:           30,
				MaxAttachBytes: 10 << 20,
				NoInteractive:  true,
				ToolVersion:    "test",
			}, ui.NewPrinter(io.Discard, false))
			if err != nil {
				t.Fatalf("export.Run: %v", err)
			}

			html, err := os.ReadFile(filepath.Join(dir, "index.html"))
			if err != nil {
				t.Fatalf("read index.html: %v", err)
			}
			if len(html) == 0 {
				t.Fatal("index.html is empty")
			}
			if bytes.Contains(html, []byte("{{base}}")) {
				t.Fatal("index.html still contains the {{base}} placeholder")
			}
			assets, err := os.ReadDir(filepath.Join(dir, "assets"))
			if err != nil {
				t.Fatalf("read assets dir: %v", err)
			}
			if len(assets) == 0 {
				t.Fatal("no assets were written")
			}
		})
	}
}

// TestAuthorized covers the fake server's token check, including the
// AllowAnyToken relaxation used only by demo recordings.
func TestAuthorized(t *testing.T) {
	strict := &fakeServer{}
	if !strict.authorized("Bearer " + FakeToken) {
		t.Fatal("strict server should accept the exact FakeToken")
	}
	if strict.authorized("Bearer other-token") {
		t.Fatal("strict server should reject a different token")
	}
	if strict.authorized("") {
		t.Fatal("strict server should reject a missing Authorization header")
	}

	any := &fakeServer{anyBearer: true}
	if !any.authorized("Bearer anything-goes") {
		t.Fatal("anyBearer server should accept any non-empty Bearer token")
	}
	if any.authorized("Bearer ") {
		t.Fatal("anyBearer server should reject an empty Bearer value")
	}
	if any.authorized("") {
		t.Fatal("anyBearer server should reject a missing Authorization header")
	}
}

// TestReplaceBaseURL asserts every {{base}} placeholder in a fixture is
// rewritten to the serving base URL, so no asset reference is left dangling.
func TestReplaceBaseURL(t *testing.T) {
	const base = "http://127.0.0.1:9999"
	sc := ScenarioJA(time.Now())
	sc.ReplaceBaseURL(base)

	if got := sc.TeamInfo.Icon.Image68; strings.Contains(got, "{{base}}") || !strings.HasPrefix(got, base) {
		t.Fatalf("team icon URL = %q, want it rewritten to %q", got, base)
	}
	for id, u := range sc.Users {
		if strings.Contains(u.Profile.Image72, "{{base}}") {
			t.Fatalf("user %s avatar still has the placeholder: %q", id, u.Profile.Image72)
		}
	}
	for name, raw := range sc.Emoji {
		if strings.Contains(raw, "{{base}}") {
			t.Fatalf("emoji %s URL still has the placeholder: %q", name, raw)
		}
	}
	for _, m := range sc.Messages {
		for _, f := range m.Files {
			if strings.Contains(f.URLPrivateDownload, "{{base}}") {
				t.Fatalf("file %q download URL still has the placeholder", f.Name)
			}
		}
	}
}
