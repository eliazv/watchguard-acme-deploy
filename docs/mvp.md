# v0.1 MVP specification

## Product statement

`watchguard-acme-deploy` is a small open-source tool that takes an **already issued/renewed TLS certificate** and deploys it to one or more supported cloud-managed WatchGuard Fireboxes through the official Firebox Management API.

It is not an ACME client and it is not a certificate-management platform.

The binary name is planned as `wgcert`.

## Primary use case

A sysadmin already has a working ACME renewal process, for example Certbot or acme.sh:

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
Firebox
```

## Supported target

v0.1 targets only Fireboxes supported by WatchGuard's official certificate-management API:

- cloud-managed Firebox;
- associated with a WatchGuard Cloud account accessible through API credentials;
- Fireware version compatible with the Certificate API.

Locally-managed Fireboxes are explicitly out of scope for v0.1.

## Commands

### `wgcert devices`

Purpose: discover available devices and make compatibility obvious before a user attempts a deployment.

Example:

```bash
wgcert devices
```

Suggested output:

```text
ID          NAME              MODEL   FIRMWARE   MANAGEMENT   STATUS
FB-12345    HQ-Firebox        M390    12.x       cloud        online
FB-67890    Branch-Firebox    T80     12.x       local        online
```

Machine-readable output may be added as:

```bash
wgcert devices --output json
```

### `wgcert deploy`

Purpose: inspect the local certificate, compare it with the current remote state, and deploy only when required.

Example:

```bash
wgcert deploy \
  --device FB-12345 \
  --cert /etc/letsencrypt/live/firewall.example.com/fullchain.pem \
  --key /etc/letsencrypt/live/firewall.example.com/privkey.pem
```

Required v0.1 behavior:

1. parse certificate and private key;
2. verify that certificate and key match;
3. extract local fingerprint and metadata;
4. retrieve existing device certificates;
5. decide whether deployment is required;
6. create/upload the certificate if required;
7. install it to the target device;
8. retrieve state again;
9. verify the resulting fingerprint;
10. return a meaningful exit code.

Useful flags:

```text
--device
--cert
--key
--name
--dry-run
--output text|json
--verbose
```

Do not add many options until a real integration requires them.

### `wgcert check`

Purpose: inspect current certificate state without modifying the device.

Example:

```bash
wgcert check --device FB-12345
```

Optional external TLS verification:

```bash
wgcert check \
  --device FB-12345 \
  --verify-host firewall.example.com:443
```

The external TLS check is useful because API state and the certificate actually served on a network endpoint are two different things.

## Idempotency

Idempotency is a core requirement, not a later enhancement.

Simplified rule:

```text
if local fingerprint == installed fingerprint:
    exit successfully without mutation
else:
    deploy and verify
```

The implementation must not use only certificate names or subjects as identity because a renewal normally preserves hostname/subject while changing the certificate itself.

## Configuration

Initial configuration should work with environment variables so Certbot/acme.sh hooks can call the binary non-interactively.

Proposed names:

```text
WATCHGUARD_ACCOUNT_ID
WATCHGUARD_API_URL
WATCHGUARD_AUTH_URL
WATCHGUARD_API_KEY
WATCHGUARD_ACCESS_ID
WATCHGUARD_ACCESS_PASSWORD
```

A small config file can be added if it genuinely improves MSP/multi-device usage, but it is not required for the first usable CLI.

Secrets must never be emitted in logs, JSON errors, stack traces or debug output.

## Architecture

Recommended language: **Go**.

Suggested structure:

```text
cmd/
  wgcert/
    main.go
internal/
  watchguard/
    auth.go
    client.go
    devices.go
    certificates.go
  certificate/
    parse.go
    fingerprint.go
    verify.go
  config/
    config.go
  output/
    output.go
hooks/
  certbot/
  acme.sh/
testdata/
docs/
```

Important implementation property: the WatchGuard client must accept an injectable HTTP base URL/transport so almost the entire integration can be tested against an in-process fake server.

## Error behavior

CLI errors should be actionable.

Prefer:

```text
Error: device FB-12345 is locally managed.
The WatchGuard Certificate API supports cloud-managed Fireboxes only.
```

rather than:

```text
HTTP 200: empty response
```

Important categories:

- configuration missing;
- authentication rejected;
- unsupported device;
- certificate/key invalid;
- certificate/key mismatch;
- rate limited;
- API unavailable;
- deployment rejected;
- verification failed.

Use distinct non-zero exit codes only if they improve scripting; avoid an unnecessarily complex exit-code taxonomy in v0.1.

## Security requirements

The tool handles private keys and privileged API credentials, so v0.1 should already enforce sensible behavior:

- never print private-key contents;
- never print access password or API key;
- redact bearer tokens;
- avoid persisting private keys or tokens;
- use TLS verification for WatchGuard API requests;
- support timeouts;
- avoid retrying unsafe mutations blindly;
- make `--dry-run` genuinely mutation-free.

## ACME integration

Do not implement certificate issuance.

After the CLI works independently, document a Certbot deploy hook and an acme.sh deploy hook.

Conceptually:

```bash
certbot renew --deploy-hook "/usr/local/bin/wgcert deploy ..."
```

Exact hook examples should be tested before being presented as copy/paste production commands.

## Out of scope

Do not implement in the initial timebox:

- dashboard;
- web UI;
- database;
- users;
- SaaS backend;
- billing;
- scheduler;
- ACME protocol implementation;
- certificate authority functionality;
- email/Slack notifications;
- full certificate inventory product;
- SSH/Expect support for locally-managed Fireboxes;
- FortiGate/Palo Alto/F5/other vendors.

## Development order

### Phase 1 — offline core

- Go project;
- config;
- certificate parsing/fingerprint;
- CLI skeleton;
- mock WatchGuard server;
- tests.

### Phase 2 — API implementation

- OAuth;
- device discovery;
- certificate GET/create/install;
- idempotency;
- dry-run;
- error handling.

### Phase 3 — verification

- `check`;
- optional TLS endpoint verification;
- realistic API fixtures;
- integration tests against fake server.

### Phase 4 — real environment

- WatchGuard API credentials;
- FireboxV or real cloud-managed Firebox;
- end-to-end renewal test;
- fix API/documentation differences.

### Phase 5 — release experiment

- Certbot hook;
- acme.sh hook;
- binaries for common platforms;
- `v0.1.0` pre-release;
- request external beta testers.

## Go / no-go condition

The most important question is not whether the API accepts an uploaded certificate. It is:

> Can a renewed certificate become active for the intended Firebox use case without a recurring manual configuration step?

If this cannot be automated reliably with the supported APIs, stop the experiment or sharply redefine its scope instead of building a large workaround platform.

## Timebox

Target: 3-5 focused development days.

Absolute initial timebox: approximately one week.

The timebox can be extended only when real-device/API evidence reveals a small, tractable path to a useful release.
