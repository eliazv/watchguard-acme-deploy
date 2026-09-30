# v0.1 MVP specification

## Product statement

`watchguard-acme-deploy` is a small open-source tool that takes an **already issued/renewed TLS certificate** and deploys it to supported cloud-managed WatchGuard Fireboxes through the official Firebox Management API.

It is not an ACME client and it is not a certificate-management platform. The binary is `wgcert`.

## Primary use case

A sysadmin already has a working ACME renewal process such as Certbot or acme.sh:

```text
certificate renewed
        |
        v
ACME deploy hook
        |
        v
wgcert deploy
        |
        v
WatchGuard API
        |
        v
cloud-managed Firebox
```

## Supported target

v0.1 targets only Fireboxes supported by WatchGuard's official certificate-management API:

- cloud-managed Firebox;
- associated with a WatchGuard Cloud account accessible through API credentials;
- Fireware version compatible with the Certificate API.

Locally-managed Fireboxes are explicitly out of scope for v0.1.

## Commands

### `wgcert devices`

Discover devices and make compatibility obvious before a user attempts a deployment.

```bash
wgcert devices
wgcert devices --include-hierarchy
wgcert devices --output json
```

`--include-hierarchy` is intended for WatchGuard Service Provider accounts and asks the Device Details API to include Subscriber-account devices. v0.1 treats this as discovery only; it is not a fleet deployment feature.

### `wgcert deploy`

Inspect the local certificate, reuse an identical remote certificate object when one already exists, request installation and verify the remote object after the request.

```bash
wgcert deploy \
  --device FB-12345 \
  --cert /etc/letsencrypt/live/firewall.example.com/fullchain.pem \
  --key /etc/letsencrypt/live/firewall.example.com/privkey.pem \
  --wait 2m
```

Required behavior:

1. parse certificate and private key;
2. verify that certificate and key match;
3. extract local fingerprint and metadata;
4. retrieve existing device certificate objects;
5. reuse an object with the same fingerprint or create one when required;
6. request installation on the target device;
7. optionally wait for the asynchronous install transaction with `--wait`;
8. retrieve certificate state again;
9. verify that the remote certificate object matches the local fingerprint;
10. optionally verify the certificate served by a named TLS endpoint;
11. return a meaningful exit code.

Useful flags currently include:

```text
--device
--cert
--key
--name
--dry-run
--output text|json
--wait 2m
--verify-host host:port
--deploy-config
```

`--deploy-config` requests a full configuration deployment. This may include **all pending changes on the device**, not only certificate-related changes, and must remain opt-in.

### `wgcert check`

Inspect current WatchGuard Cloud certificate inventory without modifying the device, with optional external TLS verification:

```bash
wgcert check --device FB-12345
wgcert check --device FB-12345 --verify-host firewall.example.com:443
```

API inventory and the certificate actually served by a Firebox service are different facts. `--verify-host` is the stronger end-to-end check where the intended service is reachable through TLS.

## Idempotency

Creation idempotency is a core requirement, but certificate inventory must not be confused with active service configuration.

The v0.1 rule is:

```text
if a remote certificate object has the local fingerprint:
    do not upload a duplicate certificate object
else:
    create the certificate object

request install
optionally wait for the asynchronous install transaction
re-read remote certificate state
verify fingerprint
optionally verify the served TLS endpoint
```

The tool may therefore issue another install request when the same certificate object already exists. Merely finding the object in WatchGuard Cloud does not prove that the intended management UI, proxy, VPN endpoint or other service is actively using it.

Identity must be based on the certificate fingerprint, not only its name or subject, because renewals commonly preserve hostname/subject while changing the certificate.

## Configuration

The initial configuration uses environment variables so Certbot/acme.sh hooks can call the binary non-interactively:

```text
WATCHGUARD_ACCOUNT_ID
WATCHGUARD_API_URL
WATCHGUARD_AUTH_URL
WATCHGUARD_API_KEY
WATCHGUARD_ACCESS_ID
WATCHGUARD_ACCESS_PASSWORD
```

