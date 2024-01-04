#!/usr/bin/env bash

source ./common.sh

echo "Starting entire system"
# TODO: add --profile dev for metrics etc.,
# once the dockerfile is added
$DC  up -d
sleep 5
$DC ps
