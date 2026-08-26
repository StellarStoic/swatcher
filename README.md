<p align="center">
  <img src="icon.png" alt="s/watcher Logo" width="21%">
</p>

# s/watcher on StartOS

> Everything not listed in this document should behave the same as upstream
> s/watcher. If a feature, setting, or behavior is not mentioned here, the
> upstream documentation is accurate and fully applicable — see the
> Documentation section of `instructions.md` for links.

[s/watcher](https://github.com/StellarStoic/swatcher) is a watch-only Bitcoin activity monitor: it tracks addresses, extended public keys, and output descriptors, and reports incoming and outgoing transactions. Every blockchain query is answered by the Electrs instance on this server, so no watched address is disclosed to a third-party explorer. Optional alerts go out over Telegram or NIP-17 encrypted Nostr messages.

This repository holds the application as well as its package — the Go source under `cmd/` and `internal/` is s/watcher itself. `upstreamRepo` therefore names the author's repository while `packageRepo` names the fork CI builds from.

- **Upstream repo:** <https://github.com/StellarStoic/swatcher>
- **Wrapper repo:** <https://github.com/Start9-Community/swatcher>

---

## Table of Contents

- [Image and Container Runtime](#image-and-container-runtime)
- [Volume and Data Layout](#volume-and-data-layout)
- [File Models](#file-models)
- [Dependencies](#dependencies)
- [Network Access and Interfaces](#network-access-and-interfaces)
- [Installation and First-Run Flow](#installation-and-first-run-flow)
- [Actions](#actions)
- [Tasks](#tasks)
- [Health Checks](#health-checks)
- [Backups and Restore](#backups-and-restore)
- [Limitations and Differences](#limitations-and-differences)
- [Quick Reference for AI Consumers](#quick-reference-for-ai-consumers)

---

## Image and Container Runtime

Built from this repository's own `Dockerfile`: a Go builder stage compiles a static binary, which is copied into an Alpine runtime carrying `su-exec`.

| Property      | Value                                             |
| ------------- | ------------------------------------------------- |
| Image         | Built here — no third-party or upstream image     |
| Architectures | x86_64, aarch64                                   |
| Entrypoint    | Replaced (see below)                              |
| User          | Starts as `root`, runs as `swatcher`              |

| Subcontainer | Purpose                                       |
| ------------ | --------------------------------------------- |
| `s-watcher`  | The `primary` daemon — the one to `attach` to |

The daemon replaces the image entrypoint with `sh -c 'chown swatcher:swatcher /data && exec su-exec swatcher s-watcher'`. StartOS presents the volume owned by root, so the container takes root just long enough to hand `/data` to the unprivileged user and then drops to it for the life of the process. **Anything else that writes into `/data` — an action, a temporary subcontainer — also runs as root and must chown what it wrote**, or the daemon cannot rewrite that file afterwards.

The package supplies the runtime's configuration entirely through environment variables:

| Variable                | Supplies                                                              |
| ----------------------- | --------------------------------------------------------------------- |
| `SWATCHER_LISTEN`       | The web listen address                                                |
| `SWATCHER_DATA`         | The volume mount point                                                |
| `ELECTRUM_ADDR`         | The Electrs bridge address, resolved at start                         |
| `SWATCHER_MEMPOOL_URLS` | JSON array of local Mempool addresses, empty when Mempool is absent   |
| `TOR_SOCKS_ADDR`        | The Tor SOCKS bridge address, empty when Tor is absent                |

Each is read once per launch, so a change to any of them takes effect on the next start. The daemon re-runs whenever `notifications.json` changes, which is what picks up new notification settings.

## Volume and Data Layout

One volume holds everything, and the application is its primary writer.

| Volume | Mount Point | Purpose                                          |
| ------ | ----------- | ------------------------------------------------ |
| `main` | `/data`     | Watches, history, credentials, and all settings  |

| Path                       | Contents                                                                       |
| -------------------------- | ------------------------------------------------------------------------------ |
| `/data/state.json`         | Watches, derived addresses, observed activity, delivery state, and UI settings  |
| `/data/auth.json`          | The Argon2id password verifier and the session-invalidation counter             |
| `/data/notifications.json` | Telegram credentials and the generated Nostr sender identity                    |

The package keeps no `store.json`; there is no StartOS-side state outside the volume.

## File Models

Two models, and the ownership split between them is the thing to understand.

| Model                | File                 | Format | Written by the package     |
| -------------------- | -------------------- | ------ | -------------------------- |
| `stateConfig`        | `state.json`         | JSON   | No — read only             |
| `notificationConfig` | `notifications.json` | JSON   | Yes — by the Notifications action |

**`state.json` is the application's file.** It holds the watch list and every transaction s/watcher has observed, and the daemon rewrites it continuously. The package declares only the eight settings its actions expose and never writes the file: Privacy Mode, Privacy Indicators, Smart Wallet Discovery, and Theme each shell out to the application's own `s-watcher set-…` subcommand, which reads, edits, and rewrites the file, and then chown the result back to the service user. Undeclared keys are preserved because the model is read-only, so no packaging change can drop a watch. A hand edit does not survive: the daemon rewrites the file on its next scan.

**`notifications.json` is the package's file, jointly.** The Notifications action writes it wholesale from the submitted form, so every key it declares is re-asserted on each save and a hand edit to any of them is lost at the next save. The application writes the same file back in two cases: to record the Nostr sender keypair it derives when Nostr is first enabled, and to mark the sender profile as published once a relay has accepted it. Nothing seeds it — until Notifications is saved for the first time, the file does not exist and the model reads as `null`.

Neither model is written during install or start, so a restored volume comes back exactly as it was backed up.

## Dependencies

One required, two optional — and the optional two are declared to StartOS only once they are actually in use.

| Service | Required | Health checks       | Purpose                                                                    |
| ------- | -------- | ------------------- | -------------------------------------------------------------------------- |
| Electrs | Yes      | `electrs`, `sync`   | Every address history, balance, transaction, and block-header query        |
| Mempool | No       | `webui`             | Private explorer links from transaction IDs and address lookups            |
| Tor     | No       | `tor`               | SOCKS transport for NIP-17 delivery to `.onion` relays                     |

StartOS warns the user whenever a declared dependency is missing or stopped, which is the wrong prompt for a feature nobody opted into. So Mempool is declared only while it is installed, and Tor only while Nostr notifications are enabled **and** at least one configured relay is an `.onion` address — the only case in which the application routes anything through the proxy. Clearnet Telegram and Nostr traffic goes direct whether or not Tor is present.

No dependency volume is mounted. Electrs and Tor are reached over the StartOS bridge; Mempool addresses are read from its exported interface and handed to the browser, never dialed by this service.

## Network Access and Interfaces

One interface, carrying both the dashboard and the HTTP endpoints behind it.

| Interface | Id   | Type | Port | Description                     |
| --------- | ---- | ---- | ---- | ------------------------------- |
| Web UI    | `ui` | ui   | 8080 | Watch dashboard and history     |

The port is bound on the `ui` MultiHost over plain HTTP and is not masked. Nothing else is exposed: the service opens no peer port, and its outbound traffic is Electrs over the bridge plus, when notifications are configured, Telegram and Nostr relays.

## Installation and First-Run Flow

Install Electrs first and let it finish syncing — s/watcher answers every query from it and can do nothing useful before it is ready.

Nothing is seeded at install and no credential is generated. **A new installation has no web password**, so run **Set Web Password** before opening the interface. There is no task enforcing this, and no default password to change.

From there the whole flow is inside the Web UI: add a watch, and the first Electrs scan establishes the baseline. That first scan deliberately imports existing history **without** sending notifications for it — otherwise a wallet with years of transactions would deliver years of alerts on the day it was added.

Transaction history is paginated at 100 transactions per page. Within each transaction card, address lists longer than 10 rows are collapsed to the first 10 and can be expanded on demand; text exports still include the complete list.

Removing a saved watch requires a second, named Yes/No confirmation so an accidental click cannot immediately delete it.

## Actions

Six actions, all user-facing. Four of them write a setting through the application's own CLI and then restart the service; none of them is destructive, and every one is safe to repeat.

### `web-password` — Set Web Password

- **When to run it:** before first use, and any time the password should change or has been forgotten. There is no recovery flow and none is needed — setting a new password does not require the old one.
- **What it changes:** `auth.json`, replacing the Argon2id verifier and invalidating existing sessions.
- **Cost:** a few seconds in a temporary container. The service is not restarted and keeps serving.
- **Repeat safety:** idempotent. Each run replaces the verifier and signs out open browsers.
- **Outputs:** none — the password is what the caller supplied.

### `notifications` — Notifications

- **When to run it:** to turn Telegram or Nostr alerts on, to change delivery scheduling (quiet hours, daily digest, UTC offset), or to read back the Nostr sender keys s/watcher generated.
- **What it changes:** `notifications.json`, wholesale. Enabling Nostr for the first time also derives a sender keypair and a unique sender name, both stored there and both retained if Nostr is later switched off.
- **Cost:** instant unless a test message is requested, which spends a few seconds in a temporary container per channel.
- **Repeat safety:** idempotent. The sender keypair is generated once and reused; re-saving never rotates it.
- **What happens next:** the daemon re-runs on the file change, so new settings apply without an explicit restart. **Reopen the action to see the generated `npub` and `nsec`** — they are written by the save, so the form that submitted it cannot show them.
- **Outputs:** the generated sender `npub` and (masked) `nsec`, on the next open.

### `privacy-mode` — Privacy Mode

- **When to run it:** to mask balances, amounts, wallet identifiers, transaction IDs, and the Nostr sender `npub` in the Web UI — before a screen share, or as a standing setting.
- **What it changes:** one key in `state.json`. **Turning it off requires the web password**; turning it on does not.
- **Cost:** a few seconds, then a service restart.
- **Repeat safety:** idempotent, and submitting the value it already holds is a no-op that skips the restart.
- **Outputs:** none.

Masking is a Web UI behavior only. Notification messages still carry full amounts and identifiers.

### `privacy-indicators` — Privacy Indicators

- **When to run it:** to turn the informational address-reuse, small-deposit, and combined-wallet badges on or off, or to move the small-deposit threshold.
- **What it changes:** four keys in `state.json`. The badges are advisory labels on transactions already displayed; nothing is recomputed or discarded.
- **Cost:** a few seconds, then a service restart.
- **Repeat safety:** idempotent.
- **Outputs:** none.

### `smart-discovery` — Smart Wallet Discovery

- **When to run it:** when an extended key or ranged descriptor is missing addresses — a wallet that skipped indexes needs a larger gap than the default.
- **What it changes:** the discovery gap in `state.json`. **Lowering it never deletes addresses already derived**; it only stops new derivation beyond the new limit.
- **Cost:** a few seconds and a restart, but the consequence is ongoing: every extra index is more Electrs queries on every scan. Per-branch derivation is capped at 500 addresses regardless.
- **Repeat safety:** idempotent. Newly derived historical addresses are baselined without firing notifications, the same as a new watch.
- **Outputs:** none.

### `theme` — Theme

- **When to run it:** to change the Web UI palette.
- **What it changes:** one key in `state.json`.
- **Cost:** a few seconds, then a service restart.
- **Repeat safety:** idempotent.
- **Outputs:** none.

## Tasks

None. This package creates no tasks on itself and none on its dependencies, so the service is never held on a prompt and its ordinary controls are always available.

Note that the first-run password is therefore **not** enforced by a task — see [Installation and First-Run Flow](#installation-and-first-run-flow).

## Health Checks

One check, on the only daemon.

| Check     | Displayed       | Method                 |
| --------- | --------------- | ---------------------- |
| `primary` | "Web Interface" | Port 8080 is listening |

The check covers the web server only. It says nothing about Electrs: a green check with Electrs stopped or unsynced means the dashboard loads and no watch updates. Diagnose that from the Electrs dependency warning and the service logs, not from this check.

A failure means the process is not serving. The startup path that can fail before the listener binds is the volume `chown`, so check the logs for a permission error before suspecting the application.

## Backups and Restore

The `main` volume is copied wholesale — `sdk.Backups.ofVolumes('main')`. Nothing is dumped and nothing is excluded, so a restore returns watches, derived addresses, observed history, the password verifier, notification credentials, and the generated Nostr sender identity exactly as they were.

Nothing needs rebuilding on restore, but Electrs does need to be present and synced before the restored watches update again.

**The backup is as sensitive as the service.** It contains the Telegram bot token and the Nostr sender `nsec`, and it contains every extended public key and descriptor being watched — which reveals the transaction graph of those wallets even though none of it can spend.

## Limitations and Differences

1. **Bitcoin mainnet only.** Testnet, signet, and regtest are not supported.
2. **Watch-only, by design.** No transaction construction, signing, or spending, and no private key, WIF, extended private key, or seed phrase is ever accepted.
3. **Hardened derivation below an extended key is impossible** and is rejected. Descriptors need a public extended key and a non-hardened wildcard path.
4. **Address discovery is bounded** at 500 addresses per branch, to keep an extended key from generating unbounded Electrs load.
5. **Runes and Ordinals are detected only.** No content is decoded, rendered, fetched, or linked.
6. **Input addresses are not sender identity.** They identify the outputs a transaction consumed, which is transaction-level evidence and nothing more.
7. **Telegram, and Nostr over clearnet relays, disclose notification traffic** to that provider or relay. Electrs queries stay local either way.
8. **The generated Nostr profile references an avatar on `api.dicebear.com`.** The image is not fetched by this service, but it is published in the sender's profile metadata, so anyone viewing that profile requests it from a third party.
9. **riscv64 is not built.**

---

## Quick Reference for AI Consumers

```yaml
package_id: s-watcher
image: built-from-repo # no third-party image
architectures:
  - x86_64
  - aarch64
subcontainers:
  - s-watcher # the only container
volumes:
  main: /data
file_models:
  - state.json # application-owned; package reads only
  - notifications.json # written by the notifications action and by the app
startos_managed_env_vars:
  - SWATCHER_LISTEN
  - SWATCHER_DATA
  - ELECTRUM_ADDR
  - SWATCHER_MEMPOOL_URLS
  - TOR_SOCKS_ADDR
dependencies:
  - electrs # required
  - mempool # optional, declared only while installed
  - tor # optional, declared only for .onion relays
interfaces:
  ui: { type: ui, port: 8080 }
actions:
  - web-password
  - notifications
  - privacy-mode
  - privacy-indicators
  - smart-discovery
  - theme
tasks: []
health_checks:
  - primary # displayed "Web Interface"
```
