package hikvision

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"context"
	"fmt"
	"runtime"
	"runtime/cgo"
	"sync"
	"time"
	"unsafe"
)

// PlaybackControl mirrors HCNetSDK's NET_DVR_PLAYBACKCONTROL codes for
// NET_DVR_PlayBackControl_V40.
type PlaybackControl uint32

const (
	PlaybackStart PlaybackControl = 1
	PlaybackStop  PlaybackControl = 2
	PlaybackPause PlaybackControl = 3
)

// Recording describes one entry returned by FindRecordings.
type Recording struct {
	FileName  string
	Start     time.Time
	Stop      time.Time
	SizeBytes uint32
	Locked    bool
}

// hikTime extracts t's wall-clock components as-is, in whatever Location t
// is already in - it must NOT normalize to UTC first. FindRecordings,
// Playback, and Download all document their start/stop parameters as
// "server local time, per HCNetSDK convention": the SDK's find/playback/
// download commands take a bare Y/M/D h:m:s reading with no timezone, which
// it interprets as the device's own local time. This used to call t.UTC()
// before extracting components, which silently reinterprets whatever
// wall-clock time the caller intended as if it were UTC - e.g. a caller in
// UTC+5 passing "device-local 22:00" (time.Date(..., 22, 0, 0, 0,
// deviceLoc)) would have this convert it to 17:00 UTC first and send *that*
// as the raw digits, shifting the search/playback window by 5 hours. This
// is the input-side mirror of the CaptureTime decode bug fixed in
// timeFromHik below - same root cause (conflating "UTC" with "whatever
// zone this Time value happens to carry"), opposite direction.
func hikTime(t time.Time) C.hik_time {
	return C.hik_time{
		year:   C.uint16_t(t.Year()),
		month:  C.uint8_t(t.Month()),
		day:    C.uint8_t(t.Day()),
		hour:   C.uint8_t(t.Hour()),
		minute: C.uint8_t(t.Minute()),
		second: C.uint8_t(t.Second()),
	}
}

// timeFromHik converts a hik_time to time.Time, returning the Go zero Time
// when the device reports no timestamp at all (year 0) - e.g. an ANPR
// snapshot/event with no recognized vehicle in frame - rather than the
// nonsensical "-0001-11-30" that time.Date(0, 0, 0, ...) would otherwise
// produce from a zero month/day.
//
// h.year/month/day/hour/minute/second is the device's wall-clock reading,
// not UTC - labeling it time.UTC outright (as this used to do) silently lies
// about the instant it represents, and formatting that as RFC3339 stamps a
// "Z" onto a time that was never actually UTC, corrupting the value for
// anyone who parses it (e.g. the frontend converting it to browser-local
// time doubles the device's UTC offset on top of itself). When the device
// supplied a valid UTC offset (tz_valid != 0, from
// cTimeDifferenceH/cTimeDifferenceM/byISO8601), use it to build the correct
// absolute instant instead. Without one, fall back to the process's local
// zone - a device's unlabeled wall-clock reading is far more likely to match
// the viewer's local time (same site) than true UTC.
func timeFromHik(h C.hik_time) time.Time {
	if h.year == 0 || h.month == 0 || h.day == 0 {
		return time.Time{}
	}
	loc := time.Local
	if h.tz_valid != 0 {
		offsetSeconds := (int(h.tz_offset_hour)*60 + int(h.tz_offset_min)) * 60
		loc = time.FixedZone(fmt.Sprintf("UTC%+03d:%02d", h.tz_offset_hour, abs(int(h.tz_offset_min))), offsetSeconds)
	}
	return time.Date(int(h.year), time.Month(h.month), int(h.day), int(h.hour), int(h.minute), int(h.second), 0, loc)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

const (
	fileFinding    = 0
	fileSuccess    = 1000
	fileNoFind     = 1001
	fileIsFinding  = 1002
	fileNoMoreFile = 1003
)

// FindRecordings searches server-side recordings on channel between start
// and stop. start/stop are interpreted as the device's own local time, per
// HCNetSDK convention (see hikTime) - only their wall-clock components
// (Y/M/D h:m:s) are sent, in whatever Location the caller supplied, so pass
// a time.Time already expressed in the device's local zone (e.g. from a
// Recording/PlateEvent this package returned, or a literal wall-clock
// reading of that site's local time). It blocks until the search completes
// or ctx is cancelled.
func (d *Device) FindRecordings(ctx context.Context, channel int32, start, stop time.Time) ([]Recording, error) {
	findH, err := sdkCallHandle("FindFile", func() C.int32_t {
		return C.hik_find_file_open(C.int32_t(d.userID), C.int32_t(channel), hikTime(start), hikTime(stop))
	})
	if err != nil {
		return nil, err
	}
	defer C.hik_find_file_close(C.int32_t(findH))

	var out []Recording
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			default:
			}
		}

		// hik_find_file_next's status codes don't fit the plain
		// 0-on-success/negative-on-failure conventions sdkCall0/
		// sdkCallHandle assume, so this locks around the call + conditional
		// lastError manually - see the comment above lastError in errors.go.
		var data C.hik_find_data
		runtime.LockOSThread()
		rc := int32(C.hik_find_file_next(C.int32_t(findH), &data))
		var findErr error
		if rc != fileFinding && rc != fileIsFinding && rc != fileSuccess && rc != fileNoFind && rc != fileNoMoreFile {
			findErr = lastError("FindNextFile")
		}
		runtime.UnlockOSThread()

		switch rc {
		case fileIsFinding, fileFinding:
			time.Sleep(50 * time.Millisecond)
			continue
		case fileSuccess:
			out = append(out, Recording{
				FileName:  C.GoString(&data.file_name[0]),
				Start:     timeFromHik(data.start_time),
				Stop:      timeFromHik(data.stop_time),
				SizeBytes: uint32(data.file_size),
				Locked:    data.locked != 0,
			})
		case fileNoFind, fileNoMoreFile:
			return out, nil
		default:
			return out, findErr
		}
	}
}

