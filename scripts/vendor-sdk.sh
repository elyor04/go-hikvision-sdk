#!/usr/bin/env bash
# Vendors the Hikvision HCNetSDK headers and libraries into internal/sdklib so that
# `go build` can cgo-compile and link against them.
#
# This copies files OUT of your local extracted copy of the official Hikvision SDK
# archives (EN-HCNetSDKV6.1.9.48_build20230410_win64 / _linux64, or a newer build) INTO
# this repo's internal/sdklib/ directory, which is gitignored. Every developer/machine
# runs this once (and again after updating the vendor SDK) instead of committing the
# proprietary binaries to git. See LICENSE for why.
#
# Usage:
#   ./scripts/vendor-sdk.sh [win64_sdk_root] [linux64_sdk_root]
#
# Defaults to the sibling-folder locations next to this repo, matching how the
# official archives are typically extracted side by side.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SDK_LIB="$REPO_ROOT/internal/sdklib"

WIN64_SDK="${1:-$REPO_ROOT/../EN-HCNetSDKV6.1.9.48_build20230410_win64}"
LINUX64_SDK="${2:-$REPO_ROOT/../EN-HCNetSDKV6.1.9.48_build20230410_linux64}"

vendor_platform() {
    local source_root="$1"
    local platform_dir="$2"
    local platform_label="$3"

    if [ ! -d "$source_root" ]; then
        echo "WARNING: [$platform_label] SDK root not found at '$source_root' - skipping. Pass it as an argument if it lives elsewhere." >&2
        return
    fi

    local inc_src="$source_root/incEn"
    local lib_src="$source_root/lib"

    if [ ! -d "$inc_src" ]; then echo "ERROR: [$platform_label] Expected headers at '$inc_src' - is this a genuine HCNetSDK release folder?" >&2; exit 1; fi
    if [ ! -d "$lib_src" ]; then echo "ERROR: [$platform_label] Expected libs at '$lib_src' - is this a genuine HCNetSDK release folder?" >&2; exit 1; fi

    local dest_include="$SDK_LIB/$platform_dir/include"
    local dest_lib="$SDK_LIB/$platform_dir/lib"

    mkdir -p "$dest_include" "$dest_lib"

    echo "[$platform_label] Copying headers: $inc_src -> $dest_include"
    cp -R "$inc_src"/. "$dest_include"/

    echo "[$platform_label] Copying libs (shared libs + HCNetSDKCom plugins): $lib_src -> $dest_lib"
    cp -R "$lib_src"/. "$dest_lib"/

    echo "[$platform_label] Done."
}

vendor_platform "$WIN64_SDK" "windows_amd64" "windows/amd64"
vendor_platform "$LINUX64_SDK" "linux_amd64" "linux/amd64"

echo
echo "Vendoring complete. internal/sdklib/ is gitignored - rerun this script on any new machine/checkout before building."
echo "Linux note: the process must be able to find libhcnetsdk.so and friends at startup (via rpath baked in by this module,"
echo "or LD_LIBRARY_PATH=internal/sdklib/linux_amd64/lib) - see README.md."
