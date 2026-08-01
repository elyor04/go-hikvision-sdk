package hikvision

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// componentDir is where the vendored HCNetSDKCom/ plugin directory (and
// friends like libcrypto/libssl) lives at build time on this machine. It is
// only a sane *default* for local development against this repo's own
// internal/sdklib/ - real deployments should set Config.ComponentPath to
// wherever they've placed the HCNetSDK runtime files next to their compiled
// binary, and must ensure the OS can find the main shared library itself
// (PATH on Windows, rpath/LD_LIBRARY_PATH on Linux) - see README.md.
func defaultComponentDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	dir := filepath.Dir(thisFile)
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(dir, "..", "internal", "sdklib", "windows_amd64", "lib")
	case "linux":
		return filepath.Join(dir, "..", "internal", "sdklib", "linux_amd64", "lib")
	default:
		return ""
	}
}

// LogLevel mirrors HCNetSDK's NET_DVR_SetLogToFile log levels.
type LogLevel int32

const (
	LogNone LogLevel = iota
	LogError
	LogInfo
	LogDebug = 3
)

// Config controls process-wide HCNetSDK behavior. All fields are optional;
// the zero Config applies sane defaults. Configure (if used at all) must be
// called before the first Device login, since some settings can only be
// applied before NET_DVR_Init.
type Config struct {
	// ComponentPath points the SDK at its own runtime component directory
	// (HCNetSDKCom/, libcrypto, libssl, ...). Defaults to this module's
	// vendored internal/sdklib/<platform>/lib - override this for real
	// deployments where the runtime files live next to your binary instead.
	ComponentPath string

	ConnectTimeout time.Duration // default 3s
	ConnectRetries uint32        // default 3

	AutoReconnect     bool          // default true
	ReconnectInterval time.Duration // default 30s

	// LogDir, if non-empty, enables SDK-internal logging to that directory.
	LogDir   string
	LogLevel LogLevel
}

var (
	initMu       sync.Mutex
	initCount    int
	appliedCfg   Config
	configured   bool
	configuredMu sync.Mutex
)

// Configure applies process-wide SDK settings. Optional: if never called,
// ensureInit applies Config{} (all defaults) on first use. Calling it after
// the SDK has already been initialized (i.e. after any Device has logged in)
// returns an error for settings that HCNetSDK only accepts pre-Init
// (ComponentPath); timeouts/reconnect/logging can be changed at any time.
func Configure(cfg Config) error {
	configuredMu.Lock()
	defer configuredMu.Unlock()

	initMu.Lock()
	alreadyInit := initCount > 0
	initMu.Unlock()

	if alreadyInit && cfg.ComponentPath != appliedCfg.ComponentPath {
		return fmt.Errorf("hikvision: Configure: ComponentPath must be set before the first Device login")
	}
	appliedCfg = cfg
	configured = true

	if alreadyInit {
		return applyRuntimeConfig(cfg)
	}
	return nil
}

func ensureInit() error {
	initMu.Lock()
	defer initMu.Unlock()

	if initCount == 0 {
		configuredMu.Lock()
		cfg := appliedCfg
		wasConfigured := configured
		configuredMu.Unlock()
		if !wasConfigured {
			cfg = Config{}
		}

		path := cfg.ComponentPath
		if path == "" {
			path = defaultComponentDir()
		}
		if path != "" {
			cPath := C.CString(path)
			C.hik_set_sdk_component_path(cPath)
			C.free(unsafe.Pointer(cPath))
		}

		if C.hik_init() != 0 {
			return lastError("Init")
		}

		if err := applyRuntimeConfig(cfg); err != nil {
			C.hik_cleanup()
			return err
		}
	}
	initCount++
	return nil
}

func applyRuntimeConfig(cfg Config) error {
	waitMS := uint32(cfg.ConnectTimeout / time.Millisecond)
	if waitMS == 0 {
		waitMS = 3000
	}
	tries := cfg.ConnectRetries
	if tries == 0 {
		tries = 3
	}
	if C.hik_set_connect_time(C.uint32_t(waitMS), C.uint32_t(tries)) != 0 {
		return lastError("SetConnectTime")
	}

	interval := uint32(cfg.ReconnectInterval / time.Millisecond)
	if interval == 0 {
		interval = 30000
	}
	auto := cfg.AutoReconnect
	if !configured {
		auto = true // default on when Configure was never called
	}
	autoInt := int32(0)
	if auto {
		autoInt = 1
	}
	if C.hik_set_reconnect(C.uint32_t(interval), C.int32_t(autoInt)) != 0 {
		return lastError("SetReconnect")
	}

	if cfg.LogDir != "" {
		cDir := C.CString(cfg.LogDir)
		defer C.free(unsafe.Pointer(cDir))
		if C.hik_set_log_to_file(C.int32_t(cfg.LogLevel), cDir, 1) != 0 {
			return lastError("SetLogToFile")
		}
	}
	return nil
}

func releaseInit() {
	initMu.Lock()
	defer initMu.Unlock()
	initCount--
	if initCount == 0 {
		C.hik_cleanup()
	}
}

// Version returns the HCNetSDK version, e.g. "6.1.9.48" - decoded from
// NET_DVR_GetSDKBuildVersion(), whose 4 bytes are major.minor.patch.build
// (confirmed against this package's own vendored SDK: 0x06010930 -> 6.1.9.48,
// matching the SDK release folder name). Only meaningful once the SDK has
// been initialized (i.e. after the first successful Login, Configure-driven
// init, or OnException registration) - returns "0.0.0.0" beforehand.
func Version() string {
	b := uint32(C.hik_get_sdk_build_version())
	return fmt.Sprintf("%d.%d.%d.%d", (b>>24)&0xff, (b>>16)&0xff, (b>>8)&0xff, b&0xff)
}