// playbackSession backs one NET_DVR_PlayBackByTime_V40 handle.
type playbackSession struct {
	handle  cgo.Handle
	playH   int32
	frames  chan Frame
	closeMu sync.Mutex
	closed  bool
}

// deliver and Close share closeMu - see the identical comment on
// previewSession.deliver in preview.go for why (closing s.frames while a
// concurrent, unsynchronized deliver() might still send on it is an
// unconditional "send on closed channel" panic).
func (s *playbackSession) deliver(t StreamDataType, data []byte) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.frames <- Frame{Type: t, Data: data}:
	default:
	}
}

func (s *playbackSession) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	err := sdkCall0("StopPlayBack", func() C.int32_t {
		return C.hik_playback_stop(C.int32_t(s.playH))
	})
	s.handle.Delete()
	close(s.frames)
	return err
}

// Playback starts a server-side (remote) VOD session for channel between
// start and stop, streaming raw compressed frames just like RealPlay. start/
// stop are the device's local time - see FindRecordings/hikTime. Use Control
// to pause/resume/stop mid-stream.
func (d *Device) Playback(ctx context.Context, channel int32, start, stop time.Time) (*Stream, *PlaybackSession, error) {
	if err := d.checkOpen("Playback"); err != nil {
		return nil, nil, err
	}
	sess := &playbackSession{frames: make(chan Frame, 64)}
	sess.handle = cgo.NewHandle(sess)

	playH, err := sdkCallHandle("PlayBackByTime", func() C.int32_t {
		return C.hik_playback_start_by_time(C.int32_t(d.userID), C.int32_t(channel), hikTime(start), hikTime(stop),
			C.uintptr_t(sess.handle))
	})
	if err != nil {
		sess.handle.Delete()
		return nil, nil, err
	}
	sess.playH = playH

	d.track(sess)
	s := &Stream{sess: sess, device: d}
	if ctx != nil {
		go func() {
			<-ctx.Done()
			_ = s.Close()
		}()
	}
	return s, &PlaybackSession{playH: playH}, nil
}

// PlaybackSession exposes VCR-style control over an in-progress Playback.
type PlaybackSession struct {
	playH int32
}

// Control sends a playback control command (pause/resume/stop mid-stream).
func (p *PlaybackSession) Control(cmd PlaybackControl) error {
	return sdkCall0("PlayBackControl", func() C.int32_t {
		return C.hik_playback_control(C.int32_t(p.playH), C.uint32_t(cmd))
	})
}

// Download starts a server-side download of channel's recording between
// start and stop into savedFileName (a local file path). start/stop are the
// device's local time - see FindRecordings/hikTime. It returns immediately;
// poll DownloadSession.Progress until it reports done.
func (d *Device) Download(channel int32, start, stop time.Time, savedFileName string) (*DownloadSession, error) {
	cPath := C.CString(savedFileName)
	defer C.free(unsafe.Pointer(cPath))

	fileH, err := sdkCallHandle("GetFileByTime", func() C.int32_t {
		return C.hik_download_start_by_time(C.int32_t(d.userID), C.int32_t(channel), hikTime(start), hikTime(stop), cPath)
	})
	if err != nil {
		return nil, err
	}
	return &DownloadSession{fileH: fileH}, nil
}

// DownloadSession tracks an in-progress file download started with Download.
type DownloadSession struct {
	fileH int32
}

// Progress returns 0-100, or -1 once the download has finished (whether it
// succeeded or failed - check the error from Wait/Stop for the outcome).
func (s *DownloadSession) Progress() int32 {
	return int32(C.hik_download_get_progress(C.int32_t(s.fileH)))
}

// Wait polls Progress until the download completes (100% or an error/stop).
func (s *DownloadSession) Wait(ctx context.Context) error {
	for {
		p := s.Progress()
		if p >= 100 || p < 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Stop cancels an in-progress download and releases its handle.
func (s *DownloadSession) Stop() error {
	return sdkCall0("StopGetFile", func() C.int32_t {
		return C.hik_download_stop(C.int32_t(s.fileH))
	})
}
