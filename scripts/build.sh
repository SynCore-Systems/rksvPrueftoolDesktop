#!/bin/bash
# Builds the Windows executables in the official Go Docker image (no local Go
# toolchain needed):
#   dist/public/RKSV-Prueftool.exe          JRE embedded, BMF engine downloaded at first start
#   dist/offline/rksvPrueftoolDesktop-Offline.exe
#                                           JRE and BMF engine embedded; not published until the
#                                           BMF permits redistribution (the engine has no license)
#   dist/linux/rksv-prueftool               headless test build (--serve), uses system java
#
# Usage: scripts/build.sh [VERSION]   (default: 0.0.0-dev)
set -euo pipefail

VERSION="${1:-0.0.0-dev}"
GO_IMAGE="golang:1.27.2"
GO_WINRES="github.com/tc-hib/go-winres@v0.3.3"

JRE_ZIP="assets/jre-windows-x64.zip"
JRE_URL="https://github.com/adoptium/temurin8-binaries/releases/download/jdk8u504-b01/OpenJDK8U-jre_x64_windows_hotspot_8u504b01.zip"
JRE_SHA256="82e2cdc6693737c5998445b31f69668fa0da77c7705121053f6508ac84961123"

BMF_ZIP="assets/regkassen-verification-1.1.1.zip"
BMF_URL="https://github.com/BMF-RKSV-Technik/at-registrierkassen-mustercode/releases/download/V1.1.1/regkassen-verification-1.1.1.zip"
BMF_SHA256="2c1d65b7b0024262e36ebbfba296c47773c68c7000a6f33c36f80b1215e414b6"

cd "$(dirname "$0")/.."

# fetch FILE URL SHA256: download unless a verified copy exists.
fetch() {
    local file="$1" url="$2" sha="$3"
    if [ -f "$file" ] && echo "$sha  $file" | sha256sum -c --quiet - 2>/dev/null; then
        return
    fi
    echo "Downloading $url"
    curl -fsSL --retry 3 -o "$file.part" "$url"
    if ! echo "$sha  $file.part" | sha256sum -c --quiet -; then
        rm -f "$file.part"
        echo "ERROR: checksum mismatch for $url" >&2
        exit 1
    fi
    mv "$file.part" "$file"
}

fetch "$JRE_ZIP" "$JRE_URL" "$JRE_SHA256"
fetch "$BMF_ZIP" "$BMF_URL" "$BMF_SHA256"

# Numeric x.y.z.0 for the Windows version resource.
WIN_VERSION="$(echo "$VERSION" | grep -oE '^[0-9]+\.[0-9]+\.[0-9]+' || echo 0.0.0).0"

mkdir -p dist/public dist/offline dist/linux .cache
docker run --rm -u "$(id -u):$(id -g)" \
    -e HOME=/tmp -e GOCACHE=/src/.cache/build -e GOMODCACHE=/src/.cache/mod -e GOFLAGS=-trimpath \
    -v "$PWD:/src" -w /src "$GO_IMAGE" bash -euc "
        go run $GO_WINRES make --in winres/winres.json --out cmd/rksv-prueftool/rsrc --arch amd64 \
            --file-version $WIN_VERSION --product-version $WIN_VERSION
        go vet ./...
        GOOS=windows GOARCH=amd64 go vet ./...
        go test ./...
        LDFLAGS='-s -w -X main.version=$VERSION'
        GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags \"-H windowsgui \$LDFLAGS\" \
            -o dist/public/RKSV-Prueftool.exe ./cmd/rksv-prueftool
        GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags bundled -ldflags \"-H windowsgui \$LDFLAGS\" \
            -o dist/offline/rksvPrueftoolDesktop-Offline.exe ./cmd/rksv-prueftool
        CGO_ENABLED=0 go build -ldflags \"\$LDFLAGS\" -o dist/linux/rksv-prueftool ./cmd/rksv-prueftool
    "

echo
ls -lh dist/public/RKSV-Prueftool.exe dist/offline/rksvPrueftoolDesktop-Offline.exe dist/linux/rksv-prueftool
sha256sum dist/public/RKSV-Prueftool.exe dist/offline/rksvPrueftoolDesktop-Offline.exe
