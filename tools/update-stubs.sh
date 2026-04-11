#!/usr/bin/env bash
set -euo pipefail

STUB_VERSION=v0.5.1
CHIPS=(esp8266 esp32 esp32s2 esp32s3 esp32c2 esp32c3 esp32c5 esp32c6 esp32h2)
BASE_URL=https://github.com/espressif/esp-flasher-stub/releases/download/${STUB_VERSION}
STUB_DIR=$(cd "$(dirname "$0")/.." && pwd)/pkg/espflasher/stubs

echo "Downloading stubs from ${BASE_URL} to ${STUB_DIR}"

for chip in "${CHIPS[@]}"; do
  curl -sL -o "${STUB_DIR}/${chip}.json" "${BASE_URL}/${chip}.json"
done

echo "Successfully updated all ${#CHIPS[@]} stubs to ${STUB_VERSION}"
