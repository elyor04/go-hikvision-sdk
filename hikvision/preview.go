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
	// done is closed exactly once, by Close, regardless of which caller
	// triggered it (Stream.Close, Device.Close, or the ctx-watcher goroutine
	// below) - it lets that watcher goroutine exit as soon as the session is
	// closed by any other path instead of leaking until ctx is eventually
	// cancelled (which, for a ctx that's never cancelled, e.g.
	// context.Background(), would otherwise be never).
	done chan struct{}
}

// droppedFrames counts frames discarded because a previewSession's Frames()
// channel was full - i.e. the consumer (the app's ffmpeg-feed goroutine)
// wasn't draining fast enough. A nonzero/growing count while a stream is
// active means something downstream is stalling and the SDK's realtime
// callback thread is shedding load rather than blocking, which shows up as
// visible artifacts/jumps in the remuxed output, not just delay.
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
		// Consumer isn't keeping up - drop the frame rather than blocking
		// the SDK's internal delivery thread (which would eventually stall
		// the whole connection).
		droppedFrames.Add(1)
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
	sess := &previewSession{frames: make(chan Frame, 64), done: make(chan struct{})}
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

// Close stops the stream and releases its HCNetSDK handle.
func (s *Stream) Close() error {
	if s.device != nil {
		s.device.untrack(s.sess)
	}
	return s.sess.Close()
}
