#!/bin/bash

# build toc for release on all platforms

set -euo pipefail

archs=(amd64 arm64)
oses=(linux darwin windows)

for arch in ${archs[@]}; do
  for os in ${oses[@]}; do
    env GOOS="${os}" GOARCH="${arch}" go build -o "binaries/toc-${os}-${arch}"
  done
done

echo -e "Built binaries:\n"
file binaries/*