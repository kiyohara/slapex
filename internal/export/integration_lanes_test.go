package export

// Integration cases for the parallel asset fetch (Issue #275, PF-03 of #272).
// The downloads run in lanes by origin (internal/lane), so the order they end
// in changes from run to run, and the export must not show it: a run whose
// downloads the fake server holds and delays at random writes the same files
// and the same log as a run that downloads one asset at a time, run after
// run. A run canceled while its downloads are under way (Ctrl-C) stops
// without leaving a temporary file behind.

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/lane"
)

// serialLanes fetch one asset at a time, as the export did before the lanes.
var serialLanes = lane.Limits{HTTP2: 1, HTTP1: 1, Total: 1, Large: 1}

func TestRunIntegrationParallelAssetsMatchSerial(t *testing.T) {
	t.Parallel()

	serialCtx := context.WithValue(context.Background(), assetLanesKey{}, serialLanes)
	serial, _, err := runExportScenarioContext(t, serialCtx, allAssetPathsScenario(), allAssetPathsOptions(t))
	if err != nil {
		t.Fatalf("serial Run() error = %v\nlogs:\n%s", err, strings.Join(serial.Logs, "\n"))
	}
	for seed := range uint64(5) {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			t.Parallel()

			sc := allAssetPathsScenario()
			sc.BeforeAsset = shuffleAssets(t, seed)
			parallel, _, err := runExportScenarioContext(t, context.Background(), sc, allAssetPathsOptions(t))
			if err != nil {
				t.Fatalf("Run() error = %v\nlogs:\n%s", err, strings.Join(parallel.Logs, "\n"))
			}
			assertSameExport(t, "the parallel run", serial, parallel)
		})
	}
}

// shuffleAssets holds the asset requests of a run so that its downloads end
// in an order of their own: the first request waits until a second one has
// come, which only a parallel fetch sends, and each request then waits a
// random time under 20ms, drawn from seed.
func shuffleAssets(t *testing.T, seed uint64) func(*http.Request) {
	var (
		mu       sync.Mutex
		rng      = rand.New(rand.NewPCG(seed, seed))
		requests int
		second   = make(chan struct{})
	)
	return func(*http.Request) {
		mu.Lock()
		requests++
		n := requests
		wait := time.Duration(rng.IntN(20)) * time.Millisecond
		mu.Unlock()
		switch n {
		case 1:
			select {
			case <-second:
			case <-time.After(10 * time.Second):
				t.Errorf("no second asset request came while the first was held: the downloads did not run in parallel")
			}
		case 2:
			close(second)
		}
		time.Sleep(wait)
	}
}

// TestRunIntegrationCanceledDuringAssets: a run canceled while its downloads
// are under way returns the cancellation, warns of no asset, writes no
// index.html, and leaves no temporary file in the output directory.
func TestRunIntegrationCanceledDuringAssets(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sc := allAssetPathsScenario()
	var once sync.Once
	sc.BeforeAsset = func(r *http.Request) {
		// The first download cancels the run, and no download is answered
		// before the client gives up on it.
		once.Do(cancel)
		<-r.Context().Done()
	}
	opts := allAssetPathsOptions(t)
	got, _, err := runExportScenarioContext(t, ctx, sc, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want the cancellation\nlogs:\n%s", err, strings.Join(got.Logs, "\n"))
	}
	for _, line := range got.Logs {
		if strings.HasPrefix(line, "WARN: asset ") {
			t.Errorf("warned of an asset after the cancel: %q", line)
		}
	}
	var left []string
	err = filepath.WalkDir(opts.OutputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name := d.Name(); name == "index.html" || strings.HasPrefix(name, "asset-") {
			left = append(left, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the output: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("left in the output = %q, want no index.html and no temporary file", left)
	}
}
