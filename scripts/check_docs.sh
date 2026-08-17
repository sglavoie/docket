#!/usr/bin/env sh

set -eu

go test -count=1 . -run 'TestREADME|TestArchitecture'
