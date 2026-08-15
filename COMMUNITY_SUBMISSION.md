# Start9 Community Registry submission

This file tracks the handoff steps for publishing s/watcher through the Start9
Community Registry. The authoritative process is the
[StartOS publishing guide](https://docs.start9.com/packaging/0.4.0.x/publishing.html).

## Repository checklist

- [x] Public GitHub repository.
- [x] OSI-approved license and SPDX manifest identifier.
- [x] Current `README.md` covering image, volume, interfaces, actions, backups,
      health checks, dependencies, limitations, and AI quick reference.
- [x] User-focused `instructions.md` with first-run and action guidance.
- [x] Contribution and private security-reporting policies.
- [x] Automated Go, TypeScript, SDK, and multi-architecture container checks.
- [x] GitHub Actions green on the release commit.
- [x] Release tag matches `v{upstream_version}_{downstream_revision}`.

## Local release checks

- [x] `go test ./cmd/... ./internal/...`
- [x] `go vet ./cmd/... ./internal/...`
- [x] `npm ci`
- [x] `npm run check`
- [x] `node node_modules/@start9labs/start-sdk/lint.mjs`
- [x] `npm run build`
- [x] Multi-architecture container build succeeds.
- [x] `start-cli s9pk pack` succeeds.
- [x] The packed manifest has the intended version, git hash, license,
      architectures, dependencies, and repository URLs.

## StartOS smoke test

- [x] Fresh install succeeds.
- [x] Required Electrs dependency is resolved.
- [x] Service starts and the Web Interface health check becomes healthy.
- [x] Web password can be set and the Web UI accepts it.
- [x] A test address can be added and scanned through local Electrs.
- [x] Backup completes and restore preserves watches and notification settings.
- [x] Uninstall and reinstall complete without lifecycle errors.
- [x] Optional Mempool links work when Mempool is installed.
- [x] Telegram and NIP-17 test messages work when configured.

Do not mark tests complete unless they were run against the exact release
commit and package.

### Tested release

- Version: `0.1.1:6`
- Tag: `v0.1.1_6`
- Commit: `60688dfcf07f5fe91bf90fe152130c8f69b5ec53`
- Architectures: `aarch64`, `x86_64`
- Package SHA-256:
  `28c4e24e47d921528f15e21532b77605d9b99bedf11135c6aebbe156552412b9`
- Automated validation: [GitHub Actions run 31546005498](https://github.com/StellarStoic/swatcher/actions/runs/31546005498)
- StartOS smoke-test results: confirmed by the package maintainer on 2026-08-15.

## Dependency audit note

The latest official Start9 SDK currently bundles ESLint dependencies that npm
flags for `brace-expansion` and `js-yaml` denial-of-service advisories. They are
used by the SDK's development lint tool, are absent from the compiled
`javascript/index.js`, and are not installed in the s/watcher application
container. `npm audit fix` cannot replace bundled SDK dependencies; update the
SDK when Start9 publishes a release containing the upstream fixes.

## Initial submission

Email `submissions@start9.com` with a link to the public repository. Start9's
current process forks accepted submissions into the `Start9-Community` GitHub
organization. Subsequent release changes are proposed against that fork; merged
changes are built into `community-beta`, tested there, and promoted to the
production community registry after maintainer approval.

Suggested email:

```text
Subject: Start9 Community package submission — s/watcher

Hello Start9 team,

I would like to submit s/watcher for the Start9 Community Registry:
https://github.com/StellarStoic/swatcher

s/watcher is a watch-only Bitcoin address and wallet monitor that uses the
StartOS-local Electrs service and optionally sends Telegram or NIP-17 Nostr
notifications. The repository includes the application, StartOS package,
license, user instructions, automated checks, and release checklist.

The tagged package has been tested on StartOS as recorded in
COMMUNITY_SUBMISSION.md.

Thank you.
```
