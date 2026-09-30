# WatchGuard ACME Deploy (`wgcert`)

[Project page](https://eliazv.github.io/watchguard-acme-deploy/) · [Step-by-step guide](https://eliazv.github.io/watchguard-acme-deploy/guide.html) · [v0.1.0 prerelease](https://github.com/eliazv/watchguard-acme-deploy/releases/tag/v0.1.0) · [Help test on a Firebox](https://github.com/eliazv/watchguard-acme-deploy/issues/new/choose)

![Illustrative wgcert dry-run terminal output](site/terminal.svg)

Open-source Go CLI for deploying already-issued TLS certificates to **cloud-managed WatchGuard Firebox** devices through the official WatchGuard Firebox Management API.

> [!IMPORTANT]
> This project is in an early validation phase and has **not yet been tested end-to-end against a real Firebox**. Do not use it in production until the real-device validation checklist is complete.

## Quick Start

1. Download the [v0.1.0 prerelease](https://github.com/eliazv/watchguard-acme-deploy/releases/tag/v0.1.0) for your platform and check it against `SHA256SUMS`, or [build from source](#build-from-source).
2. Set the WatchGuard Cloud environment variables listed in [Configuration](#configuration).
3. Discover a device and run a mutation-free plan:

```sh
wgcert devices
wgcert deploy --device FB-12345 --cert fullchain.pem --key privkey.pem --dry-run
```

4. On a disposable cloud-managed Firebox, request installation and verify the intended service:

```sh
wgcert deploy --device FB-12345 --cert fullchain.pem --key privkey.pem --wait 2m
wgcert check --device FB-12345 --verify-host firewall.example.com:443
```

`--wait` waits for the asynchronous certificate-install transaction. If you also pass `--deploy-config`, wgcert requests a full device configuration deployment and waits for that transaction too. A full configuration deployment may include **all pending changes** for the device, so review them first.

## Goal

Keep certificate issuance where it already belongs (Certbot, acme.sh, another ACME client or CA) and make WatchGuard deployment reliable and automatable.

CLI:

```bash
wgcert devices
wgcert devices --include-hierarchy
wgcert deploy --device FB-12345 --cert fullchain.pem --key privkey.pem --dry-run
wgcert deploy --device FB-12345 --cert fullchain.pem --key privkey.pem --wait 2m
wgcert check --device FB-12345
wgcert version
```

The intended flow is:

```text
ACME client / CA
      |
      v
renewed certificate
      |
      v
    wgcert
      |
      v
WatchGuard Firebox Management API
      |
      v
cloud-managed Firebox
      |
      v
remote fingerprint / optional TLS verification
```

## Scope

Initial scope is intentionally small:

- discover Firebox devices available through the WatchGuard API;
- optionally include Subscriber-account devices for WatchGuard Service Provider accounts;
- reject unsupported locally-managed devices;
- parse PEM certificates and private keys locally;
- calculate certificate fingerprints and metadata;
- upload/create certificates through the official API;
- install them on selected cloud-managed Fireboxes;
- wait for asynchronous install/deployment transactions when requested;
- make certificate creation idempotent where possible;
- re-read the remote certificate object and verify its fingerprint;
- optionally verify the certificate served by a hostname with a TLS handshake;
- provide Certbot and acme.sh deploy-hook examples.

Out of scope for v0.1: dashboard, database, scheduler, user accounts, billing, certificate issuance, SaaS hosting, SSH/Expect automation, and support for other firewall vendors.

## Current limitation

WatchGuard's Certificate API works only with **cloud-managed Fireboxes** whose configuration is stored in WatchGuard Cloud. Locally-managed Fireboxes are not part of the v0.1 target.

A matching certificate object in WatchGuard Cloud still does **not** prove that the intended management UI, proxy, VPN or other Firebox service is actively serving it. Use `--verify-host` where possible and validate the real service on a lab Firebox before unattended production use.

## Service Provider discovery

WatchGuard Service Provider accounts can ask the Device Details API to include devices from Subscriber accounts. `wgcert` exposes that discovery mode as:

```sh
wgcert devices --include-hierarchy
```

This is discovery only in v0.1. It does not yet implement a fleet configuration file or multi-account batch deployment workflow.

## Validation status

Most of the project can be developed and tested without owning a Firebox by using unit tests, generated X.509 fixtures, HTTP mock servers, API contract fixtures and dry-run behavior.

A real cloud-managed Firebox or FireboxV evaluation is still required before calling the deployment flow production-tested. See [docs/testing.md](docs/testing.md).

## Documentation

- [Research and opportunity validation](docs/research.md)
- [Testing without WatchGuard hardware](docs/testing.md)
- [v0.1 MVP specification](docs/mvp.md)

## Build from source

Requires Go 1.22 or later. From the repository root:

```sh
go test ./...
go build -o wgcert ./cmd/wgcert
./wgcert version
```

Release builds embed their tag in `wgcert version`; local builds report `dev` unless a version is injected with `-ldflags`.

## Configuration

Set these environment variables with the values from WatchGuard Cloud **Administration > Managed Access**:

```text
WATCHGUARD_ACCOUNT_ID
WATCHGUARD_API_URL             # regional base API URL, e.g. https://api.usa.cloud.watchguard.com
WATCHGUARD_AUTH_URL            # regional Authentication API URL, with or without /oauth/token
WATCHGUARD_API_KEY
WATCHGUARD_ACCESS_ID           # read-write ID for deploy
WATCHGUARD_ACCESS_PASSWORD
```

Use read-only credentials for `devices` and `check` if available. Avoid shell history and process arguments for secrets; the CLI reads them only from the environment. Keep hook files and environment configuration readable only by the service account.

## Deploy and inspect

```sh
wgcert devices --output json
wgcert devices --include-hierarchy --output json
wgcert deploy --device FB-12345 --cert fullchain.pem --key privkey.pem --dry-run
wgcert deploy --device FB-12345 --cert fullchain.pem --key privkey.pem --wait 2m --output json
wgcert check --device FB-12345 --output json
wgcert check --device FB-12345 --verify-host firewall.example.com:443
```

The generated certificate name includes a SHA-256 fingerprint prefix. Repeating a deploy reuses the matching remote certificate instead of uploading a duplicate. The install command may still be sent again because certificate inventory does not prove which certificate is active. Do not use a fixed `--name` across renewals unless you manage name collisions yourself.

After WatchGuard accepts the install command, wgcert re-reads the remote certificate object and verifies that its fingerprint matches the local certificate. With `--wait`, it also polls the install transaction until it completes or fails. Neither check alone establishes that the intended Firebox service switched certificates; `--verify-host` performs a verified TLS handshake against the endpoint you name.

To request a full configuration deployment, add `--deploy-config`. This deploys **all pending configuration changes for the device**, not just the certificate. When combined with `--wait`, wgcert also waits for the configuration-deployment transaction.

## ACME hooks

`hooks/certbot/deploy.sh` uses Certbot's `RENEWED_LINEAGE`. Set `WGCERT_DEVICE` and the WatchGuard variables in the hook environment, then install the executable hook in Certbot's deploy-hook directory. `hooks/acme.sh/reload.sh` is for acme.sh's `--install-cert` workflow: set `WGCERT_CERT` and `WGCERT_KEY` to the installed fullchain and key paths, and run the script through `--reloadcmd`. Both scripts call `wgcert deploy` only after ACME has produced files.

Do not add `--deploy-config` to unattended hooks until you have reviewed the device's pending configuration behavior and validated a complete renewal cycle on a lab Firebox.

## Validation level and feedback

**Level 2: mocked API.** Unit, CLI, TLS, and mock HTTP tests cover the documented API flow, including asynchronous transaction shapes. Real WatchGuard Cloud credentials and a cloud-managed Firebox are needed for the next validation levels. See [the testing plan](docs/testing.md).

If you can test on a real Firebox, please use the [Firebox test issue template](https://github.com/eliazv/watchguard-acme-deploy/issues/new/choose) and report the model, Fireware version, management mode, intended certificate use, and sanitized result. Never post private keys or API credentials. See [SECURITY.md](SECURITY.md) for private vulnerability reporting.

## Disclaimer

This is an unofficial community project. It is not affiliated with, endorsed by, or supported by WatchGuard Technologies.
