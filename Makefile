.PHONY: all
all: build build-arm build-arm64

.PHONY: build
build:
	go build -o bin/ev_remapper

.PHONY: build-arm
# sudo apt-get install gcc-arm-linux-gnueabi g++-arm-linux-gnueabi
build-arm:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm CC=arm-linux-gnueabi-gcc go build -o bin/ev_remapper.arm

.PHONY: build-arm64
# sudo apt-get install gcc-aarch64-linux-gnu
build-arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc go build -o bin/ev_remapper.arm64
