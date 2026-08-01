<#
.SYNOPSIS
  Vendors the Hikvision HCNetSDK headers and libraries into internal/sdklib so that
  `go build` can cgo-compile and link against them.

  This copies files OUT of your local extracted copy of the official Hikvision SDK
  archives (EN-HCNetSDKV6.1.9.48_build20230410_win64 / _linux64, or a newer build) INTO
  this repo's internal/sdklib/ directory, which is gitignored. Every developer/machine
  runs this once (and again after updating the vendor SDK) instead of committing the
  proprietary binaries to git. See LICENSE for why.

.PARAMETER Win64Sdk
  Path to the extracted Windows x64 SDK root (the folder containing incEn/ and lib/).

.PARAMETER Linux64Sdk
  Path to the extracted Linux x64 SDK root (the folder containing incEn/ and lib/).
  Optional - if you only build/run on Windows you can omit -Linux64Sdk, but the module
  will fail to compile with GOOS=linux until it's vendored too.

.EXAMPLE
  ./scripts/vendor-sdk.ps1
  (uses the default sibling-folder locations next to this repo)

.EXAMPLE
  ./scripts/vendor-sdk.ps1 -Win64Sdk "D:\sdks\EN-HCNetSDKV6.1.9.53_build20240101_win64"
#>
param(
    [string]$Win64Sdk   = (Join-Path $PSScriptRoot "..\..\EN-HCNetSDKV6.1.9.48_build20230410_win64"),
    [string]$Linux64Sdk = (Join-Path $PSScriptRoot "..\..\EN-HCNetSDKV6.1.9.48_build20230410_linux64")
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$SdkLib   = Join-Path $RepoRoot "internal\sdklib"

function Vendor-Platform {
    param(
        [string]$SourceRoot,
        [string]$PlatformDir,
        [string]$PlatformLabel
    )

    if (-not (Test-Path $SourceRoot)) {
        Write-Warning "[$PlatformLabel] SDK root not found at '$SourceRoot' - skipping. Pass -Win64Sdk/-Linux64Sdk explicitly if it lives elsewhere."
        return
    }

    $incSrc = Join-Path $SourceRoot "incEn"
    $libSrc = Join-Path $SourceRoot "lib"

    if (-not (Test-Path $incSrc)) { throw "[$PlatformLabel] Expected headers at '$incSrc' - is this a genuine HCNetSDK release folder?" }
    if (-not (Test-Path $libSrc)) { throw "[$PlatformLabel] Expected libs at '$libSrc' - is this a genuine HCNetSDK release folder?" }

    $destInclude = Join-Path $SdkLib "$PlatformDir\include"
    $destLib     = Join-Path $SdkLib "$PlatformDir\lib"

    New-Item -ItemType Directory -Force -Path $destInclude | Out-Null
    New-Item -ItemType Directory -Force -Path $destLib     | Out-Null

    Write-Host "[$PlatformLabel] Copying headers: $incSrc -> $destInclude"
    Copy-Item -Path (Join-Path $incSrc "*") -Destination $destInclude -Recurse -Force

    Write-Host "[$PlatformLabel] Copying libs (import libs, shared libs, HCNetSDKCom plugins): $libSrc -> $destLib"
    Copy-Item -Path (Join-Path $libSrc "*") -Destination $destLib -Recurse -Force

    Write-Host "[$PlatformLabel] Done."
}

Vendor-Platform -SourceRoot $Win64Sdk   -PlatformDir "windows_amd64" -PlatformLabel "windows/amd64"
Vendor-Platform -SourceRoot $Linux64Sdk -PlatformDir "linux_amd64"   -PlatformLabel "linux/amd64"

Write-Host ""
Write-Host "Vendoring complete. internal/sdklib/ is gitignored - rerun this script on any new machine/checkout before building."
Write-Host "Windows note: for 'go run'/'go test'/built .exe files to find HCNetSDK.dll and its dependent DLLs at process startup,"
Write-Host "either add internal/sdklib/windows_amd64/lib to PATH for the shell you build/run from, or copy its *.dll files next to your binary."
