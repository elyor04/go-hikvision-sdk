package hikvision

/*
#include "shim.h"
*/
import "C"

import (
	"context"
	"fmt"
	"runtime/cgo"
	"sync"
	"sync/atomic"
)

// StreamDataType identifies the kind of buffer delivered on a Frame -
// mirrors HCNetSDK's NET_DVR_SYSHEAD/STREAMDATA/... constants.
type StreamDataType uint32

const (
	// StreamSysHead is the private-protocol system header that precedes a
	// stream (only present for the private/PS protocol, not standard
	// H.264/H.265 elementary streams).
	StreamSysHead StreamDataType = 1
	// StreamData is private-protocol muxed audio/video stream data.
	StreamData StreamDataType = 2
	// StreamAudioData is private-protocol audio-only stream data.
	StreamAudioData StreamDataType = 3
	// StreamStdVideoData is a standard (non-multiplexed) H.264/H.265/MJPEG
	// elementary video stream - what you want for feeding straight into
	// ffmpeg/gocv without any private-protocol demuxing.
	StreamStdVideoData StreamDataType = 4
	// StreamStdAudioData is a standard elementary audio stream.
	StreamStdAudioData StreamDataType = 5
)

// StreamType selects which of a channel's encoder streams to pull.
type StreamType uint32

const (
	MainStream  StreamType = 0
	SubStream   StreamType = 1
	ThirdStream StreamType = 2
)

// Frame is one buffer delivered by a real-time preview or playback session.
// Data is only valid until the next Frame is delivered - copy it if you need
// to retain it (RealPlay/Playback already copy out of the SDK's own buffer
// before it's queued here, so it's safe to hold onto after receipt).
type Frame struct {
	Type StreamDataType
	Data []byte
}

// previewSession backs one NET_DVR_RealPlay_V40 handle.
type previewSession struct {
	handle  cgo.Handle
	realH   int32
	frames  chan Frame
	closeMu sync.Mutex
	closed  bool
	// dropped is this session's own share of droppedFrames. The package-level counter is
	// process-wide, so an app running several cameras at once (the normal case) cannot tell from it
	// which stream lost data -- it would have to report every camera as damaged, or none.
	dropped atomic.Int64
	// done is closed exactly once, by Close, regardless of which caller
	// triggered it (Stream.Close, Device.Close, or the ctx-watcher goroutine
	// below) - it lets that watcher goroutine exit as soon as the session is
	// closed by any other path instead of leaking until ctx is eventually
	// cancelled (which, for a ctx that's never cancelled, e.g.
	// context.Background(), would otherwise be never).
	done chan struct{}
}

// previewChanBuffer is how many deliveries a preview session queues before deliver starts
// shedding them (see droppedFrames). It must comfortably exceed the largest BURST HCNetSDK can
// hand over between two consecutive receives by the consumer, which is set by the codec, not by
// the frame rate: the SDK delivers this stream in ~4KB pieces, and one 1080p H.264 keyframe off an
// iDS-TCM203-A SubStream is 130-440KB, i.e. roughly 30-100 pieces arriving back to back as fast as
// the socket can be read.
//
// It was 64 -- less than one keyframe. Measured on that value against two live cameras
// (2026-08-26): ~10 deliveries lost per camera per minute, with the consumer's own backlog
// reading 0 the whole time, so this was never the documented "consumer isn't keeping up" case at
// all. Draining into an unbounded queue on a goroutine that does nothing else did not help either;
// the SDK's callback thread simply fills the channel faster than the Go scheduler runs any
// receiver. Only capacity fixes it: the same test at 4096 ran with zero drops.
//
// Why that mattered so much more than "a dropped frame": these deliveries are PES-packet aligned,
// so losing one punches a hole in the middle of an access unit while leaving the container framing
// on either side of it intact. Nothing downstream can tell -- the demuxer never loses sync -- and
// the browser is handed a well-formed, internally truncated IDR, which makes its VideoDecoder
// raise "Decoding error" and close permanently. Two of every 46 keyframes were arriving that way.
//
// 1024 is ~10x the worst observed burst, and costs nothing when idle (a Go channel's buffer holds
// slice headers; the ~4KB payloads are only retained while actually queued, and steady-state depth
// is 0).
const previewChanBuffer = 1024

