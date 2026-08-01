# go-hikvision-sdk

A cross-platform (Windows/Linux, amd64) Go wrapper around Hikvision's native
**HCNetSDK** (the C/C++ "Device Network SDK" — `HCNetSDK.dll` / `libhcnetsdk.so`),
not the HTTP/ISAPI REST interface.

Covers: login, real-time video streaming (raw compressed frames), PTZ
control, JPEG snapshots, remote playback/search/download, alarm/event
subscription, and full ANPR/LPR (license-plate recognition) event handling —
plus a generic escape hatch (`STDXMLConfig` / `GetConfig` / `SetConfig`) for
anything else HCNetSDK exposes.

## Why this exists / scope

HCNetSDK's own header (`HCNetSDK.h`) declares **834 functions** and **~2500
structs**. Hand-wrapping all of it isn't a good use of anyone's time — most
of it is for niche device families (access control, thermal, queue
management, ...) that a given integration will never touch. Instead, this
package:

- Hand-wraps the ~20% of the API that covers the large majority of real
  integrations, with an idiomatic Go surface (channels, `context.Context`,
  typed errors, `time.Time`, ...).
- Exposes `Device.STDXMLConfig` / `GetConfig` / `SetConfig` as a generic
  passthrough for everything else — see the per-feature "Device Network SDK
  Developer Guide" PDFs shipped inside the official SDK archive for the exact
  request/response shape of any endpoint you need that isn't wrapped
  directly.

## Architecture: why a C++ shim?

`HCNetSDK.h` declares every exported function through a macro that
unconditionally expands to `extern "C" ...` — **not** guarded by
`#ifdef __cplusplus`. That makes the header invalid in a plain C translation
unit, which is what cgo uses for the code above `import "C"`. Confirmed
directly: `gcc -c` on the raw header fails immediately on the first function
declaration; `g++ -c` compiles it cleanly.

So the actual structure is:

```
Go code  <-- plain C ABI -->  hikvision/shim.h + shim.cpp (C++)  <-- C++ -->  HCNetSDK.h / HCNetSDK.dll|.so
```

- **`hikvision/shim.h`** — a small, plain-C, fixed-width-type (`stdint.h`)
  header. This is the only header Go's cgo preamble includes.
- **`hikvision/shim.cpp`** — the *only* file that includes the real
  `HCNetSDK.h` (compiled as C++ via cgo's automatic `CXX` support for `.cpp`
  files in a cgo package). It translates between HCNetSDK's real structs and
  the plain POD structs declared in `shim.h`.
