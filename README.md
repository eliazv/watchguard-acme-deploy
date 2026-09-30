# WatchGuard ACME Deploy

Experimental open-source CLI for deploying already-issued TLS certificates to **cloud-managed WatchGuard Firebox** devices through the official WatchGuard Firebox Management API.

> [!IMPORTANT]
> This project is in an early validation phase and has **not yet been tested end-to-end against a real Firebox**. Do not use it in production until the real-device validation checklist is complete.

## Goal

Keep certificate issuance where it already belongs (Certbot, acme.sh, another ACME client or CA) and make WatchGuard deployment reliable and automatable.

Planned CLI:

```bash
wgcert devices
wgcert deploy --device FB-12345 --cert fullchain.pem --key privkey.pem
wgcert check --device FB-12345
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
fingerprint / optional TLS verification
```

## Scope

Initial scope is intentionally small:

- discover Firebox devices available through the WatchGuard API;
- reject or clearly identify unsupported locally-managed devices;
- parse PEM certificates and private keys locally;
- calculate certificate fingerprints and metadata;
- upload/create certificates through the official API;
- install them on selected cloud-managed Fireboxes;
- make deployment idempotent where possible;
- verify the resulting certificate metadata;
- optionally verify the certificate served by a hostname with a TLS handshake;
- provide Certbot and acme.sh deploy-hook examples.

Out of scope for v0.1: dashboard, database, scheduler, user accounts, billing, certificate issuance, SaaS hosting, SSH/Expect automation, and support for other firewall vendors.

## Current limitation

WatchGuard's Certificate API works only with **cloud-managed Fireboxes** whose configuration is stored in WatchGuard Cloud. Locally-managed Fireboxes are not part of the v0.1 target.

## Validation status

Most of the project can be developed and tested without owning a Firebox by using unit tests, generated X.509 fixtures, HTTP mock servers, API contract fixtures and dry-run behavior.

A real cloud-managed Firebox or FireboxV evaluation is still required before calling the deployment flow production-tested. See [docs/testing.md](docs/testing.md).

## Documentation

- [Research and opportunity validation](docs/research.md)
- [Testing without WatchGuard hardware](docs/testing.md)
- [v0.1 MVP specification](docs/mvp.md)

## Disclaimer

This is an unofficial community project. It is not affiliated with, endorsed by, or supported by WatchGuard Technologies.
