#!/usr/bin/env bash
set -euo pipefail

version=${1:?usage: build-release.sh vX.Y.Z}
mkdir -p dist

build_one() {
  local os=$1 arch=$2 ext='' name
  name="wgcert_${version}_${os}_${arch}"
  [[ $os == windows ]] && ext='.exe'
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "dist/wgcert${ext}" ./cmd/wgcert
  if [[ $os == windows ]]; then
    (cd dist && zip -q -j "${name}.zip" "wgcert${ext}" ../README.md ../LICENSE)
  else
    tar -czf "dist/${name}.tar.gz" -C dist "wgcert${ext}" -C .. README.md LICENSE
  fi
  rm "dist/wgcert${ext}"
}

build_one linux amd64
build_one linux arm64
build_one darwin amd64
build_one darwin arm64
build_one windows amd64
(cd dist && sha256sum *.tar.gz *.zip > SHA256SUMS)
