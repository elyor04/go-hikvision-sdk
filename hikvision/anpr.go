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

// VehicleKind is HCNetSDK's VTR_RESULT vehicle-classification code.
type VehicleKind uint8

const (
	VehicleOther           VehicleKind = 0
	VehicleBus             VehicleKind = 1
	VehicleTruck           VehicleKind = 2
	VehicleCar             VehicleKind = 3
	VehicleMinibus         VehicleKind = 4
	VehicleSmallTruck      VehicleKind = 5
	VehicleHuman           VehicleKind = 6
	VehicleTumbrel         VehicleKind = 7
	VehicleTrike           VehicleKind = 8
	VehicleSUVMPV          VehicleKind = 9
	VehicleMediumBus       VehicleKind = 10
	VehicleMotorVehicle    VehicleKind = 11
	VehicleNonMotorVehicle VehicleKind = 12
	VehicleSmallCar        VehicleKind = 13
	VehicleMicroCar        VehicleKind = 14
	VehiclePickup          VehicleKind = 15
	VehicleContainerTruck  VehicleKind = 16
	VehicleMiniTruck       VehicleKind = 17
	VehicleSlagCar         VehicleKind = 18
	VehicleCrane           VehicleKind = 19
	VehicleOilTankTruck    VehicleKind = 20
	VehicleConcreteMixer   VehicleKind = 21
	VehiclePlatformTrailer VehicleKind = 22
	VehicleHatchback       VehicleKind = 23
	VehicleSaloon          VehicleKind = 24
	VehicleSportSedan      VehicleKind = 25
	VehicleSmallBus        VehicleKind = 26
)

// PlateColorKind is HCNetSDK's VCA_PLATE_COLOR license-plate color code.
type PlateColorKind uint8

const (
	PlateColorBlue   PlateColorKind = 0
	PlateColorYellow PlateColorKind = 1
	PlateColorWhite  PlateColorKind = 2
	PlateColorBlack  PlateColorKind = 3
	PlateColorGreen  PlateColorKind = 4
	PlateColorBKAir  PlateColorKind = 5 // civil aviation black plate
	PlateColorRed    PlateColorKind = 6
	PlateColorOrange PlateColorKind = 7
	PlateColorBrown  PlateColorKind = 8
	PlateColorOther  PlateColorKind = 0xff
)

// VehicleColorKind is HCNetSDK's VCR_CLR_CLASS vehicle color code.
type VehicleColorKind uint8

const (
	VehicleColorUnsupported VehicleColorKind = 0
	VehicleColorWhite       VehicleColorKind = 1
	VehicleColorSilver      VehicleColorKind = 2
	VehicleColorGray        VehicleColorKind = 3
	VehicleColorBlack       VehicleColorKind = 4
	VehicleColorRed         VehicleColorKind = 5
	VehicleColorDarkBlue    VehicleColorKind = 6
	VehicleColorBlue        VehicleColorKind = 7
	VehicleColorYellow      VehicleColorKind = 8
	VehicleColorGreen       VehicleColorKind = 9
	VehicleColorBrown       VehicleColorKind = 10
	VehicleColorPink        VehicleColorKind = 11
	VehicleColorPurple      VehicleColorKind = 12
	VehicleColorDarkGray    VehicleColorKind = 13
	VehicleColorCyan        VehicleColorKind = 14
)

// Direction is HCNetSDK's byDir monitoring-direction code. Only carried by
// the newer ITS-format alarm (COMM_ITS_PLATE_RESULT); DirectionUnknown for
// the older COMM_UPLOAD_PLATE_RESULT/COMM_PLATE_RESULT_V50 alarm formats and
// for ManualSnap, which have no direction field at all.
type Direction uint8

const (
	DirectionUnknown       Direction = 0
	DirectionUp            Direction = 1
	DirectionDown          Direction = 2
	DirectionBidirectional Direction = 3
	DirectionWestward      Direction = 4
	DirectionNorthward     Direction = 5
	DirectionEastward      Direction = 6
	DirectionSouthward     Direction = 7
	DirectionOther         Direction = 8
)

