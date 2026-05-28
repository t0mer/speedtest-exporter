#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-dev}"
BINARY="speedtest-exporter"
LDFLAGS="-s -w -X main.version=${VERSION}"
OUT="dist"

mkdir -p "${OUT}"

targets=(
  "linux/amd64/"
  "linux/arm64/"
  "linux/arm/v7"
  "linux/arm/v6"
  "linux/386/"
  "darwin/amd64/"
  "darwin/arm64/"
  "windows/amd64/.exe"
  "windows/arm64/.exe"
)

for t in "${targets[@]}"; do
  OS="${t%%/*}"
  REST="${t#*/}"
  ARCH="${REST%%/*}"
  VARIANT_AND_EXT="${REST#*/}"
  VARIANT="${VARIANT_AND_EXT%%.*}"
  EXT=""
  if [[ "${VARIANT_AND_EXT}" == *"."* ]]; then
    EXT=".${VARIANT_AND_EXT##*.}"
  fi

  NAME="${BINARY}-${OS}-${ARCH}"
  [[ -n "${VARIANT}" ]] && NAME="${NAME}-${VARIANT}"
  NAME="${NAME}${EXT}"

  echo "Building ${NAME}..."
  GOARM_VAL="${VARIANT//v/}"
  CGO_ENABLED=0 GOOS="${OS}" GOARCH="${ARCH}" GOARM="${GOARM_VAL}" \
    go build -trimpath -ldflags="${LDFLAGS}" \
    -o "${OUT}/${NAME}" ./cmd/speedtest-exporter/
done

echo "Done. Artifacts in ${OUT}/"
