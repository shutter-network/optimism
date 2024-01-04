#!/usr/bin/env bash

source ./common.sh

$DC up -d

echo "Bootstrapping shuttermint"

$DC run --rm --no-deps --entrypoint /rolling-shutter chain-0-validator op-bootstrap fetch-keyperset\
    --config /config/bootstrap.toml
$DC run --rm --no-deps --entrypoint /rolling-shutter chain-0-validator op-bootstrap \
    --config /config/bootstrap.toml

# TODO: init the keyperset contracts
# $DC stop -t 30
