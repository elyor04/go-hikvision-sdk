package hikvision

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// DeviceInfo is the subset of NET_DVR_DEVICEINFO_V40 this package exposes.
// Fetch the full struct yourself via GetDVRConfig/STDXMLConfig (config.go)
// if you need fields not listed here.
type DeviceInfo struct {
	SerialNumber   string
	AlarmInPorts   uint8
	AlarmOutPorts  uint8
	DiskNum        uint8
	DVRType        uint8
	AnalogChannels uint8
	StartChannel   uint8
	// IPChannels is the device's total IP/digital channel count (combines
	// the SDK's low-byte + high-byte IP-channel-count fields).
	IPChannels     uint32
	StartIPChannel uint8
	DeviceType     uint16
}

// Device is a logged-in HCNetSDK session with a single device (DVR/NVR/IPC).
// Create one with Login. Always Close it when done - Close stops any
// outstanding preview/playback sessions before logging out, matching the
// order HCNetSDK's own documentation and demos use.
type Device struct {
	userID int32
	// Info is the device information returned at login time.
	Info DeviceInfo

	mu       sync.Mutex
	closed   bool
	sessions map[io_Closer]struct{}
}

// io_Closer avoids importing io just for this internal bookkeeping type set.
type io_Closer interface{ Close() error }

// LoginOptions configures Device login. Address and Port are required.
type LoginOptions struct {
	Address  string // IP or hostname
	Port     uint16 // typically 8000 for the private protocol
	Username string
	Password string
	// UseTLS enables the SDK's TLS transport (byHttps=1) instead of plain TCP.
	UseTLS bool
}

// Login establishes a new HCNetSDK session (NET_DVR_Login_V40). The first
// Login in the process triggers NET_DVR_Init (see Configure to customize
// process-wide settings beforehand).
func Login(opts LoginOptions) (*Device, error) {
	if err := ensureInit(); err != nil {
		return nil, err
	}

	var in C.hik_login_info
	cSetString(&in.device_address[0], len(in.device_address), opts.Address)
	in.port = C.uint16_t(opts.Port)
	cSetString(&in.user_name[0], len(in.user_name), opts.Username)
	cSetString(&in.password[0], len(in.password), opts.Password)
	if opts.UseTLS {
		in.use_https = 1
	}

	var out C.hik_device_info
	userID := int32(C.hik_login(&in, &out))
	if userID < 0 {
		releaseInit()
		return nil, lastError("Login")
	}

	return &Device{
		userID:   userID,
		Info:     deviceInfoFromC(&out),
		sessions: make(map[io_Closer]struct{}),
	}, nil
}

// UserID returns the raw HCNetSDK user/session ID for this login. Useful
// when using the escape-hatch functions in config.go.
func (d *Device) UserID() int32 { return d.userID }

func (d *Device) track(c io_Closer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions[c] = struct{}{}
}

func (d *Device) untrack(c io_Closer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sessions, c)
}

// Close stops any outstanding preview/playback sessions opened through this
// Device, logs out, and releases this package's reference on the
// process-wide SDK (calling NET_DVR_Cleanup once the last Device closes).
func (d *Device) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	sessions := make([]io_Closer, 0, len(d.sessions))
	for c := range d.sessions {
		sessions = append(sessions, c)
	}
	d.mu.Unlock()

	for _, c := range sessions {
		_ = c.Close()
	}

	var err error
	if C.hik_logout(C.int32_t(d.userID)) != 0 {
		err = lastError("Logout")
	}
	releaseInit()
	return err
}

func cSetString(dst *C.char, dstLen int, s string) {
	if len(s) >= dstLen {
		s = s[:dstLen-1]
	}
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	src := unsafe.Slice((*byte)(unsafe.Pointer(cs)), len(s)+1)
	out := unsafe.Slice((*byte)(unsafe.Pointer(dst)), dstLen)
	copy(out, src)
}

func deviceInfoFromC(in *C.hik_device_info) DeviceInfo {
	return DeviceInfo{
		SerialNumber:   cString((*C.char)(unsafe.Pointer(&in.serial_number[0])), len(in.serial_number)),
		AlarmInPorts:   uint8(in.alarm_in_port_num),
		AlarmOutPorts:  uint8(in.alarm_out_port_num),
		DiskNum:        uint8(in.disk_num),
		DVRType:        uint8(in.dvr_type),
		AnalogChannels: uint8(in.chan_num),
		StartChannel:   uint8(in.start_chan),
		IPChannels:     uint32(in.ip_chan_num),
		StartIPChannel: uint8(in.start_dchan),
		DeviceType:     uint16(in.dev_type),
	}
}

func (d *Device) String() string {
	return fmt.Sprintf("hikvision.Device{userID=%d}", d.userID)
}
