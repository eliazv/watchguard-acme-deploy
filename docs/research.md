# Research and opportunity validation

_Last updated: 2026-09-30_

This document records why this project exists, what has been verified, and what could invalidate it.

## Problem

Public TLS certificate lifetimes are getting shorter, which makes manual renewal/deployment increasingly painful for appliances and firewall fleets.

WatchGuard users have repeatedly asked for native Let's Encrypt / ACME automation. The opportunity is not to build another ACME client: mature clients such as Certbot and acme.sh already solve issuance and renewal. The narrower gap is reliable deployment of renewed certificates to WatchGuard Fireboxes.

## Why this is technically interesting now

WatchGuard now exposes certificate management through the official Firebox Management API. The API supports creating certificates, retrieving certificate metadata, installing certificates on devices, and deleting certificates.

Relevant official documentation:

- Firebox Management API: https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/management.html
- Certificates for Cloud-Managed Fireboxes: https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/certificates_fireboxes.html
- Firebox API version history: https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/version_history.html

The certificate endpoints expose metadata useful for safe/idempotent automation, including certificate fingerprint, validity dates, issuer, subject and SANs.

## Important constraint: cloud-managed only

The official Certificate API works only with **cloud-managed Fireboxes** whose configuration is stored in WatchGuard Cloud.

WatchGuard explicitly states that these certificate operations do not work with locally-managed Fireboxes, including locally-managed devices that send reports to WatchGuard Cloud.

This project therefore intentionally targets cloud-managed Fireboxes first.

## Existing open-source project

An older project exists at:

https://github.com/watchguard-toolbox-project/watchguard-letsencrypt-deploy

It is an MIT-licensed Shell/Expect-based implementation originally created in 2019. It automates a different integration path and predates the current official certificate-management API.

We deliberately do **not** fork it because this project has a different architecture and compatibility target:

- Go CLI rather than Shell/Expect scripts;
- official WatchGuard Cloud API rather than interactive remote CLI automation;
- API-level device discovery and certificate metadata;
- explicit idempotency/fingerprint checks;
- testable HTTP client abstraction;
- cross-platform single-binary distribution.

The older project remains useful prior art and evidence that the manual deployment problem is real.

## User-demand evidence

WatchGuard community discussions have requested Let's Encrypt / ACME support for years. A useful starting point for ongoing validation is the WatchGuard Community feature discussion around native Let's Encrypt/ACME support.

Community discussions should be treated as demand signals, not product guarantees. The project should avoid depending on undocumented WatchGuard behavior whenever the official API can be used instead.

## Competitive framing

This should **not** become a certificate-management SaaS during the experiment.

The initial hypothesis is much narrower:

> A sysadmin or MSP already has Certbot/acme.sh/another CA workflow and needs a dependable deploy step for cloud-managed WatchGuard Fireboxes.

If users later ask for multi-account orchestration, audit trails, scheduling, inventory, notifications or additional vendors, those requests can inform a future product. They should not be built speculatively into v0.1.

## Main risks

### 1. Real deployment semantics

The biggest technical unknown is the exact end-to-end behavior on a real Firebox when a certificate is renewed.

The API clearly supports adding and installing certificates. We still need to validate whether replacing/renewing a certificate preserves the configuration references needed for the intended Web Server / proxy usage, or whether an additional configuration step is required.

This is the primary real-device go/no-go test.

### 2. Native WatchGuard ACME support

WatchGuard could eventually ship native ACME automation and reduce the need for this project. That is why the project is deliberately time-boxed and open source.

### 3. Limited addressable device set

The official API path excludes locally-managed Fireboxes. Supporting those devices would likely require a separate adapter and should not be added to v0.1 unless real users explicitly ask for it.

## Success criteria

GitHub stars are a weak signal for a niche infrastructure tool. Better validation signals are:

1. at least three independent operators successfully try the tool on real Fireboxes;
2. at least one operator keeps it installed through an actual certificate renewal;
3. real issues/PRs are opened by WatchGuard operators;
4. an MSP asks for fleet, subscriber or multi-account support;
5. an upstream deploy-hook contribution to an ACME ecosystem becomes feasible.

## Timebox

The initial experiment should stay within roughly **3-5 focused development days**, with an absolute stop around one week unless real-device validation produces a strong reason to continue.

If the basic deployment cannot be made reliable and fully automatable, stop rather than building workarounds, dashboards or a SaaS.