// PlateEvent is a decoded ANPR/LPR (license plate recognition) result,
// whether it arrived via a live alarm subscription (Device.Alarms /
// Device.PlateEvents, commands COMM_ITS_PLATE_RESULT / COMM_UPLOAD_PLATE_RESULT
// / COMM_PLATE_RESULT_V50) or a manual snapshot (Device.ManualSnap).
type PlateEvent struct {
	License      string // recognized plate text
	Confidence   uint8  // 0-100
	PlateColor   PlateColorKind
	VehicleColor VehicleColorKind
	VehicleType  VehicleKind
	SpeedKMH     uint16
	CaptureTime  time.Time
	SceneImage   []byte // whole-scene snapshot JPEG, if present
	PlateImage   []byte // cropped plate snapshot JPEG, if present

	Direction Direction
	// Country is byCountry - see COUNTRY_INDEX in HCNetSDK.h for the code
	// table (~235 entries - not mirrored into named constants here). 0 if
	// not reported.
	Country uint8
	// Lane is byDriveChan, the 1-based traffic lane index. 0 if not reported.
	Lane uint8
	// RawXML is the full ISAPI <EventNotificationAlert> XML document the
	// device attached to this event, if any - it carries many
	// device/firmware-specific fields (safety-belt/phone-use/dangerous-goods
	// detection, illegal-type descriptions, and more) this package doesn't
	// mirror into typed fields. Parse it yourself with encoding/xml if you
	// need them.
	RawXML []byte
}

func plateEventFromC(p *C.hik_plate_event) PlateEvent {
	ev := PlateEvent{
		License:      C.GoString(&p.license[0]),
		Confidence:   uint8(p.confidence),
		PlateColor:   PlateColorKind(p.plate_color),
		VehicleColor: VehicleColorKind(p.vehicle_color),
		VehicleType:  VehicleKind(p.vehicle_type),
		SpeedKMH:     uint16(p.speed_kmh),
		CaptureTime:  timeFromHik(p.capture_time),
		Direction:    Direction(p.direction),
		Country:      uint8(p.country),
		Lane:         uint8(p.lane),
	}
	if p.scene_image != nil && p.scene_image_len > 0 {
		ev.SceneImage = C.GoBytes(unsafe.Pointer(p.scene_image), C.int(p.scene_image_len))
	}
	if p.plate_image != nil && p.plate_image_len > 0 {
		ev.PlateImage = C.GoBytes(unsafe.Pointer(p.plate_image), C.int(p.plate_image_len))
	}
	if p.raw_xml != nil && p.raw_xml_len > 0 {
		ev.RawXML = C.GoBytes(unsafe.Pointer(p.raw_xml), C.int(p.raw_xml_len))
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
// are a single JPEG snapshot off a device.
//
// These are not merely a place to copy the result into: NET_DVR_ManualSnap
// writes the JPEG directly into caller-owned memory, and the SDK is never told
// how much of it there is (see hik_manual_snap's own comment in shim.cpp).
// 4 MiB against an observed ~560 KB for a 1920x1080 scene is the headroom that
// bet needs; the length reported back is clamped to this capacity regardless.
var manualSnapBufPool = newScratchPool(4 << 20) // 4 MiB

// manualSnapPlateBufPool pools the scratch buffers ManualSnap hands to the SDK
// for the cropped plate image - see the comment on scratchPool for why. A
// plate crop is a small sub-region of the scene image (measured 384x80,
// ~4 KB), so 1 MiB is already enormous relative to manualSnapBufPool's 4 MiB.
var manualSnapPlateBufPool = newScratchPool(1 << 20) // 1 MiB

// ManualSnap triggers an immediate ANPR snapshot+recognition on channel
// (NET_DVR_ManualSnap) and returns the decoded result, including the scene and
// cropped-plate snapshot images and the device's own capture time.
//
// Fixed 2026-08-22, verified against an iDS-TCM203-A: this used to return the
// plate text and confidence with no images and no CaptureTime, which read as a
// device limitation and was not one. shim.cpp's hik_manual_snap left the
// picture buffers NULL (they are INPUTS for this call, unlike the alarm path),
// read the two images from the wrong pointers, and never parsed byAbsTime. See
// that function's doc comment for the measurements.
func (d *Device) ManualSnap(channel int32) (PlateEvent, error) {
	var out C.hik_plate_event
	sceneBuf := manualSnapBufPool.Get()
	defer manualSnapBufPool.Put(sceneBuf)
	plateBuf := manualSnapPlateBufPool.Get()
	defer manualSnapPlateBufPool.Put(plateBuf)
	var sceneLen, plateLen C.uint32_t

	err := sdkCall0("ManualSnap", func() C.int32_t {
		return C.hik_manual_snap(C.int32_t(d.userID), C.int32_t(channel), &out,
			(*C.uint8_t)(unsafe.Pointer(&sceneBuf[0])), C.uint32_t(len(sceneBuf)), &sceneLen,
			(*C.uint8_t)(unsafe.Pointer(&plateBuf[0])), C.uint32_t(len(plateBuf)), &plateLen)
	})
	if err != nil {
		return PlateEvent{}, err
	}
	ev := plateEventFromC(&out)
	if sceneLen > 0 {
		ev.SceneImage = append([]byte(nil), sceneBuf[:sceneLen]...)
	}
	if plateLen > 0 {
		ev.PlateImage = append([]byte(nil), plateBuf[:plateLen]...)
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
