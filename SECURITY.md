# Security policy

## Supported releases

Security fixes are applied to the latest published s/watcher release. Users
should update to the newest package available from their trusted StartOS
registry.

## Reporting a vulnerability

Please report suspected vulnerabilities through
[GitHub Private Vulnerability Reporting](https://github.com/StellarStoic/swatcher/security/advisories/new).
Do not include exploit details, wallet identifiers, xpubs, descriptors,
notification credentials, or server addresses in a public issue.

A useful private report includes:

- The affected component and commit or package version.
- Reproduction steps using non-sensitive test data.
- Expected and observed behavior.
- Security impact and any suggested mitigation.

## Scope notes

s/watcher is watch-only and must never accept Bitcoin seed phrases or private
keys. Public extended keys and descriptors are not spending secrets, but they
are privacy-sensitive because they reveal wallet derivation and transaction
history. The generated Nostr sender `nsec` and Telegram bot token are
application credentials and should also be treated as secrets.

Reports about StartOS itself, Electrs, Mempool, Telegram, Nostr relays, or Tor
should be sent to those projects unless the issue is caused by s/watcher's
integration.

## Build-tool advisories

The repository records known npm audit findings and their runtime exposure in
[COMMUNITY_SUBMISSION.md](COMMUNITY_SUBMISSION.md). A tooling-only finding is
not silently presented as a clean audit, but it is distinguished from code
included in the application container or compiled StartOS JavaScript bundle.
