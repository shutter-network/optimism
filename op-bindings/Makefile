SHELL := /usr/bin/env bash

pkg := bindings
pkg-shutter := shutter-bindings
monorepo-base := $(shell dirname $(realpath .))
contracts-dir := $(monorepo-base)/packages/contracts-bedrock
shutter-contracts-dir := $(monorepo-base)/packages/shop-contracts

all: version mkdir bindings

version:
	forge --version
	abigen --version

compile:
	cd $(contracts-dir) && \
		forge clean && \
		pnpm build

bindings: compile bindings-build bindings-shutter

bindings-build:
	go run ./gen/main.go \
		-forge-artifacts $(contracts-dir)/forge-artifacts \
		-out ./bindings \
		-contracts ./artifacts.json \
		-source-maps MIPS,PreimageOracle \
		-package $(pkg) \
		-monorepo-base $(monorepo-base)


compile-shutter:
	cd $(shutter-contracts-dir) && \
		forge clean && \
		forge build

bindings-shutter: compile-shutter bindings-shutter-build

bindings-shutter-build:
	go run ./gen/main.go \
		-forge-artifacts $(shutter-contracts-dir)/forge-artifacts \
    -out ./bindings \
    -contracts ./artifacts-shutter.json \
		-package $(pkg) \
		-monorepo-base $(monorepo-base)

mkdir:
	mkdir -p $(pkg)

clean-contracts:
	cd $(contracts-dir) && \
		pnpm clean

clean:
	rm -rf $(pkg)

test:
	go test ./...
