package hikvision

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// xmlConfigBufPool pools the scratch buffers STDXMLConfig hands to the SDK -
// see the comment on scratchPool for why.
var xmlConfigBufPool = newScratchPool(8 << 20) // 8 MiB

// STDXMLConfig calls HCNetSDK's generic ISAPI-style configuration passthrough
// (NET_DVR_STDXMLConfig). url is an ISAPI method+path such as
// "GET /ISAPI/System/deviceInfo" or
// "PUT /ISAPI/Traffic/channels/1/vehiclePositionControl?format=json"; body is
// the request payload (XML or JSON, as the endpoint expects - empty for GET).
// This is the primary escape hatch for the vast majority of HCNetSDK/ISAPI
// functionality this package doesn't wrap with a dedicated method: anything
// documented in the per-feature Device Network SDK developer guides (ANPR,
// access control, thermal, ...) that exposes an ISAPI-style endpoint can be
// driven through here.
func (d *Device) STDXMLConfig(url string, body []byte) ([]byte, error) {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	var inPtr *C.uint8_t
	if len(body) > 0 {
		inPtr = (*C.uint8_t)(unsafe.Pointer(&body[0]))
	}

	out := xmlConfigBufPool.Get()
	defer xmlConfigBufPool.Put(out)
	var outLen C.uint32_t

	err := sdkCall0("STDXMLConfig", func() C.int32_t {
		return C.hik_stdxml_config(C.int32_t(d.userID), cURL, inPtr, C.uint32_t(len(body)),
			(*C.uint8_t)(unsafe.Pointer(&out[0])), C.uint32_t(len(out)), &outLen)
	})
	if err != nil {
		return nil, err
	}
	// Copy into a right-sized slice rather than returning out[:outLen] - out
	// came from xmlConfigBufPool and is returned to the pool via defer above,
	// so the result must not alias it; a right-sized copy also avoids handing
	// the caller a slice backed by an 8 MiB array for what's typically a much
	// smaller response.
	return append([]byte(nil), out[:outLen]...), nil
}

// GetConfig is the escape hatch for HCNetSDK's legacy binary configuration
// API (NET_DVR_GetDVRConfig). command is one of the NET_DVR_GET_* command
// codes from HCNetSDK.h; channel is the target channel (0 for device-wide
// commands, per that command's documentation). Callers are responsible for
// knowing and decoding the exact struct layout HCNetSDK.h documents for
// command.
func (d *Device) GetConfig(command uint32, channel int32, maxSize int) ([]byte, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("hikvision: GetConfig: maxSize must be positive, got %d", maxSize)
	}
	buf := make([]byte, maxSize)
	var outLen C.uint32_t
	err := sdkCall0("GetDVRConfig", func() C.int32_t {
		return C.hik_get_dvr_config(C.int32_t(d.userID), C.uint32_t(command), C.int32_t(channel),
			(*C.uint8_t)(unsafe.Pointer(&buf[0])), C.uint32_t(len(buf)), &outLen)
	})
	if err != nil {
		return nil, err
	}
	// Copy into a right-sized slice rather than returning buf[:outLen] - see
	// the identical comment in CaptureJPEG/STDXMLConfig above: otherwise the
	// returned slice keeps the whole maxSize backing array alive for as long
	// as the caller holds onto it, which matters when a caller passes a
	// generous maxSize "to be safe" for a command whose actual response is
	// much smaller.
	return append([]byte(nil), buf[:outLen]...), nil
}

// SetConfig is the write counterpart to GetConfig (NET_DVR_SetDVRConfig).
func (d *Device) SetConfig(command uint32, channel int32, data []byte) error {
	var inPtr *C.uint8_t
	if len(data) > 0 {
		inPtr = (*C.uint8_t)(unsafe.Pointer(&data[0]))
	}
	err := sdkCall0("SetDVRConfig", func() C.int32_t {
		return C.hik_set_dvr_config(C.int32_t(d.userID), C.uint32_t(command), C.int32_t(channel), inPtr, C.uint32_t(len(data)))
	})
	if err != nil {
		return err
	}
	return nil
}
