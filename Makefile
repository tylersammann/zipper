SHELL=/bin/bash

BUILD_DIR=build
PROJECT_NAME=zipper

.PHONY: build, clean, generate-static, run

clean:
	rm -rf $(BUILD_DIR)

build: generate-static
	chmod +x build.sh
	mkdir -p $(BUILD_DIR)
	BUILD_DIR=$(BUILD_DIR) PROJECT_NAME=$(PROJECT_NAME) ./build.sh "linux/amd64" "darwin/amd64" "windows/amd64"

generate-static:
	go run -tags generate generate.go

run: generate-static
	go run main.go