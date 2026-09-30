# Testing without WatchGuard hardware

## Current validation level

Current project status is **Level 2 — mocked API**.

The offline suite covers generated X.509 certificate/key pairs, key mismatch detection, fingerprints, OAuth, device discovery, certificate create/list/install requests, documented asynchronous transaction response shapes, transaction polling, mutation retry safety, token refresh and CLI dry-run behavior.

The next meaningful milestone is not more mocked code. It is a cloud-managed Firebox or FireboxV test.

## What we can test without a Firebox

### 1. Certificate parsing

Generate temporary test certificates and validate:

- PEM and private-key parsing;
- certificate/private-key matching;
- subject/SAN/validity extraction;
- SHA-256 fingerprint generation;
- malformed or unsupported key behavior;
- expired and not-yet-valid certificates.

### 2. WatchGuard API client

The HTTP client uses an injectable base URL, so tests can run against `httptest.Server` and assert:

- OAuth/token authentication;
- `Authorization` and `WatchGuard-API-Key` headers;
- normal Firebox discovery;
- Service Provider `include_hierarchy` discovery;
- cloud-managed vs locally-managed rejection;
- certificate create/get/install payloads;
- install responses where `device` is an array;
- transaction polling from non-terminal to terminal state;
- 401 token refresh;
- useful HTTP error handling;
- unsafe POST mutations are not blindly retried;
- secrets/private keys are not leaked into errors.

Official references:

- https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/management.html
- https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/certificates_fireboxes.html
- https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/device_details.html
- https://www.watchguard.com/help/docs/API/Content/en-US/firebox/management/v1/deployments.html

### 3. Contract shapes

Mocks should reflect documented production shapes rather than convenient invented JSON. In particular, asynchronous command responses may contain a `device` array and deployments may return transaction arrays.

Passing these tests proves only that the client matches the documented contract. It does not prove WatchGuard production behavior or appliance behavior.

### 4. Creation idempotency

The part we can prove offline is certificate-object idempotency:

```text
same local and remote fingerprint
    -> reuse remote certificate object
    -> no duplicate upload

different fingerprint
    -> create a new certificate object
```

The tool may still request installation of an already-existing certificate object because cloud inventory does not prove which certificate an actual Firebox service is using.

Cover:

- same subject but renewed certificate;
- same domain with new validity dates;
- duplicate fingerprints;
- conflicting fixed names;
- missing remote object;
- multiple objects returned for one device.

### 5. Dry-run

`--dry-run` must parse local inputs, authenticate/read remote state and show the mutation plan while issuing **zero mutation requests**.

```bash
wgcert deploy \
  --device FB-12345 \
  --cert ./testdata/fullchain.pem \
  --key ./testdata/privkey.pem \
  --dry-run
```

### 6. Transaction waiting

`--wait 2m` should be tested with mocked transaction sequences such as:

```text
in_progress -> complete
pending -> failed
pending -> context deadline exceeded
```

The install transaction and optional configuration-deployment transaction are separate operations and should be surfaced separately in text/JSON output.

### 7. TLS verification

The optional external verification step can be exercised against a local TLS server using a known generated certificate.

`wgcert check --verify-host ...` should compare the actually served leaf-certificate fingerprint to WatchGuard inventory. During deploy, `--verify-host` should compare it to the local certificate intended for deployment.

### 8. CLI integration

Exercise:

```text
wgcert devices
wgcert devices --include-hierarchy
wgcert deploy --dry-run ...
wgcert deploy --wait ...
wgcert check ...
wgcert version
```

Assert exit behavior, text output and JSON output where applicable.

## What mocks cannot prove

Without a real Firebox we cannot honestly claim:

- WatchGuard production accepts every payload exactly as documented;
- the install transaction causes the intended appliance configuration to use the certificate;
- an existing service reference follows a renewed/replaced certificate automatically;
- a full configuration deployment is necessary for this workflow;
- the timing/terminal statuses observed in production match the documentation;
- Fireware versions behave identically;
- the externally served certificate changes as intended.

These remain explicit validation questions.

## Why `--deploy-config` is opt-in

`--deploy-config` requests a **full configuration deployment** with `staged=false`. WatchGuard documents that deployment as distributing pending device configuration. It may therefore include changes unrelated to wgcert.

Do not add this flag to unattended Certbot/acme.sh hooks until lab testing demonstrates that it is necessary and safe for the intended workflow.

## Best path to real-device testing

Prefer **FireboxV**, WatchGuard's virtual Firebox appliance, over purchasing physical hardware.

WatchGuard documents FireboxV evaluations under:

`Products > Virtual Appliance Evaluations`

Official trial documentation:

https://www.watchguard.com/help/docs/help-center/en-US/content/en-US/WG-Cloud/trials_enable-trial-licenses.html

Evaluation availability depends on account type and permissions. If we cannot obtain one directly, use an external beta tester who already manages a cloud-managed Firebox.

## Real-device validation checklist

Before production-ready wording, complete the following on a disposable lab device or explicitly approved test device:

1. authenticate using dedicated API credentials;
2. discover the Firebox through `wgcert devices`;
3. confirm `cloud_managed=yes`;
4. run a dry-run with a non-production test certificate;
5. create/reuse the remote certificate object;
6. request installation;
7. wait for and record the real install transaction statuses;
8. re-read the object and compare its fingerprint;
9. determine whether a separate configuration deployment is required;
10. if required, review pending changes before using `--deploy-config`;
11. verify the intended Firebox service is using the certificate;
12. repeat with a renewed certificate for the same hostname;
13. determine whether existing configuration references continue to work;
14. repeat the same certificate and confirm no duplicate certificate object is created;
15. verify the externally served certificate where applicable;
16. test one rejected/failed transaction and its error output;
17. remove test credentials and test certificates.

The critical product question is steps 11–13: can renewal become active without a recurring manual configuration step?

## External beta testing

Once the mock-tested prerelease is available:

- clearly label it as experimental;
- ask specifically for cloud-managed Firebox testers;
- request model, Fireware version, intended certificate use and sanitized command output;
- never request API keys, access passwords or private keys in issues;
- distinguish “API command accepted” from “certificate confirmed active on service”;
- collect whether the tester manages one Firebox or a multi-device/MSP estate.

One genuine completed renewal is more valuable than many stars or downloads.

## Confidence ladder

- **Level 0 — design:** documentation only;
- **Level 1 — unit-tested:** local certificate/config logic tested offline;
- **Level 2 — mocked API:** complete documented CLI/API flows exercised against mocks;
- **Level 3 — WatchGuard Cloud:** real authentication/discovery tested;
- **Level 4 — FireboxV:** create/install/wait/check tested on a virtual appliance;
- **Level 5 — independent beta:** an external operator completes a real renewal;
- **Level 6 — production validated:** multiple independent installations survive real renewal cycles.

Use the level in public documentation instead of vague “production ready” wording.
