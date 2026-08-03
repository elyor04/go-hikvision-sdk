package hikvision

import (
	"sync"
	"testing"
	"time"
)

// TestPreviewSessionDeliverCloseNoPanic is a regression/stress test for the
// same class of bug fixed in alarm.go's dispatchAlarm/Close: deliver() and
// Close() both touch s.frames, and previously only Close() took closeMu - a
// frame delivered by the SDK callback thread concurrently with Close()
// could still be sent on s.frames after Close() closed it (an unconditional
// "send on closed channel" panic, not just a data race). deliver() now also
// takes closeMu and checks s.closed, so Close() cannot close the channel
// while a delivery is in flight.
//
// This drives deliver() and the closeMu+closed+close(frames) sequence
// directly rather than the real Close() method, which also makes a cgo
// NET_DVR_StopRealPlay call requiring a live session - orthogonal to this
// race. playbackSession.deliver/Close share the identical shape and fix and
// are not separately tested here.
func TestPreviewSessionDeliverCloseNoPanic(t *testing.T) {
	stop := make(chan struct{})
	time.AfterFunc(200*time.Millisecond, func() { close(stop) })

	var sessMu sync.RWMutex // test-harness only: safely swaps which *previewSession is "current"
	sess := &previewSession{frames: make(chan Frame, 1)}

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sessMu.RLock()
				s := sess
				sessMu.RUnlock()
				s.deliver(StreamStdVideoData, []byte("x"))
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			time.Sleep(time.Microsecond)

			sessMu.Lock()
			old := sess
			sess = &previewSession{frames: make(chan Frame, 1)}
			sessMu.Unlock()

			old.closeMu.Lock()
			old.closed = true
			close(old.frames)
			old.closeMu.Unlock()
		}
	}()

	wg.Wait()
}
