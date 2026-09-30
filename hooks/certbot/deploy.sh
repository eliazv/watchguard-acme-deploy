#!/bin/sh
# Certbot calls this only after a successful renewal.
set -eu
: "${RENEWED_LINEAGE:?Certbot must set RENEWED_LINEAGE}"
: "${WGCERT_DEVICE:?set WGCERT_DEVICE}"

# An unattended renewal should not report success merely because WatchGuard
# accepted the asynchronous install request. Override WGCERT_WAIT if needed.
WGCERT_WAIT=${WGCERT_WAIT:-2m}

exec wgcert deploy \
  --device "$WGCERT_DEVICE" \
  --cert "$RENEWED_LINEAGE/fullchain.pem" \
  --key "$RENEWED_LINEAGE/privkey.pem" \
  --wait "$WGCERT_WAIT" \
  "$@"