// droppedFrames counts deliveries discarded because a previewSession's
// Frames() channel was full. This is NOT only "the consumer is too slow" as
// this comment used to claim - it is far more often the channel being smaller
// than one keyframe's burst, which no consumer speed can compensate for (see
// previewChanBuffer).
//
// Treat any growth as data corruption, not as delay or dropped detail: a
// delivery is a fragment of a compressed frame, so losing one leaves a hole
// inside an access unit that downstream framing cannot detect. Callers should
// surface this, not just record it.
var droppedFrames atomic.Int64

// DroppedFrameCount reports the cumulative number of frames dropped by
// previewSession.deliver since process start (see droppedFrames).
func DroppedFrameCount() int64 { return droppedFrames.Load() }

// deliver and Close share closeMu so a frame delivered by the SDK's callback
// thread concurrently with Close can never be sent on s.frames after Close
// has closed it - closing a channel a concurrent, unsynchronized sender
// might still write to is an unconditional "send on closed channel" panic,
// not just a data race (see the identical, confirmed bug this mirrors in
// alarm.go's dispatchAlarm/alarmChanSession.Close).
func (s *previewSession) deliver(t StreamDataType, data []byte) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.frames <- Frame{Type: t, Data: data}:
	default:
		// The queue is full - drop rather than block the SDK's internal
		// delivery thread (which would eventually stall the whole
		// connection). See previewChanBuffer for why reaching this at all
		// silently corrupts the stream, not merely thins it.
		droppedFrames.Add(1)
		s.dropped.Add(1)
	}
}

func (s *previewSession) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	err := sdkCall0("StopRealPlay", func() C.int32_t {
		return C.hik_realplay_stop(C.int32_t(s.realH))
	})
	s.handle.Delete()
	close(s.frames)
	return err
}

// RealPlay starts a live stream on the given channel (1-based, per HCNetSDK
// convention) and stream type, and returns a channel of raw compressed video
// frames (StreamStdVideoData in the common case). The returned stream must be
// closed with Stop when done, or by cancelling ctx.
//
// This package deliberately does not decode video - buffers arrive exactly
// as HCNetSDK/the device produced them (typically H.264/H.265 elementary
// stream data); pipe them into ffmpeg, gocv, or any decoder of your choice.
func (d *Device) RealPlay(ctx context.Context, channel int32, stream StreamType) (*Stream, error) {
	if err := d.checkOpen("RealPlay"); err != nil {
		return nil, err
	}
	sess := &previewSession{frames: make(chan Frame, previewChanBuffer), done: make(chan struct{})}
	sess.handle = cgo.NewHandle(sess)

	realH, err := sdkCallHandle("RealPlay", func() C.int32_t {
		return C.hik_realplay_start(C.int32_t(d.userID), C.int32_t(channel), C.uint32_t(stream),
			C.uintptr_t(sess.handle))
	})
	if err != nil {
		sess.handle.Delete()
		return nil, err
	}
	sess.realH = realH

	d.track(sess)
	s := &Stream{sess: sess, device: d}
	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				_ = s.Close()
			case <-sess.done:
			}
		}()
	}
	return s, nil
}

// Stream is a live preview or playback session delivering Frames.
type Stream struct {
	sess   io_Closer
	device *Device
}

// Frames returns the channel Frame values are delivered on. It is closed
// when the stream stops (either explicitly via Close, or because the
// underlying HCNetSDK connection was torn down).
func (s *Stream) Frames() <-chan Frame {
	switch sess := s.sess.(type) {
	case *previewSession:
		return sess.frames
	case *playbackSession:
		return sess.frames
	default:
		panic(fmt.Sprintf("hikvision: unknown stream session type %T", sess))
	}
}

// DroppedCount reports how many deliveries THIS stream has lost because its queue was full - the
// per-session counterpart to the process-wide DroppedFrameCount. Treat any growth as corruption of
// this stream specifically; see droppedFrames and previewChanBuffer.
//
// Playback sessions do not track this and always report 0.
func (s *Stream) DroppedCount() int64 {
	if sess, ok := s.sess.(*previewSession); ok {
		return sess.dropped.Load()
	}
	return 0
}

// Close stops the stream and releases its HCNetSDK handle.
func (s *Stream) Close() error {
	if s.device != nil {
		s.device.untrack(s.sess)
	}
	return s.sess.Close()
}
