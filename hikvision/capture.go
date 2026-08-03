package hikvision

/*
#include "shim.h"
*/
import "C"

import "unsafe"

// JPEGQuality mirrors HCNetSDK's NET_DVR_JPEGPARA wPicQuality values.
type JPEGQuality uint16

const (
	JPEGBest    JPEGQuality = 0
	JPEGBetter  JPEGQuality = 1
	JPEGAverage JPEGQuality = 2
)

// PicSizeAuto (0xff) uses the current stream's resolution instead of a fixed
// size - the common choice unless you specifically need one of HCNetSDK's
// enumerated resolution codes (see the Device Network SDK developer guide).
const PicSizeAuto uint16 = 0xff

// CaptureJPEG takes a single JPEG snapshot of channel and returns the
// encoded image bytes (NET_DVR_CaptureJPEGPicture_NEW).
func (d *Device) CaptureJPEG(channel int32, quality JPEGQuality) ([]byte, error) {
	const maxJPEGSize = 4 << 20 // 4 MiB is generous for a single snapshot
	buf := make([]byte, maxJPEGSize)

	var written C.uint32_t
	err := sdkCall0("CaptureJPEGPicture", func() C.int32_t {
		return C.hik_capture_jpeg(C.int32_t(d.userID), C.int32_t(channel), C.uint16_t(PicSizeAuto), C.uint16_t(quality),
			(*C.uint8_t)(unsafe.Pointer(&buf[0])), C.uint32_t(len(buf)), &written)
	})
	if err != nil {
		return nil, err
	}
	return buf[:written], nil
}
