#!/usr/bin/env bash

# Build executables for multiple platforms.
# https://www.digitalocean.com/community/tutorials/how-to-build-go-executables-for-multiple-platforms-on-ubuntu-16-04

package=$1
outpath=$2
ldflags=$3

if [[ "$package" = "help" ]]; then
    echo "usage: $0 <package-name> <output_path>"
    exit 1
fi

if [[ -z "$package" ]]; then
    echo "building package in working directory..."
    package=$(pwd)
fi

# grab the last segment
package_name=${package##*/}

set -e

platforms=("windows/amd64" "windows/386" "darwin/amd64" "linux/amd64" "linux/386")

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    GOARCH=${platform_split[1]}

    output_name=${package_name}'-'${GOOS}'-'${GOARCH}
    if [[ ${GOOS} = "windows" ]]; then
        output_name+='.exe'
    fi

    env GOOS=${GOOS} GOARCH=${GOARCH} /usr/local/go/bin/go build -i -ldflags "-X main.Path=./" -o ${outpath}/${output_name} ${package}
done



#   GOOS - Target Operating System	    GOARCH - Target Platform
#   android	                            arm
#   darwin	                            386
#   darwin	                            amd64
#   darwin	                            arm
#   darwin	                            arm64
#   dragonfly	                        amd64
#   freebsd	                            386
#   freebsd	                            amd64
#   freebsd	                            arm
#   linux	                            386
#   linux	                            amd64
#   linux	                            arm
#   linux	                            arm64
#   linux	                            ppc64
#   linux	                            ppc64le
#   linux	                            mips
#   linux	                            mipsle
#   linux	                            mips64
#   linux	                            mips64le
#   netbsd	                            386
#   netbsd	                            amd64
#   netbsd	                            arm
#   openbsd	                            386
#   openbsd	                            amd64
#   openbsd	                            arm
#   plan9	                            386
#   plan9	                            amd64
#   solaris	                            amd64
#   windows	                            386
#   windows	                            amd64

# Warning: Cross-compiling executables for Android requires the Android NDK,
# and some additional setup which is beyond the scope of this tutorial.
