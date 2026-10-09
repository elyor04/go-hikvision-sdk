package hikvision

import (
	"errors"
	"runtime/cgo"
	"sync"
	"testing"
	"time"
)

// TestSessionCloseDoesNotHoldLockAcrossStop is a regression test for Close holding closeMu
// across the SDK stop call. NET_DVR_StopRealPlay/StopPlayBack wait for a data callback already in
// progress, and the callback needs closeMu in deliver; the stand-in stop below does the same, so
// the old Close deadlocked here exactly as it did against a real NVR.
func TestSessionCloseDoesNotHoldLockAcrossStop(t *testing.T) {
	errDeadlock := errors.New("stop waited 2s for an in-progress callback")
	stopLike := func(deliver func()) error {
		done := make(chan struct{})
		go func() { deliver(); close(done) }()
		select {
		case <-done:
			return nil
		case <-time.After(2 * time.Second):
			return errDeadlock
		}
	}

	t.Run("preview", func(t *testing.T) {
		s := &previewSession{frames: make(chan Frame, 1), done: make(chan struct{})}
		s.handle = cgo.NewHandle(s)
		orig := stopRealPlay
		defer func() { stopRealPlay = orig }()
		stopRealPlay = func(int32) error {
			return stopLike(func() { s.deliver(StreamStdVideoData, []byte("x")) })
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if _, ok := <-s.frames; ok {
			t.Fatal("a frame delivered during the stop was queued after Close began")
		}
	})

	t.Run("playback", func(t *testing.T) {
		s := &playbackSession{frames: make(chan Frame, 1), done: make(chan struct{})}
		s.handle = cgo.NewHandle(s)
		orig := stopPlayBack
		defer func() { stopPlayBack = orig }()
		stopPlayBack = func(int32) error {
			return stopLike(func() { s.deliver(StreamStdVideoData, []byte("x")) })
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if _, ok := <-s.frames; ok {
			t.Fatal("a frame delivered during the stop was queued after Close began")
		}
	})
}

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
