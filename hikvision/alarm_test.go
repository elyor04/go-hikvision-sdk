package hikvision

import (
	"sync"
	"testing"
	"time"
)

// TestDispatchAlarmCloseNoSendOnClosedChannel is a regression/stress test
// for a real race between dispatchAlarm and a session's Close(): dispatchAlarm
// looks up the subscriber channel under alarmMu, then (after releasing the
// lock) sends to it; Close() deletes the map entry and closes that same
// channel. Nothing previously serialized "a lookup that already happened"
// against "the channel getting closed", so a dispatchAlarm call could still
// be mid-send after Close() closed the channel - an unconditional "send on
// closed channel" panic (not merely a data race) once that ordering occurs,
// which crashes the whole process since a panic on a non-test goroutine
// cannot be recovered from outside it. Mirrors exactly what
// alarmChanSession.Close does at the Go-bookkeeping level (the C
// hik_alarm_chan_close call itself is orthogonal to this race and requires
// a live session, so it's intentionally not exercised here).
func TestDispatchAlarmCloseNoSendOnClosedChannel(t *testing.T) {
	const uid = int32(-424242) // synthetic - never a real device userID

	stop := make(chan struct{})
	time.AfterFunc(200*time.Millisecond, func() { close(stop) })

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
				dispatchAlarm(Event{Alarmer: Alarmer{UserIDValid: true, UserID: uid}})
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
			ch := make(chan Event, 1)
			alarmMu.Lock()
			alarmSubs[uid] = ch
			alarmMu.Unlock()

			time.Sleep(time.Microsecond)

			alarmMu.Lock()
			delete(alarmSubs, uid)
			close(ch)
			alarmMu.Unlock()
		}
	}()

	wg.Wait()
}
