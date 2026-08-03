package hikvision

/*
#include "shim.h"
*/
import "C"

import (
	"context"
	"time"
	"unsafe"
)

// PlateEvent is a decoded ANPR/LPR (license plate recognition) result,
// whether it arrived via a live alarm subscription (Device.Alarms /
// Device.PlateEvents, commands COMM_ITS_PLATE_RESULT / COMM_UPLOAD_PLATE_RESULT
// / COMM_PLATE_RESULT_V50) or a manual snapshot (Device.ManualSnap).
type PlateEvent struct {
	License      string // recognized plate text
	Confidence   uint8  // 0-100
	PlateColor   uint8  // VCA_PLATE_COLOR
	VehicleColor uint8  // VCR_CLR_CLASS
	VehicleType  uint8
	SpeedKMH     uint16
	CaptureTime  time.Time
	SceneImage   []byte // whole-scene snapshot JPEG, if present
	PlateImage   []byte // cropped plate snapshot JPEG, if present
}

func plateEventFromC(p *C.hik_plate_event) PlateEvent {
	ev := PlateEvent{
		License:      C.GoString(&p.license[0]),
		Confidence:   uint8(p.confidence),
		PlateColor:   uint8(p.plate_color),
		VehicleColor: uint8(p.vehicle_color),
		VehicleType:  uint8(p.vehicle_type),
		SpeedKMH:     uint16(p.speed_kmh),
		CaptureTime:  timeFromHik(p.capture_time),
	}
	if p.scene_image != nil && p.scene_image_len > 0 {
		ev.SceneImage = C.GoBytes(unsafe.Pointer(p.scene_image), C.int(p.scene_image_len))
	}
	if p.plate_image != nil && p.plate_image_len > 0 {
		ev.PlateImage = C.GoBytes(unsafe.Pointer(p.plate_image), C.int(p.plate_image_len))
	}
	return ev
}

// PlateEvents is a convenience wrapper around Alarms that filters to just
// the decoded ANPR events, discarding every other alarm/event type.
func (d *Device) PlateEvents(ctx context.Context) (<-chan PlateEvent, error) {
	events, err := d.Alarms(ctx)
	if err != nil {
		return nil, err
	}
	out := make(chan PlateEvent, 32)
	go func() {
		defer close(out)
		for ev := range events {
			if ev.Plate == nil {
				continue
			}
			select {
			case out <- *ev.Plate:
			default:
			}
		}
	}()
	return out, nil
}

// manualSnapBufPool pools the scratch buffers ManualSnap hands to the SDK -
// see the comment on scratchPool for why. Matches CaptureJPEG's 4 MiB - both
// are a single JPEG snapshot off a device, and shim.cpp's hik_manual_snap
// silently truncates rather than erroring if the real image exceeds this
// buffer, so it's worth staying generous here.
var manualSnapBufPool = newScratchPool(4 << 20) // 4 MiB

// ManualSnap triggers an immediate ANPR snapshot+recognition on channel
// (NET_DVR_ManualSnap) and returns the decoded result, including the scene
// snapshot image when the device provides one.
func (d *Device) ManualSnap(channel int32) (PlateEvent, error) {
	var out C.hik_plate_event
	sceneBuf := manualSnapBufPool.Get()
	defer manualSnapBufPool.Put(sceneBuf)
	var sceneLen C.uint32_t

	err := sdkCall0("ManualSnap", func() C.int32_t {
		return C.hik_manual_snap(C.int32_t(d.userID), C.int32_t(channel), &out,
			(*C.uint8_t)(unsafe.Pointer(&sceneBuf[0])), C.uint32_t(len(sceneBuf)), &sceneLen,
			nil, 0, nil)
	})
	if err != nil {
		return PlateEvent{}, err
	}
	ev := plateEventFromC(&out)
	if sceneLen > 0 {
		ev.SceneImage = append([]byte(nil), sceneBuf[:sceneLen]...)
	}
	return ev, nil
}

// HCNetSDK config command codes for license-plate-recognition parameters
// (NET_DVR_GET_PLATERECOG_PARA / NET_DVR_SET_PLATERECOG_PARA), for use with
// GetConfig/SetConfig in config.go. The exact struct layout for these
// commands is documented in the ANPR/Traffic Device Network SDK developer
// guide (see doc/ in the vendor SDK archive) - this package does not mirror
// it, since it is large, device-family-specific, and rarely needs anything
// beyond the STDXMLConfig/ISAPI passthrough for configuration purposes.
const (
	ConfigCommandGetPlateRecognitionParam uint32 = 8012
	ConfigCommandSetPlateRecognitionParam uint32 = 8013
)
