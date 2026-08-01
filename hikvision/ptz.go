package hikvision

/*
#include "shim.h"
*/
import "C"

// PTZCommand mirrors HCNetSDK's PTZ direction/zoom command codes (used with
// NET_DVR_PTZControlWithSpeed_Other).
type PTZCommand uint32

const (
	PTZTiltUp    PTZCommand = 21
	PTZTiltDown  PTZCommand = 22
	PTZPanLeft   PTZCommand = 23
	PTZPanRight  PTZCommand = 24
	PTZUpLeft    PTZCommand = 25
	PTZUpRight   PTZCommand = 26
	PTZDownLeft  PTZCommand = 27
	PTZDownRight PTZCommand = 28
	PTZZoomIn    PTZCommand = 11
	PTZZoomOut   PTZCommand = 12
	PTZFocusNear PTZCommand = 13
	PTZFocusFar  PTZCommand = 14
	PTZIrisOpen  PTZCommand = 15
	PTZIrisClose PTZCommand = 16
)

// PTZPresetCommand mirrors HCNetSDK's preset command codes (used with
// NET_DVR_PTZPreset_Other).
type PTZPresetCommand uint32

const (
	PTZSetPreset   PTZPresetCommand = 8
	PTZClearPreset PTZPresetCommand = 9
	PTZGotoPreset  PTZPresetCommand = 39
)

// PTZControl issues a continuous PTZ move command at the given speed (1-100)
// on channel. Pass stop=true to halt movement (matching a prior non-stop
// call with the same command).
func (d *Device) PTZControl(channel int32, cmd PTZCommand, stop bool, speed uint32) error {
	stopArg := uint32(0)
	if stop {
		stopArg = 1
	}
	if C.hik_ptz_control(C.int32_t(d.userID), C.int32_t(channel), C.uint32_t(cmd), C.uint32_t(stopArg), C.uint32_t(speed)) != 0 {
		return lastError("PTZControlWithSpeed")
	}
	return nil
}

// PTZPreset issues a preset command (set/clear/goto) for the given 1-based
// preset index on channel.
func (d *Device) PTZPreset(channel int32, cmd PTZPresetCommand, presetIndex uint32) error {
	if C.hik_ptz_preset(C.int32_t(d.userID), C.int32_t(channel), C.uint32_t(cmd), C.uint32_t(presetIndex)) != 0 {
		return lastError("PTZPreset")
	}
	return nil
}
