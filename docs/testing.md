# Testing without WatchGuard hardware

## Implemented offline checks

Run `go test ./...` and `go vet ./...`. The suite generates X.509 certificate/key pairs, checks mismatch detection and fingerprints, exercises the documented OAuth/device/certificate/install/deployment requests against `httptest.Server`, verifies that POST mutations are not retried after an HTTP 500 and that API errors do not include response bodies, and checks that CLI dry-run sends no mutation request.

The remaining high-value validation is on a cloud-managed Firebox. In particular, the install command is asynchronous, and the certificate inventory reports WatchGuard Cloud objects rather than the certificate served on a specific Firebox endpoint. Test both a real deployment transaction and `--verify-host` before claiming a renewal is fully automatic.

`--deploy-config` sends a full configuration deployment with `staged=false`. WatchGuard documents that this distributes all pending configuration changes for that device. Review those changes before using the flag.

This project can be developed to a high confidence level without initially owning a WatchGuard appliance, but it must distinguish between **software correctness** and **real-device validation**.

## What we can test without a Firebox

### 1. Certificate parsing

Generate temporary test certificates during tests and validate:

- PEM parsing;
- private-key parsing;
- certificate/private-key matching;
- subject and SAN extraction;
- validity dates;
- SHA-256 fingerprint generation;
- malformed PEM handling;
- encrypted/unsupported key behavior;
- expired and not-yet-valid certificates.

These tests require no WatchGuard dependency.

### 2. WatchGuard API client

The HTTP client should depend on an injectable base URL and transport so the test suite can use an in-process mock HTTP server.

Mock the WatchGuard endpoints and assert:

- OAuth/token requests use the expected credentials and headers;
- API calls send `Authorization: Bearer ...` and `WatchGuard-API-Key`;
- device-list responses are decoded correctly;
- cloud-managed vs locally-managed devices are handled correctly;
- certificate create/get/install requests use the expected JSON payloads;
- 401, 403, 404, 409, 429 and 5xx responses become useful CLI errors;
- token refresh works;
- retry behavior never causes unsafe duplicate deployment;
- secrets/private keys are not leaked into logs or error messages.

Official API references:

- https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/management.html
- https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/certificates_fireboxes.html

### 3. Contract fixtures

Store sanitized JSON fixtures that reflect the documented API response/request shapes. Tests should deserialize these fixtures so accidental client-model drift is caught early.

Do not claim contract fixtures prove that WatchGuard production behavior matches the docs. They prove only that our client matches the documented contract.

### 4. Idempotency logic

The core decision logic can be tested completely offline:

```text
local fingerprint == remote fingerprint
    -> no upload/install

local fingerprint != remote fingerprint
    -> deployment required
```

Cover edge cases such as:

- same subject but different key/certificate;
- same domain with renewed validity dates;
- duplicate names;
- missing remote certificate;
- multiple certificates returned for one device.

### 5. Dry-run mode

A planned `--dry-run` mode should parse configuration and certificates and show what would be done without sending mutation requests.

This is valuable both for tests and for first-time users.

Example:

```bash
wgcert deploy \
  --device FB-12345 \
  --cert ./testdata/fullchain.pem \
  --key ./testdata/privkey.pem \
  --dry-run
```

Expected output should make clear that no WatchGuard changes were made.

### 6. TLS verification

The optional external verification step can be tested against a local TLS server created by the Go test suite.

The test can start a TLS listener with a known generated certificate and verify that `wgcert check --verify-host ...` reads and compares the served certificate fingerprint correctly.

### 7. CLI integration tests

Build the binary and execute commands against the mock API server:

```text
wgcert devices
wgcert deploy ...
wgcert check ...
```

Assert exit codes, stdout/stderr and JSON output if a machine-readable mode is added.

## What we cannot honestly validate without a real Firebox

Mocks cannot prove:

- that WatchGuard accepts every payload exactly as documented;
- that an installed certificate becomes active in the expected Firebox configuration;
- how existing certificate references behave during renewal;
- how long configuration deployment takes;
- whether particular Fireware versions behave differently;
- whether the externally served certificate changes as expected after deployment.

These claims must remain marked unverified until tested against a real cloud-managed Firebox.

## Best path to real-device testing

The preferred lab is **FireboxV**, WatchGuard's virtual Firebox appliance, rather than buying physical hardware.

WatchGuard documents FireboxV evaluations under:

`Products > Virtual Appliance Evaluations`

Official trial documentation:

https://www.watchguard.com/help/docs/help-center/en-US/content/en-US/WG-Cloud/trials_enable-trial-licenses.html

Whether an evaluation can be self-activated depends on the WatchGuard account and its permissions/relationship. If we cannot obtain an evaluation directly, the fallback should be an external beta tester who already has a cloud-managed Firebox.

## Real-device validation checklist

Before v1.0 / production-ready wording, complete all of these on a disposable lab device or explicitly approved test device:

1. authenticate using dedicated API credentials;
2. discover the device through the API;
3. confirm it is reported as cloud-managed;
4. upload a non-production test certificate;
5. retrieve it and compare the API fingerprint;
6. install the certificate;
7. confirm configuration deployment succeeds;
8. confirm the certificate is usable in the intended Firebox configuration;
9. renew/replace it with another certificate for the same hostname;
10. verify whether existing configuration references continue to work;
11. repeat the same deployment and confirm idempotency;
12. verify the externally served certificate where applicable;
13. test rollback/error behavior;
14. remove test credentials and test certificates.

## Beta testing before owning a device

An OSS project can reach real hardware through volunteer testers before the maintainer owns a Firebox.

Once the mock-tested CLI is ready:

- publish a clearly marked pre-release;
- open a GitHub issue titled along the lines of `Looking for cloud-managed Firebox beta testers`;
- request only non-sensitive diagnostics;
- never ask users to paste API keys or private keys into issues;
- offer a `--debug` mode that redacts credentials and PEM/private-key material;
- ask testers to report Fireware version, device model, cloud-managed status, command result and sanitized error response.

This can produce the first real-device feedback with almost no infrastructure cost.

## Testing confidence model

A useful status ladder is:

- **Level 0 — design:** based only on documentation;
- **Level 1 — unit-tested:** certificate/config/domain logic tested offline;
- **Level 2 — mocked API:** complete CLI flows pass against an API simulator;
- **Level 3 — WatchGuard account/API:** authentication/discovery tested against the real cloud API;
- **Level 4 — FireboxV:** create/install/check tested on a virtual Firebox;
- **Level 5 — independent beta:** at least one external operator completes a real renewal;
- **Level 6 — production validated:** multiple independent installations survive real renewal cycles.

The README should state the current level rather than using vague claims such as "production ready".
