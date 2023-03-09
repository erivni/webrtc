#!/bin/sh

#
# DO NOT EDIT THIS FILE
#
# It is automatically copied from https://github.com/pion/.goassets repository.
#
# If you want to update the shared CI config, send a PR to
# https://github.com/pion/.goassets instead of this repository.
#

set -eu

SCRIPT_PATH="$(realpath "$(dirname "$0")")"
GOASSETS_PATH="${SCRIPT_PATH}/.goassets"

GOASSETS_REF=${GOASSETS_REF:-master}

if [ -d "${GOASSETS_PATH}" ]; then
  if ! git -C "${GOASSETS_PATH}" diff --exit-code; then
    echo "${GOASSETS_PATH} has uncommitted changes" >&2
    exit 1
  fi
  git -C "${GOASSETS_PATH}" fetch origin
  git -C "${GOASSETS_PATH}" checkout ${GOASSETS_REF}
  git -C "${GOASSETS_PATH}" reset --hard origin/${GOASSETS_REF}
else
  git clone -b ${GOASSETS_REF} https://github.com/pion/.goassets.git "${GOASSETS_PATH}"
fi
