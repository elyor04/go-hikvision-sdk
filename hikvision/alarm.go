package hikvision

/*
#include "shim.h"
*/
import "C"

import (
	"context"
	"sync"
)

// Command mirrors a curated subset of HCNetSDK's COMM_* alarm/event command
// codes - the ones this package can decode into a typed payload (Plate).
// Any other command still arrives on Event with Raw populated - see
// HCNetSDK.h for the struct matching a given code and decode it yourself.
type Command int32

const (
	CommAlarm             Command = C.HIK_COMM_ALARM
	CommAlarmV30          Command = C.HIK_COMM_ALARM_V30
	CommUploadPlateResult Command = C.HIK_COMM_UPLOAD_PLATE_RESULT
	CommITSPlateResult    Command = C.HIK_COMM_ITS_PLATE_RESULT
	CommPlateResultV50    Command = C.HIK_COMM_PLATE_RESULT_V50
)

// ExceptionType mirrors HCNetSDK's EXCEPTION_* codes delivered by the
// process-wide exception/reconnect callback.
type ExceptionType uint32

const (
	ExceptionNetworkExchange ExceptionType = 0x8000
	ExceptionAlarm           ExceptionType = 0x8002
	ExceptionPreview         ExceptionType = 0x8003
	ExceptionReconnect       ExceptionType = 0x8005 // preview reconnected
	ExceptionAlarmReconnect  ExceptionType = 0x8006
	ExceptionPlayback        ExceptionType = 0x8010
	ExceptionRelogin         ExceptionType = 0x8040
)

// Alarmer identifies which device/login an Event came from - decoded from
// NET_DVR_ALARMER. Fields are only meaningful when their *Valid companion is
// true, matching HCNetSDK's own convention.
type Alarmer struct {
	UserIDValid   bool
	UserID        int32
	DeviceIPValid bool
	DeviceIP      string
	SerialValid   bool
	SerialNumber  string
}

// Event is one alarm/event message delivered by HCNetSDK's message callback.
type Event struct {
	Command Command
	Alarmer Alarmer
	// Raw is the undecoded payload for Command - only meaningful together
	// with the exact struct HCNetSDK.h documents for that command code. This
	// package decodes the common ANPR commands into Plate for you.
	Raw []byte
	// Plate is populated (non-nil) when Command is one of the ANPR plate
	// commands and the payload was successfully decoded. See anpr.go.
	Plate *PlateEvent
}

var (
	alarmMu       sync.Mutex
	alarmOnce     sync.Once
	alarmSubs     = map[int32]chan Event{}
	exceptionMu   sync.Mutex
	exceptionSubs []func(ExceptionType, int32, int32)
	exceptionOnce sync.Once
	exceptionErr  error
)

func dispatchAlarm(ev Event) {
	if !ev.Alarmer.UserIDValid {
		return
	}
	alarmMu.Lock()
	ch, ok := alarmSubs[ev.Alarmer.UserID]
	alarmMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}

func dispatchException(t ExceptionType, userID, handle int32) {
	exceptionMu.Lock()
	subs := append([]func(ExceptionType, int32, int32){}, exceptionSubs...)
	exceptionMu.Unlock()
	for _, f := range subs {
		f(t, userID, handle)
	}
}

// OnException registers a process-wide handler for HCNetSDK exception and
// reconnect notifications (network drops, reconnect success, etc). HCNetSDK
// only supports a single native callback per process; this package fans it
// out to every handler registered via OnException.
//
// The first call to OnException keeps the SDK initialized for the remaining
// lifetime of the process (mirroring HCNetSDK's own process-wide, permanent
// callback registration) rather than tying it to any particular Device.
func OnException(f func(t ExceptionType, userID, handle int32)) error {
	exceptionOnce.Do(func() {
		if exceptionErr = ensureInit(); exceptionErr != nil {
			return
		}
		if C.hik_set_exception_callback() != 0 {
			exceptionErr = lastError("SetExceptionCallBack")
		}
	})
	if exceptionErr != nil {
		return exceptionErr
	}

	exceptionMu.Lock()
	exceptionSubs = append(exceptionSubs, f)
	exceptionMu.Unlock()
	return nil
}

// alarmChanSession backs one NET_DVR_SetupAlarmChan_V41 handle.
type alarmChanSession struct {
	userID  int32
	alarmH  int32
	events  chan Event
	closeMu sync.Mutex
	closed  bool
}

func (s *alarmChanSession) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	alarmMu.Lock()
	delete(alarmSubs, s.userID)
	alarmMu.Unlock()

	var err error
	if C.hik_alarm_chan_close(C.int32_t(s.alarmH)) != 0 {
		err = lastError("CloseAlarmChan")
	}
	close(s.events)
	return err
}

// Alarms opens an alarm subscription for this device (NET_DVR_SetupAlarmChan_V41)
// and returns a channel of decoded Events. The channel is closed when ctx is
// cancelled or the Device is closed. HCNetSDK's message callback is
// process-wide and registered lazily on first use across the whole package.
func (d *Device) Alarms(ctx context.Context) (<-chan Event, error) {
	var setupErr error
	alarmOnce.Do(func() {
		setupErr = C_setAlarmCallback()
	})
	if setupErr != nil {
		return nil, setupErr
	}

	alarmH := int32(C.hik_alarm_chan_open(C.int32_t(d.userID)))
	if alarmH < 0 {
		return nil, lastError("SetupAlarmChan")
	}

	sess := &alarmChanSession{userID: d.userID, alarmH: alarmH, events: make(chan Event, 32)}
	alarmMu.Lock()
	alarmSubs[d.userID] = sess.events
	alarmMu.Unlock()

	d.track(sess)
	if ctx != nil {
		go func() {
			<-ctx.Done()
			d.untrack(sess)
			_ = sess.Close()
		}()
	}
	return sess.events, nil
}

// C_setAlarmCallback registers HCNetSDK's single process-wide message
// callback. Split out from Alarms so alarmOnce.Do has a plain func() error.
func C_setAlarmCallback() error {
	if C.hik_set_alarm_callback() != 0 {
		return lastError("SetDVRMessageCallBack")
	}
	return nil
}