- Callbacks: cgo cannot express "give me a C function pointer to one of my
  own `//export`ed Go functions" from Go code, so `shim.cpp` calls the Go
  trampolines (`callbacks.go`) directly via `extern "C"` forward
  declarations, rather than Go passing function pointers in. Handles are
  round-tripped through `void* pUser` as a plain `uintptr_t` (the
  [`runtime/cgo.Handle`](https://pkg.go.dev/runtime/cgo#Handle) pattern), not
  a disguised pointer.

This was validated empirically before writing the rest of the package: a
minimal shim (`NET_DVR_Init`/`GetSDKVersion`/`Cleanup`) was compiled with
`g++`, linked with plain `gcc`/`ld` against the vendor-supplied
`HCNetSDK.lib` (an MSVC-format import library — mingw-w64's linker consumes
it fine for `extern "C"` exports), and run successfully before any Go code
was written.

## Setup

You need your own copy of the official Hikvision SDK (this repo does not
vendor or redistribute Hikvision's binaries/headers — see [Licensing](#licensing)).

1. Extract the official SDK archive(s) for the platform(s) you build for,
   e.g. `EN-HCNetSDKV6.1.x.x_buildYYYYMMDD_win64` and/or `_linux64`. Each
   should contain an `incEn/` and `lib/` folder.
2. Vendor them into this module (gitignored — every machine/checkout runs
   this once):

   ```powershell
   ./scripts/vendor-sdk.ps1 -Win64Sdk "C:\path\to\EN-HCNetSDKV6.1.x.x_..._win64" -Linux64Sdk "C:\path\to\..._linux64"
   ```

   ```bash
   ./scripts/vendor-sdk.sh /path/to/..._win64 /path/to/..._linux64
   ```

   Both default to sibling folders next to this repo if you don't pass
   arguments (matching how the archives are typically extracted).
3. `go build ./...` — requires `CGO_ENABLED=1` and a C++ compiler
   (`g++`/mingw-w64 on Windows, `g++`/`gcc` on Linux) on `PATH`.

### Runtime library discovery

`NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SDK_PATH, ...)` is called
automatically before `NET_DVR_Init` to point the SDK at its own component
directory (`HCNetSDKCom/`, `libcrypto`, `libssl`, ...) — this is the
official, documented mechanism and avoids any `PATH`/`LD_LIBRARY_PATH`
manipulation for *that* part. It defaults to this module's own
`internal/sdklib/<platform>/lib`; override it for real deployments via
`hikvision.Configure(hikvision.Config{ComponentPath: "..."})` **before**
your first `Login`.

That said, the OS still has to find the *main* shared library
(`HCNetSDK.dll` / `libhcnetsdk.so`) at process startup, before any of your Go
code runs — `SetSDKInitCfg` can't help with that part:

- **Windows**: put `internal/sdklib/windows_amd64/lib`'s `*.dll` files next
  to your built `.exe`, or add that directory to `PATH`.
- **Linux**: this module's `#cgo LDFLAGS` bakes an `-Wl,-rpath` pointing at
  `internal/sdklib/linux_amd64/lib` at **build time** — this works as-is if
  you build and run on the same machine/filesystem (including most
  single-stage Docker builds), which covers most deployments. If you build
  and deploy separately, set `LD_LIBRARY_PATH` to wherever you've copied
  those `.so` files instead.

## Usage

```go
package main

import (
	"context"
	"log"

	"github.com/elyor04/go-hikvision-sdk/hikvision"
)

func main() {
	dev, err := hikvision.Login(hikvision.LoginOptions{
		Address:  "192.168.1.64",
		Port:     8000,
		Username: "admin",
		Password: "your-password",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer dev.Close()

	// Raw H.264/H.265 elementary stream - pipe into ffmpeg/gocv/whatever you like.
	stream, err := dev.RealPlay(context.Background(), 1, hikvision.MainStream)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	for frame := range stream.Frames() {
		_ = frame // frame.Type, frame.Data
	}
}
```

See `examples/` for complete, runnable programs:

| Example | Demonstrates |
|---|---|
| `login-and-snapshot` | Login, device info, JPEG capture |
| `realplay-dump` | Live streaming raw video to stdout (pipe into ffmpeg) |
| `ptz-control` | PTZ pan/tilt/zoom |
| `alarm-listener` | Generic alarm/event subscription + exception callback |
| `anpr-listener` | License-plate recognition event subscription, saving snapshots |
| `playback-download` | Server-side recording search + download |

Each example reads connection details from env vars (`HIK_HOST`, `HIK_PORT`,
`HIK_USER`, `HIK_PASS`, and sometimes `HIK_CHANNEL`) since none of this can
be exercised without a real device reachable from wherever you run it —
there is no camera reachable from the environment this package was
originally developed in, so these are the closest thing to an integration
test available; run them yourself against your own hardware to validate
end-to-end.

```powershell
$env:HIK_HOST="192.168.1.64"; $env:HIK_USER="admin"; $env:HIK_PASS="..."
go run ./examples/login-and-snapshot
```

## ANPR / license-plate recognition

```go
plates, err := dev.PlateEvents(ctx)
// ...
for ev := range plates {
    fmt.Println(ev.License, ev.Confidence, ev.SpeedKMH)
    // ev.SceneImage / ev.PlateImage are JPEG bytes, when present
}
```

`PlateEvents` is a filtered view over `Device.Alarms`, decoding
`COMM_ITS_PLATE_RESULT` / `COMM_UPLOAD_PLATE_RESULT` / `COMM_PLATE_RESULT_V50`
events into `PlateEvent`. `Device.ManualSnap` triggers an immediate
recognition instead of waiting for a live event. Plate-recognition device
*configuration* (thresholds, regions, etc.) goes through the generic
`GetConfig`/`SetConfig`/`STDXMLConfig` escape hatch — see
`ConfigCommandGetPlateRecognitionParam` / `...Set...` in `anpr.go` and the
"ANPR" Device Network SDK developer guide PDF in the vendor archive for the
exact struct/JSON shape.

## Testing

`go test ./...` runs the package's unit tests — pure decode/formatting logic
exercised through synthetic C structs (`hikvision/cgo_test_support.go`; Go's
`_test.go` files cannot themselves use `import "C"`, so the small amount of
cgo-touching test scaffolding lives in a regular, unexported-only source
file instead). No live device is required or contacted by `go test`.

On Windows, the vendor DLL directory needs to be on `PATH` for the test
binary to load HCNetSDK (same requirement as running any built binary — see
above):

```powershell
$env:PATH = "$PWD\internal\sdklib\windows_amd64\lib;$env:PATH"
go test ./...
```

## Package layout

```
hikvision/            the public Go package
  shim.h / shim.cpp    C++ bridge (see Architecture above)
  cgo.go               #cgo directives (per-platform CFLAGS/LDFLAGS)
  sdk.go               process-wide Init/Cleanup (refcounted), Configure, Version
  errors.go            typed Error + curated NET_DVR error-code table
  callbacks.go         cgo.Handle registry + //export trampolines
  device.go            Login/Logout, DeviceInfo
  preview.go           RealPlay (live streaming)
  playback.go          FindRecordings, Playback, Download
  ptz.go                PTZ control/presets
  capture.go            JPEG snapshot
  alarm.go              Alarm subscription, exception callback
  anpr.go                License-plate recognition
  config.go              STDXMLConfig / GetConfig / SetConfig escape hatch
internal/sdklib/        vendored SDK headers/libs (gitignored - see Setup)
scripts/                 vendor-sdk.ps1 / vendor-sdk.sh
examples/                 runnable demos (see table above)
```

## Licensing

This repository's own Go/C++ source is MIT-licensed (see `LICENSE`). It does
**not** include or redistribute any Hikvision-supplied binaries, headers, or
documentation — `internal/sdklib/` is populated locally by
`scripts/vendor-sdk.*` from your own copy of the official SDK and is
gitignored. Those vendored files remain subject to Hikvision's own SDK
license terms (see the `Open Source Software Licenses*.txt` files shipped
inside the official SDK archive).
