#!/usr/bin/env bash

# Build all plugins for current platform.

set -e

for package in ./plugins/*/ ; do

  package_name="${package%%/}"; # trim trailing slash
  cd "$package_name"

  output_name="${package_name##*/}.so"; # remove everything before the last / that still remains

  `which go` build -trimpath -buildmode=plugin -o "$output_name"
#  cp "./$output_name" "../../artifacts/$output_name" && chmod 666 "../../artifacts/$output_name"

# build plugins for debug
  `which go` build -trimpath -buildmode=plugin -o "../../local/$output_name" -gcflags "all=-N -l"

  cd -

done
