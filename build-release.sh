#!/bin/bash

# build toc for release on all platforms

set -euo pipefail

archs=(amd64 arm64)
oses=(linux darwin windows)

for arch in ${archs[@]}; do
  for os in ${oses[@]}; do
    dir="releases/${arch}/${os}"
    mkdir -p "${dir}"
    env GOOS="${os}" GOARCH="${arch}" go build -o "${dir}/toc"
  done
done

echo -e "Built binaries:\n"
file releases/**/**/toc