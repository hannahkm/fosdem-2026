#!/usr/bin/env bash
set -eu

export GOMAXPROCS="${workers:-1}"

exec ./main "$1"