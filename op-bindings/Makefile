SHELL := /usr/bin/env bash

pkg := bindings
monorepo-base := $(shell dirname $(realpath .))
contracts-dir := $(monorepo-base)/packages/contracts-bedrock
shutter-contracts-dir := $(monorepo-base)/packages/shop-contracts
shutter-bindings-dir := $(shutter-contracts-dir)/bindings

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

bindings-shutter: compile compile-shutter bindings-shutter-build

# XXX: revert: for now, use the artifacts with all contracts inside
bindings-shutter-build:
	mkdir ./tmp-artifacts
	cp -r  $(contracts-dir)/forge-artifacts/ tmp-artifacts
	cp -r  $(shutter-contracts-dir)/out/ tmp-artifacts
	go run ./gen/main.go \
		-forge-artifacts ./tmp-artifacts \
    -out ./bindings \
    -contracts ./artifacts-with-shutter.json \
		-source-maps MIPS,PreimageOracle \
		-package $(pkg) \
		-monorepo-base $(monorepo-base)
	rm -r ./tmp-artifacts

bindings-shutter-build-package:
	go run ./gen/main.go \
		-forge-artifacts $(shutter-contracts-dir)/out \
    -out $(shutter-bindings-dir) \
    -contracts $(shutter-contracts-dir)/artifacts.json \
		-package bindings

mkdir:
	mkdir -p $(pkg)

clean-contracts:
	cd $(contracts-dir) && \
		pnpm clean

clean:
	rm -rf $(pkg)

test:
	go test ./...
