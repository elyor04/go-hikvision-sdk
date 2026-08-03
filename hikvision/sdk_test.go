package hikvision

import (
	"sync"
	"testing"
	"time"
)

// TestConfigureEnsureInitConcurrent exercises Configure concurrently with
// the real ensureInit/releaseInit init/cleanup cycle. This is a regression
// test for two real bugs previously in this code path:
//
//  1. A data race: applyRuntimeConfig used to read the package-level
//     `configured` global directly. Configure's call into it was protected
//     by configuredMu, but ensureInit's call happened after it had already
//     released configuredMu, leaving that read unsynchronized against a
//     concurrent Configure() call writing the same global (caught by
//     -race). Fixed by threading the already-captured value through as a
//     plain parameter (wasConfigured) instead.
//  2. An AB-BA deadlock: Configure locked configuredMu then initMu;
//     ensureInit locked initMu then configuredMu. Concurrent calls could
//     each grab their first lock and then permanently block on the other's
//     - this test reliably hung forever on the unfixed code. Fixed by
//     merging both into a single sdkMu, which makes lock-order inversion
//     between them structurally impossible.
//
// ensureInit/releaseInit each pay for a real NET_DVR_Init/Cleanup cycle
// (expensive, especially under -race instrumentation), and Configure's own
// applyRuntimeConfig call makes two more real SDK calls once alreadyInit -
// so both loops are kept small/throttled rather than busy-spinning.
func TestConfigureEnsureInitConcurrent(t *testing.T) {
	if defaultComponentDir() == "" {
		t.Skip("no default SDK component dir for this GOOS")
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
			}
			_ = Configure(Config{AutoReconnect: i%2 == 0})
		}
	}()

	const iterations = 5
	for i := 0; i < iterations; i++ {
		if err := ensureInit(); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("ensureInit: %v", err)
		}
		releaseInit()
	}
	close(stop)
	wg.Wait()
}
