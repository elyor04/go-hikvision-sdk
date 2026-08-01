package hikvision

import "testing"

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
