#!/usr/bin/env sh

set -e

deadcode -test ./... || true
