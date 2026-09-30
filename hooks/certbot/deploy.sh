#!/bin/sh
# Certbot calls this only after a successful renewal.
set -eu
: "${RENEWED_LINEAGE:?Certbot must set RENEWED_LINEAGE}"
: "${WGCERT_DEVICE:?set WGCERT_DEVICE}"
exec wgcert deploy --device "$WGCERT_DEVICE" --cert "$RENEWED_LINEAGE/fullchain.pem" --key "$RENEWED_LINEAGE/privkey.pem" "$@"
