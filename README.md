<p align="center">
  <img src="icon.svg" alt="s/watcher logo" width="21%" />
</p>

# s/watcher on StartOS

> **Project and source:** <https://github.com/StellarStoic/swatcher>

s/watcher is a watch-only Bitcoin activity monitor built for StartOS. It uses
the Electrs service on the same server to monitor mainnet addresses, public
extended keys, and supported output descriptors without sending blockchain
queries to a public API. Optional alerts are delivered through Telegram or
NIP-17 encrypted Nostr messages.

This repository contains both the Go application and its StartOS package. The
package is licensed under `AGPL-3.0-or-later`; see [LICENSE](LICENSE).

## Table of contents

- [Image and container runtime](#image-and-container-runtime)
- [Volume and data layout](#volume-and-data-layout)
- [Installation and first-run flow](#installation-and-first-run-flow)
- [Configuration management](#configuration-management)
- [Network access and interfaces](#network-access-and-interfaces)
- [Actions](#actions-startos-ui)
- [Backups and restore](#backups-and-restore)
- [Health checks](#health-checks)
- [Dependencies](#dependencies)
- [Bitcoin monitoring behavior](#bitcoin-monitoring-behavior)
- [Notifications](#notifications)
- [Security and privacy](#security-and-privacy)
- [Limitations and differences](#limitations-and-differences)
- [Development and contributing](#development-and-contributing)
- [Quick reference](#quick-reference-for-ai-consumers)

## Image and container runtime

- The image is built from the repository's [Dockerfile](Dockerfile).
- A statically linked Go binary is compiled in a builder stage and copied into
  a minimal Alpine runtime with `su-exec`.
- The StartOS daemon briefly starts as root only to repair ownership of the
  persistent volume, then executes s/watcher as the unprivileged `swatcher`
  user.
- Supported architectures are `x86_64` and `aarch64`.
- The service listens on container port `8080` over HTTP. StartOS provides the
  user-facing LAN, Tor, and custom-domain routing.

## Volume and data layout

The StartOS volume named `main` is mounted read-write at `/data`. Its important
files are:

| Path                       | Purpose                                                                                        |
| -------------------------- | ---------------------------------------------------------------------------------------------- |
| `/data/state.json`         | Watches, derived addresses, activity, notification delivery state, privacy settings, and theme |
| `/data/auth.json`          | Argon2id password verifier and session-invalidating authentication state                       |
| `/data/notifications.json` | Telegram configuration and the generated Nostr sender identity                                 |

No wallet private keys or seed phrases are accepted. The Nostr sender key is an
application identity generated solely for notifications; it is not a Bitcoin
key.

## Installation and first-run flow

StartOS installs the required Electrs dependency and exposes the Web UI. A new
installation has no default web password, so run **Set Web Password** before
opening the interface. Add an address, extended public key, or supported
descriptor in the Web UI; the first local Electrs scan establishes historical
state without sending old activity as a new alert.

The application does not have a separate upstream setup wizard. StartOS owns
dependency discovery, interface routing, persistence, backups, and service
lifecycle.

## Configuration management

| StartOS-managed setting               | Source                  |
| ------------------------------------- | ----------------------- |
| Web listen address                    | `SWATCHER_LISTEN`       |
| Persistent data directory             | `SWATCHER_DATA`         |
| Electrs bridge endpoint               | `ELECTRUM_ADDR`         |
| Optional local Mempool interface URLs | `SWATCHER_MEMPOOL_URLS` |
| Optional Tor SOCKS endpoint           | `TOR_SOCKS_ADDR`        |

Wallets, web privacy preferences, notification rules, and UI themes are managed
through the Web UI or StartOS actions and persist under `/data`.

## Network access and interfaces

| Interface     | Internal port | Protocol | Purpose                                                    |
| ------------- | ------------: | -------- | ---------------------------------------------------------- |
| Web UI (`ui`) |        `8080` | HTTP     | Password-protected watch dashboard and transaction history |

No public application port is opened directly. StartOS exports the interface
through its own LAN/Tor routing. Electrs traffic uses the private StartOS
dependency bridge.

## Actions (StartOS UI)

All actions are visible and available from the service page.

| Action                 | ID                   | Purpose                                                                                |
| ---------------------- | -------------------- | -------------------------------------------------------------------------------------- |
| Set Web Password       | `web-password`       | Set or replace the password required by the Web UI; stores only an Argon2id verifier   |
| Notifications          | `notifications`      | Configure Telegram, NIP-17 Nostr, quiet hours, daily digest, and channel test messages |
| Privacy Mode           | `privacy-mode`       | Persistently mask balances and identifiers; disabling requires the web password        |
| Privacy Indicators     | `privacy-indicators` | Configure address-reuse, small-deposit, and combined-wallet information badges         |
| Smart Wallet Discovery | `smart-discovery`    | Set the unused-address discovery gap for extended keys and ranged descriptors          |
| Theme                  | `theme`              | Select the persistent Web UI color theme                                               |

## Backups and restore

The StartOS backup contains the complete `main` volume. Restoring it preserves
watches, derived coverage, transaction history, web authentication, privacy
settings, themes, Telegram settings, and the generated Nostr notification
identity. No other persistent path exists.

## Health checks

The primary readiness check verifies that the Web UI is listening on port
`8080`. StartOS also requires the configured Electrs service and its sync health
check before treating the dependency as ready.

## Dependencies

| Service | Required | Purpose                                                                                      |
| ------- | -------- | -------------------------------------------------------------------------------------------- |
| Electrs | Yes      | Local Bitcoin address history, balance, raw transaction, and block-header queries            |
| Mempool | No       | Private links from visible transaction IDs and unfound-address results to the local explorer |
| Tor     | No       | SOCKS transport for NIP-17 delivery to onion relays                                          |

No dependency volume is mounted. Communication uses StartOS private bridge or
interface discovery.

## Bitcoin monitoring behavior

s/watcher supports:

- Mainnet P2PKH, P2SH, SegWit, and Taproot addresses.
- Account-level `xpub`, `ypub`, and `zpub` imports.
- `pkh()`, `wpkh()`, `sh(wpkh())`, and `tr()` descriptors with non-hardened
  wildcard derivation and optional `<0;1>` receive/change branches.
- Smart discovery with a configurable gap and a bounded per-branch scan.
- Atomic bulk imports and consolidation of existing watch groups.
- Confirmed and mempool activity, exact received/sent/net values, RBF status,
  printable OP_RETURN text, and lightweight Runes/Ordinals markers.
- Complete transaction history with per-input/per-output address amounts,
  sorting, pagination, local Mempool links, copyable visible addresses, and
  privacy-aware branded text/PNG exports.
- Local address ownership lookup across every address already stored for a
  watch, including derived children.

All blockchain data comes from the StartOS-local Electrs dependency. Historical
transactions are imported without notification delivery. Mempool events are
updated when confirmed.

## Notifications

Each watch can notify on every transaction, incoming only, outgoing only, or
not at all. Policies can apply a minimum amount and wait for mempool detection
or a selected confirmation count. Quiet hours defer immediate messages; daily
digest mode combines pending activity.

Telegram uses a user-supplied bot token and recipient ID. Nostr delivery uses
NIP-17 gift wrapping, a receiver `npub`, relay discovery, and a dedicated sender
identity generated and retained under `/data`. Delivery state is persisted per
channel so restarts do not intentionally duplicate successful alerts.

## Security and privacy

- The service is watch-only and cannot spend Bitcoin.
- Private Bitcoin keys and seed phrases are rejected and must never be entered.
- Public extended keys and descriptors still reveal a wallet's transaction
  graph; protect the Web UI and backups accordingly.
- The Web UI uses an Argon2id password verifier, server-side sessions, CSRF and
  origin checks, a restrictive Content Security Policy, output escaping, and
  server-side privacy masking.
- Privacy Mode prevents masked addresses from being embedded as copyable values.
- Telegram and public Nostr relays are external services when enabled. Electrs
  blockchain queries remain local.

See [SECURITY.md](SECURITY.md) for responsible vulnerability reporting.

## Limitations and differences

1. Bitcoin mainnet only; testnet, signet, and regtest are not supported.
2. Watch-only data only; signing and spending are intentionally absent.
3. Smart discovery is bounded to protect the server from unbounded derivation
   and Electrs workloads.
4. Input addresses identify previous transaction outputs, not a proven
   real-world sender.
5. Runes and Ordinals support is detection-only; content is not decoded,
   rendered, fetched, or linked.
6. Large bulk imports and extended-key histories may take several scan cycles.
7. Telegram or non-onion Nostr delivery discloses notification traffic to the
   configured external provider or relay.

## What is unchanged from upstream

s/watcher is the upstream application as well as the StartOS package; there is
no separate third-party image or patched submodule. The StartOS TypeScript layer
adds lifecycle, dependency, interface, action, and backup integration around
the same Go application built from this repository.

## Development and contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the local validation and packaging
workflow. Security reports belong in the private process described by
[SECURITY.md](SECURITY.md), not in public issues.

The release checklist is maintained in
[COMMUNITY_SUBMISSION.md](COMMUNITY_SUBMISSION.md).

## Quick reference for AI consumers

```yaml
package_id: s-watcher
title: s/watcher
architectures:
  - x86_64
  - aarch64
volumes:
  main: /data
interfaces:
  ui:
    port: 8080
    protocol: http
dependencies:
  required:
    - electrs
  optional:
    - mempool
    - tor
startos_managed_env_vars:
  - SWATCHER_LISTEN
  - SWATCHER_DATA
  - ELECTRUM_ADDR
  - SWATCHER_MEMPOOL_URLS
  - TOR_SOCKS_ADDR
actions:
  - web-password
  - notifications
  - privacy-mode
  - privacy-indicators
  - smart-discovery
  - theme
backup_volumes:
  - main
```
