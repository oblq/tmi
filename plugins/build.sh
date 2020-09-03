#!/usr/bin/env bash

# Build all plugins for current platform.

set -e

if [[ -z "$path" ]]; then
  echo "building package in working directory..."
  path="."
fi

for package in ./plugins/*/; do

  package_name="${package%%/}" # trim trailing slash
  cd "$package_name"

  output_name="${package_name##*/}" # remove everything before the last / that still remains

  go build -trimpath -buildmode=plugin -o "${path}/${output_name}.so"
  cp -n "./${output_name}.yaml" "${path}/${output_name}.yaml" 2>/dev/null || : # && chmod 666 $(path)/tmi.yaml;

  #  cp "./$output_name" "../../artifacts/$output_name" && chmod 666 "../../artifacts/$output_name"

  # build plugins for debug
  go build -trimpath -buildmode=plugin -o "../../local/$output_name" -gcflags "all=-N -l"

  cd -

done
