#!/bin/sh
# Use with acme.sh --install-cert --fullchain-file/--key-file/--reloadcmd.
set -eu
: "${WGCERT_DEVICE:?set WGCERT_DEVICE}"
: "${WGCERT_CERT:?set WGCERT_CERT to installed fullchain path}"
: "${WGCERT_KEY:?set WGCERT_KEY to installed key path}"
exec wgcert deploy --device "$WGCERT_DEVICE" --cert "$WGCERT_CERT" --key "$WGCERT_KEY" "$@"
