package hikvision

import (
	"testing"
	"time"
)

func TestPlateEventFromC(t *testing.T) {
	ev := testBuildPlateEvent(testPlateEventInput{
		License:      "ABC1234",
		Confidence:   92,
		PlateColor:   1,
		VehicleColor: 2,
		VehicleType:  3,
		SpeedKMH:     65,
		Year:         2026, Month: 3, Day: 15,
		Hour: 10, Minute: 30, Second: 5,
	})

	if ev.License != "ABC1234" {
		t.Errorf("License = %q, want %q", ev.License, "ABC1234")
	}
	if ev.Confidence != 92 {
		t.Errorf("Confidence = %d, want 92", ev.Confidence)
	}
	if ev.SpeedKMH != 65 {
		t.Errorf("SpeedKMH = %d, want 65", ev.SpeedKMH)
	}
	if ev.CaptureTime.Year() != 2026 || ev.CaptureTime.Month() != 3 || ev.CaptureTime.Day() != 15 {
		t.Errorf("CaptureTime = %v, want 2026-03-15", ev.CaptureTime)
	}
	if ev.SceneImage != nil {
		t.Errorf("expected nil SceneImage when scene_image pointer is NULL, got %d bytes", len(ev.SceneImage))
	}
}

// TestPlateEventFromCTimezone verifies that a device-reported UTC offset
// (cTimeDifferenceH/M via byISO8601) is honored so the resulting time.Time
// represents the correct absolute instant, rather than mislabeling the
// device's local wall-clock reading as UTC (the bug this guards against:
// a device in UTC+5 reporting 22:04:16 must not be read as 22:04:16Z, which
// would shift it 5 hours into the wrong day for any UTC/local consumer).
func TestPlateEventFromCTimezone(t *testing.T) {
	ev := testBuildPlateEvent(testPlateEventInput{
		License: "TZ0001",
		Year:    2026, Month: 8, Day: 3,
		Hour: 22, Minute: 4, Second: 16,
		TZValid: true, TZOffsetHour: 5, TZOffsetMin: 0,
	})

	wantUTC := time.Date(2026, 8, 3, 17, 4, 16, 0, time.UTC)
	if !ev.CaptureTime.Equal(wantUTC) {
		t.Errorf("CaptureTime = %v (UTC %v), want instant %v", ev.CaptureTime, ev.CaptureTime.UTC(), wantUTC)
	}
	if _, offset := ev.CaptureTime.Zone(); offset != 5*3600 {
		t.Errorf("zone offset = %d seconds, want %d", offset, 5*3600)
	}
}

// TestPlateEventFromCNoTimezone verifies that when the device does not
// report a UTC offset (byISO8601/byLocalOrUTC = 0), the wall-clock reading
// is preserved verbatim in the process's local zone rather than being
// mislabeled UTC.
func TestPlateEventFromCNoTimezone(t *testing.T) {
	ev := testBuildPlateEvent(testPlateEventInput{
		License: "TZ0002",
		Year:    2026, Month: 8, Day: 3,
		Hour: 22, Minute: 4, Second: 16,
	})

	if ev.CaptureTime.Location() != time.Local {
		t.Errorf("Location = %v, want time.Local", ev.CaptureTime.Location())
	}
	if ev.CaptureTime.Hour() != 22 || ev.CaptureTime.Minute() != 4 || ev.CaptureTime.Second() != 16 {
		t.Errorf("wall clock = %v, want 22:04:16 preserved verbatim", ev.CaptureTime)
	}
}

func TestPlateEventFromCWithImages(t *testing.T) {
	sceneData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // fake JPEG magic bytes
	plateData := []byte{0xAA, 0xBB}

	ev := testBuildPlateEvent(testPlateEventInput{
		License:    "XYZ999",
		SceneImage: sceneData,
		PlateImage: plateData,
	})

	if len(ev.SceneImage) != len(sceneData) {
		t.Fatalf("SceneImage len = %d, want %d", len(ev.SceneImage), len(sceneData))
	}
	for i := range sceneData {
		if ev.SceneImage[i] != sceneData[i] {
			t.Errorf("SceneImage[%d] = %x, want %x", i, ev.SceneImage[i], sceneData[i])
		}
	}
	if len(ev.PlateImage) != len(plateData) {
		t.Fatalf("PlateImage len = %d, want %d", len(ev.PlateImage), len(plateData))
	}
}
