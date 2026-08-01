package hikvision

/*
#include "shim.h"
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

// Preview/playback callbacks pass a runtime/cgo.Handle through HCNetSDK's
// void* pUser as a plain integer (uintptr_t on the C side) rather than as a
// disguised pointer - the recommended cgo.Handle idiom, and it avoids the
// (legitimate) "possible misuse of unsafe.Pointer" vet warning that
// synthesizing an unsafe.Pointer from an arbitrary uintptr would trigger.

//export goRealDataTrampoline
func goRealDataTrampoline(playHandle C.int32_t, dataType C.uint32_t, buffer *C.uint8_t, bufSize C.uint32_t, user C.uintptr_t) {
	v := cgo.Handle(user).Value()
	sess, ok := v.(*previewSession)
	if !ok || sess == nil {
		return
	}
	data := C.GoBytes(unsafe.Pointer(buffer), C.int(bufSize))
	sess.deliver(StreamDataType(dataType), data)
}

//export goPlaybackDataTrampoline
func goPlaybackDataTrampoline(playHandle C.int32_t, dataType C.uint32_t, buffer *C.uint8_t, bufSize C.uint32_t, user C.uintptr_t) {
	v := cgo.Handle(user).Value()
	sess, ok := v.(*playbackSession)
	if !ok || sess == nil {
		return
	}
	data := C.GoBytes(unsafe.Pointer(buffer), C.int(bufSize))
	sess.deliver(StreamDataType(dataType), data)
}

//export goExceptionTrampoline
func goExceptionTrampoline(exceptionType C.uint32_t, userID C.int32_t, handle C.int32_t, user unsafe.Pointer) {
	dispatchException(ExceptionType(exceptionType), int32(userID), int32(handle))
}

//export goAlarmTrampoline
func goAlarmTrampoline(command C.int32_t, alarmer *C.hik_alarmer, raw *C.uint8_t, rawLen C.uint32_t,
	isPlate C.int32_t, plate *C.hik_plate_event, user unsafe.Pointer) {
	a := Alarmer{}
	if alarmer != nil {
		a.UserIDValid = alarmer.user_id_valid != 0
		a.UserID = int32(alarmer.user_id)
		a.DeviceIPValid = alarmer.device_ip_valid != 0
		a.DeviceIP = cString(&alarmer.device_ip[0], len(alarmer.device_ip))
		a.SerialValid = alarmer.serial_valid != 0
		a.SerialNumber = cString((*C.char)(unsafe.Pointer(&alarmer.serial_number[0])), len(alarmer.serial_number))
	}

	ev := Event{
		Command: Command(command),
		Alarmer: a,
		Raw:     C.GoBytes(unsafe.Pointer(raw), C.int(rawLen)),
	}
	if isPlate != 0 && plate != nil {
		pe := plateEventFromC(plate)
		ev.Plate = &pe
	}
	dispatchAlarm(ev)
}
