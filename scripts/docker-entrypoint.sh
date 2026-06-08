#!/bin/sh
set -e
CONFIG="${GM_CONFIG:-/etc/giraffemail/config.yaml}"

giraffemail migrate --config "$CONFIG"
giraffemail seed --config "$CONFIG"
exec giraffemail serve --config "$CONFIG" "$@"
