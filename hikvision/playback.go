package hikvision

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"context"
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

func hikTime(t time.Time) C.hik_time {
	t = t.UTC()
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
func timeFromHik(h C.hik_time) time.Time {
	if h.year == 0 || h.month == 0 || h.day == 0 {
		return time.Time{}
	}
	return time.Date(int(h.year), time.Month(h.month), int(h.day), int(h.hour), int(h.minute), int(h.second), 0, time.UTC)
}

const (
	fileFinding    = 0
	fileSuccess    = 1000
	fileNoFind     = 1001
	fileIsFinding  = 1002
	fileNoMoreFile = 1003
)

// FindRecordings searches server-side recordings on channel between start
// and stop (server local time, per HCNetSDK convention). It blocks until the
// search completes or ctx is cancelled.
func (d *Device) FindRecordings(ctx context.Context, channel int32, start, stop time.Time) ([]Recording, error) {
	findH := int32(C.hik_find_file_open(C.int32_t(d.userID), C.int32_t(channel), hikTime(start), hikTime(stop)))
	if findH < 0 {
		return nil, lastError("FindFile")
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

		var data C.hik_find_data
		rc := int32(C.hik_find_file_next(C.int32_t(findH), &data))
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
			return out, lastError("FindNextFile")
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

func (s *playbackSession) deliver(t StreamDataType, data []byte) {
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
	var err error
	if C.hik_playback_stop(C.int32_t(s.playH)) != 0 {
		err = lastError("StopPlayBack")
	}
	s.handle.Delete()
	close(s.frames)
	return err
}

// Playback starts a server-side (remote) VOD session for channel between
// start and stop, streaming raw compressed frames just like RealPlay. Use
// Control to pause/resume/stop mid-stream.
func (d *Device) Playback(ctx context.Context, channel int32, start, stop time.Time) (*Stream, *PlaybackSession, error) {
	sess := &playbackSession{frames: make(chan Frame, 64)}
	sess.handle = cgo.NewHandle(sess)

	playH := int32(C.hik_playback_start_by_time(C.int32_t(d.userID), C.int32_t(channel), hikTime(start), hikTime(stop),
		C.uintptr_t(sess.handle)))
	if playH < 0 {
		sess.handle.Delete()
		return nil, nil, lastError("PlayBackByTime")
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
	if C.hik_playback_control(C.int32_t(p.playH), C.uint32_t(cmd)) != 0 {
		return lastError("PlayBackControl")
	}
	return nil
}

// Download starts a server-side download of channel's recording between
// start and stop into savedFileName (a local file path). It returns
// immediately; poll DownloadSession.Progress until it reports done.
func (d *Device) Download(channel int32, start, stop time.Time, savedFileName string) (*DownloadSession, error) {
	cPath := C.CString(savedFileName)
	defer C.free(unsafe.Pointer(cPath))

	fileH := int32(C.hik_download_start_by_time(C.int32_t(d.userID), C.int32_t(channel), hikTime(start), hikTime(stop), cPath))
	if fileH < 0 {
		return nil, lastError("GetFileByTime")
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
	if C.hik_download_stop(C.int32_t(s.fileH)) != 0 {
		return lastError("StopGetFile")
	}
	return nil
}
