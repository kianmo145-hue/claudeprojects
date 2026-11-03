#!/usr/bin/env bash
# Builds the Go core (../mobile) into app/libs/paydar.aar with gomobile.
# Needs Go, the Android SDK (ANDROID_HOME) and an NDK (ANDROID_NDK_HOME).
set -euo pipefail
cd "$(dirname "$0")/../.."
GOBIN="$PWD/.bin" go install golang.org/x/mobile/cmd/gomobile golang.org/x/mobile/cmd/gobind
export PATH="$PWD/.bin:$PATH"
targets="${GOMOBILE_TARGETS:-android/arm64,android/arm,android/amd64}"
tags="with_gvisor,with_quic,with_utls,badlinkname,tfogo_checklinkname0"
mkdir -p android/app/libs
gomobile bind -v -trimpath -tags "$tags" -ldflags="-s -w -checklinkname=0 -X main.version=${PAYDAR_VERSION:-dev}" \
  -target="$targets" -androidapi 26 -javapkg org.paydar.core \
  -o android/app/libs/paydar.aar ./mobile
