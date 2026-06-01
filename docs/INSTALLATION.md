# Installation

This page documents reproducible installation paths for `pg-contract`.

Use a pinned version when installing in CI or sharing setup instructions:

```sh
VERSION=v0.1.0-alpha.2
```

## Go Install

If Go is already available, install the CLI directly from the tagged module version:

```sh
go install github.com/get-felipe/pg-contract/cmd/pg-contract@v0.1.0-alpha.2
pg-contract version
```

Go installs command binaries into `GOBIN` when it is set, otherwise into the `bin` directory under `GOPATH`. Make sure that directory is on your `PATH`.

## Release Archives

GitHub Release archives are available for macOS, Linux, and Windows on `amd64` and `arm64`.

Published assets for `v0.1.0-alpha.2`:

- `pg-contract_0.1.0-alpha.2_darwin_amd64.tar.gz`
- `pg-contract_0.1.0-alpha.2_darwin_arm64.tar.gz`
- `pg-contract_0.1.0-alpha.2_linux_amd64.tar.gz`
- `pg-contract_0.1.0-alpha.2_linux_arm64.tar.gz`
- `pg-contract_0.1.0-alpha.2_windows_amd64.zip`
- `pg-contract_0.1.0-alpha.2_windows_arm64.zip`
- `pg-contract_0.1.0-alpha.2_checksums.txt`

### macOS

Use `ARCH=arm64` for Apple Silicon and `ARCH=amd64` for Intel Macs.

```sh
VERSION=v0.1.0-alpha.2
ASSET_VERSION="${VERSION#v}"
OS=darwin
ARCH=arm64
ARCHIVE="pg-contract_${ASSET_VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="pg-contract_${ASSET_VERSION}_checksums.txt"
BASE_URL="https://github.com/get-felipe/pg-contract/releases/download/${VERSION}"

curl -LO "${BASE_URL}/${ARCHIVE}"
curl -LO "${BASE_URL}/${CHECKSUMS}"

grep " ${ARCHIVE}$" "${CHECKSUMS}" | shasum -a 256 -c -

tar -xzf "${ARCHIVE}"
mkdir -p "$HOME/.local/bin"
install -m 0755 pg-contract "$HOME/.local/bin/pg-contract"

pg-contract version
```

Make sure `$HOME/.local/bin` is on your `PATH`, or install the binary into another directory that already is.

### Linux

Use `ARCH=amd64` for x86-64 Linux and `ARCH=arm64` for ARM64 Linux.

```sh
VERSION=v0.1.0-alpha.2
ASSET_VERSION="${VERSION#v}"
OS=linux
ARCH=amd64
ARCHIVE="pg-contract_${ASSET_VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="pg-contract_${ASSET_VERSION}_checksums.txt"
BASE_URL="https://github.com/get-felipe/pg-contract/releases/download/${VERSION}"

curl -LO "${BASE_URL}/${ARCHIVE}"
curl -LO "${BASE_URL}/${CHECKSUMS}"

grep " ${ARCHIVE}$" "${CHECKSUMS}" | sha256sum -c -

tar -xzf "${ARCHIVE}"
mkdir -p "$HOME/.local/bin"
install -m 0755 pg-contract "$HOME/.local/bin/pg-contract"

pg-contract version
```

Make sure `$HOME/.local/bin` is on your `PATH`, or install the binary into another directory that already is.

### Windows PowerShell

Use `$Arch = "amd64"` for x86-64 Windows and `$Arch = "arm64"` for ARM64 Windows.

```powershell
$Version = "v0.1.0-alpha.2"
$AssetVersion = $Version.TrimStart("v")
$Arch = "amd64"
$Archive = "pg-contract_${AssetVersion}_windows_${Arch}.zip"
$Checksums = "pg-contract_${AssetVersion}_checksums.txt"
$BaseUrl = "https://github.com/get-felipe/pg-contract/releases/download/$Version"

Invoke-WebRequest "$BaseUrl/$Archive" -OutFile $Archive
Invoke-WebRequest "$BaseUrl/$Checksums" -OutFile $Checksums

$Pattern = " $([regex]::Escape($Archive))$"
$Expected = ((Select-String -Path $Checksums -Pattern $Pattern).Line -split "\s+")[0]
$Actual = (Get-FileHash $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($Actual -ne $Expected) {
  throw "checksum mismatch for $Archive"
}

Expand-Archive $Archive -DestinationPath . -Force

$InstallDir = Join-Path $env:LOCALAPPDATA "pg-contract\bin"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item ".\pg-contract.exe" (Join-Path $InstallDir "pg-contract.exe") -Force

& (Join-Path $InstallDir "pg-contract.exe") version
```

Add `%LOCALAPPDATA%\pg-contract\bin` to your `PATH` if you want to run `pg-contract` from any shell.

## From Source

Use this path when changing the project locally:

```sh
git clone https://github.com/get-felipe/pg-contract.git
cd pg-contract
make build
./bin/pg-contract version
```

## Uninstall

Remove the binary from the directory where it was installed. For example:

```sh
rm -f "$HOME/.local/bin/pg-contract"
```
