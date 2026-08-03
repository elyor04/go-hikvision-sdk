package hikvision

import (
	"testing"
	"time"
)

// TestHikTimePreservesWallClock is a regression test for hikTime silently
// normalizing to UTC before extracting Y/M/D h:m:s - the input-side mirror
// of the CaptureTime decode bug fixed in timeFromHik. FindRecordings/
// Playback/Download document start/stop as the device's local time; hikTime
// must send the wall-clock reading of whatever Location the caller passed,
// unchanged, not silently reinterpret it as UTC first.
func TestHikTimePreservesWallClock(t *testing.T) {
	deviceLoc := time.FixedZone("UTC+5", 5*3600)
	in := time.Date(2026, 8, 3, 22, 4, 16, 0, deviceLoc)

	got := testHikTime(in)
	want := testHikTimeResult{Year: 2026, Month: 8, Day: 3, Hour: 22, Minute: 4, Second: 16}
	if got != want {
		t.Errorf("testHikTime(%v) = %+v, want %+v (wall-clock digits unchanged regardless of Location)", in, got, want)
	}
}

// TestHikTimeDifferentZonesSameWallClock confirms hikTime keys off the wall
// clock, not the absolute instant - two time.Time values naming the same
// instant but expressed in different zones must produce different hik_time
// digits (whatever the caller's Location says the local reading is), and
// two values with the same wall-clock digits in different zones must
// produce identical hik_time output.
func TestHikTimeDifferentZonesSameWallClock(t *testing.T) {
	utc5 := time.FixedZone("UTC+5", 5*3600)
	utc0 := time.UTC

	sameInstant := time.Date(2026, 8, 3, 22, 0, 0, 0, utc5) // == 17:00 UTC
	gotInstant := testHikTime(sameInstant)
	if gotInstant.Hour != 22 {
		t.Errorf("hour = %d, want 22 (device wall clock, not the UTC-shifted instant)", gotInstant.Hour)
	}

	sameWallClock := time.Date(2026, 8, 3, 22, 0, 0, 0, utc0) // a different instant, same digits
	gotWallClock := testHikTime(sameWallClock)
	if gotWallClock != gotInstant {
		t.Errorf("testHikTime(%v) = %+v, testHikTime(%v) = %+v, want equal (same wall-clock digits)",
			sameInstant, gotInstant, sameWallClock, gotWallClock)
	}
}
