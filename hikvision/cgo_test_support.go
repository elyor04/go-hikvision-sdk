package hikvision

/*
#include "shim.h"
*/
import "C"

import "unsafe"

// This file exists solely to give the test suite (errors_test.go, util_test.go,
// anpr_test.go, ...) a way to exercise cgo-boundary code (cString,
// plateEventFromC, ...) without themselves using `import "C"` - Go does not
// allow cgo preambles in _test.go files, even for the package under test. The
// helpers here are unexported and have no effect on the package's public API.

func setCCharBuf(buf []C.char, s string) {
	for i := range buf {
		buf[i] = 0
	}
	for i := 0; i < len(s) && i < len(buf)-1; i++ {
		buf[i] = C.char(s[i])
	}
}

// testCString round-trips s through a fixed-size C char buffer of size
// bufLen and back through cString, exercising the exact code path used to
// decode fields like Alarmer.DeviceIP / PlateEvent.License.
func testCString(s string, bufLen int) string {
	buf := make([]C.char, bufLen)
	setCCharBuf(buf, s)
	return cString(&buf[0], len(buf))
}

// testCStringRaw builds a bufLen-byte C char buffer directly from raw (no NUL
// padding/truncation), to test cString's behavior when the buffer has no NUL
// terminator at all.
func testCStringRaw(raw []byte) string {
	buf := make([]C.char, len(raw))
	for i, b := range raw {
		buf[i] = C.char(b)
	}
	return cString(&buf[0], len(buf))
}

type testPlateEventInput struct {
	License              string
	Confidence           uint8
	PlateColor           uint8
	VehicleColor         uint8
	VehicleType          uint8
	SpeedKMH             uint16
	Year                 uint16
	Month, Day           uint8
	Hour, Minute, Second uint8
	SceneImage           []byte
	PlateImage           []byte
}

// testBuildPlateEvent constructs a C.hik_plate_event from in and decodes it
// via plateEventFromC, exercising the real decode path exactly as
// callbacks.go's goAlarmTrampoline would.
func testBuildPlateEvent(in testPlateEventInput) PlateEvent {
	var p C.hik_plate_event
	setCCharBuf(p.license[:], in.License)
	p.confidence = C.uint8_t(in.Confidence)
	p.plate_color = C.uint8_t(in.PlateColor)
	p.vehicle_color = C.uint8_t(in.VehicleColor)
	p.vehicle_type = C.uint8_t(in.VehicleType)
	p.speed_kmh = C.uint16_t(in.SpeedKMH)
	p.capture_time = C.hik_time{
		year: C.uint16_t(in.Year), month: C.uint8_t(in.Month), day: C.uint8_t(in.Day),
		hour: C.uint8_t(in.Hour), minute: C.uint8_t(in.Minute), second: C.uint8_t(in.Second),
	}
	if len(in.SceneImage) > 0 {
		p.scene_image = (*C.uint8_t)(unsafe.Pointer(&in.SceneImage[0]))
		p.scene_image_len = C.uint32_t(len(in.SceneImage))
	}
	if len(in.PlateImage) > 0 {
		p.plate_image = (*C.uint8_t)(unsafe.Pointer(&in.PlateImage[0]))
		p.plate_image_len = C.uint32_t(len(in.PlateImage))
	}
	return plateEventFromC(&p)
}
