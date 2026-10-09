#!/bin/bash
sum="sha1sum"

if ! hash sha1sum 2>/dev/null; then
	if ! hash shasum 2>/dev/null; then
		echo "I can't see 'sha1sum' or 'shasum'"
		echo "Please install one of them!"
		exit
	fi
	sum="shasum"
fi

UPX=false
if hash upx 2>/dev/null; then
	UPX=true
fi

VERSION=`date -u +%Y%m%d`
LDFLAGS="-X main.VERSION=$VERSION -s -w"
GCFLAGS=""

OSES=(linux darwin windows freebsd)
ARCHS=(amd64 386)
for os in ${OSES[@]}; do
	for arch in ${ARCHS[@]}; do
		# Skip darwin/386 as it's no longer supported in newer Go versions
		if [ "$os" == "darwin" ] && [ "$arch" == "386" ]; then
			continue
		fi
		suffix=""
		ldflags="$LDFLAGS"
		if [ "$os" == "windows" ]
		then
			suffix=".exe"
			# Use -H windowsgui to prevent console window popup when running as subprocess
			ldflags="$ldflags -H windowsgui"
		fi
		env CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags "$ldflags" -gcflags "$GCFLAGS" -o client_${os}_${arch}${suffix} ./src/main
		if $UPX; then upx -9 client_${os}_${arch}${suffix};fi
		tar -zcf lanproxy-client-${os}-${arch}-$VERSION.tar.gz client_${os}_${arch}${suffix}
		$sum lanproxy-client-${os}-${arch}-$VERSION.tar.gz
	done
done

# Build for Linux ARM64 (aarch64)
env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o client_linux_arm64 ./src/main
if $UPX; then upx -9 client_linux_arm64;fi
tar -zcf lanproxy-client-linux-arm64-$VERSION.tar.gz client_linux_arm64
$sum lanproxy-client-linux-arm64-$VERSION.tar.gz

# Build for Apple Silicon (darwin arm64)
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o client_darwin_arm64 ./src/main
if $UPX; then upx -9 client_darwin_arm64;fi
tar -zcf lanproxy-client-darwin-arm64-$VERSION.tar.gz client_darwin_arm64
$sum lanproxy-client-darwin-arm64-$VERSION.tar.gz

# Create universal binary for macOS (amd64 + arm64)
# Need to rebuild uncompressed versions for lipo
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o client_darwin_amd64_universal ./src/main
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o client_darwin_arm64_universal ./src/main
lipo -create -output client_darwin_universal client_darwin_amd64_universal client_darwin_arm64_universal
if $UPX; then upx -9 client_darwin_universal;fi
tar -zcf lanproxy-client-darwin-universal-$VERSION.tar.gz client_darwin_universal
$sum lanproxy-client-darwin-universal-$VERSION.tar.gz
rm -f client_darwin_amd64_universal client_darwin_arm64_universal

# ARM
ARMS=(5 6 7)
for v in ${ARMS[@]}; do
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=$v go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o client_linux_arm$v  ./src/main
done
if $UPX; then upx -9 client_linux_arm* server_linux_arm*;fi
tar -zcf lanproxy-client-linux-arm-$VERSION.tar.gz client_linux_arm*
$sum lanproxy-client-linux-arm-$VERSION.tar.gz

#MIPS32LE
env CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o client_linux_mipsle ./src/main
env CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o client_linux_mips ./src/main

if $UPX; then upx -9 client_linux_mips* server_linux_mips*;fi
tar -zcf lanproxy-client-linux-mipsle-$VERSION.tar.gz client_linux_mipsle
tar -zcf lanproxy-client-linux-mips-$VERSION.tar.gz client_linux_mips
$sum lanproxy-client-linux-mipsle-$VERSION.tar.gz
$sum lanproxy-client-linux-mips-$VERSION.tar.gz