A config file may be added only if real MSP/multi-device feedback justifies it.

Secrets must never be emitted in logs, JSON errors, stack traces or debug output.

## Architecture

Language: **Go**.

```text
cmd/
  wgcert/
internal/
  watchguard/
  certificate/
  config/
hooks/
  certbot/
  acme.sh/
testdata/
docs/
```

The WatchGuard client accepts an injectable HTTP base URL/transport so the integration can be exercised against an in-process fake server without owning a Firebox.

## Asynchronous operations

WatchGuard certificate installation and configuration deployment are transaction-based operations.

`--wait <duration>` polls the relevant transaction until it reaches a known successful or failed terminal state, or until the context timeout expires. Without `--wait`, an accepted request means only that WatchGuard accepted the command, not that the device completed it.

The install response shape must remain tolerant of documented response variations such as `device` being an array.

## Error behavior

CLI errors should be actionable. Important categories include:

- configuration missing;
- authentication rejected;
- unsupported locally-managed device;
- certificate/key invalid or mismatched;
- rate limited/API unavailable;
- certificate-name collision;
- install transaction failed/timed out;
- configuration deployment rejected/failed;
- remote fingerprint verification failed;
- external TLS verification failed.

Use distinct non-zero exit codes only if they materially improve scripting; avoid an unnecessarily complex taxonomy in v0.1.

## Security requirements

The tool handles private keys and privileged API credentials, so v0.1 must already enforce sensible behavior:

- never print private-key contents;
- never print access password or API key;
- redact bearer tokens;
- avoid persisting private keys or tokens;
- use TLS verification for WatchGuard API requests;
- support timeouts;
- never retry unsafe mutations blindly;
- make `--dry-run` genuinely mutation-free;
- do not claim an API inventory match proves an active service certificate.

## ACME integration

Do not implement certificate issuance.

The CLI can be invoked after renewal by Certbot or acme.sh using the scripts under `hooks/`. Automatic hooks should not enable `--deploy-config` until full-device deployment behavior has been validated on a lab Firebox.

## Out of scope

Do not implement in the initial timebox:

- dashboard or web UI;
- database/users/billing;
- hosted SaaS backend;
- scheduler;
- ACME protocol implementation;
- certificate authority functionality;
- email/Slack notifications;
- generic certificate inventory product;
- SSH/Expect support for locally-managed Fireboxes;
- batch/fleet deployment orchestration;
- FortiGate/Palo Alto/F5/other vendors.

## Development / validation order

### Phase 1 — offline core

- Go project and CLI;
- certificate parsing/fingerprint;
- configuration;
- mock WatchGuard server;
- unit and integration tests;
- dry-run.

### Phase 2 — API implementation

- OAuth;
- device discovery;
- Service Provider hierarchy discovery;
- certificate GET/create/install;
- asynchronous transaction polling;
- remote-object fingerprint verification;
- error handling.

### Phase 3 — verification

- `check`;
- TLS endpoint verification;
- realistic documented API response shapes;
- hooks and release artifacts.

### Phase 4 — real environment

- WatchGuard API credentials;
- FireboxV or real cloud-managed Firebox;
- create/install/wait flow;
- determine whether the intended Firebox service switches certificate automatically;
- renewal of the same hostname;
- verify configuration-reference behavior;
- fix any gap between production API behavior and documentation.

### Phase 5 — external validation

- request external beta testers;
- capture model/Fireware/use-case metadata without secrets;
- observe at least one genuine renewal cycle before using production-ready wording.

## Go / no-go condition

The most important question is not whether the API accepts an uploaded certificate. It is:

> Can a renewed certificate become active for the intended Firebox use case without a recurring manual configuration step?

If this cannot be automated reliably with the supported APIs, stop the experiment or sharply redefine its scope instead of building a large workaround platform.

## Timebox

Target: 3–5 focused development days.

Absolute initial timebox: approximately one week. Extend it only when real-device/API evidence reveals a small, tractable path to a useful release.
