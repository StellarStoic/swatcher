# AGENTS.md

This repository builds `s-watcher`, a StartOS 0.4 `.s9pk` service package.

- Keep `README.md` and `instructions.md` synchronized with behavior.
- Never accept, request, log, or persist Bitcoin private keys or seed phrases.
- The notification subsystem may generate, accept, and persist only its own
  Nostr sender nsec and Telegram bot token under `/data`. These notification
  secrets must never be written to logs or exposed by the web UI.
- Bitcoin data must come from the StartOS-local Electrs dependency.
- Persistent application state belongs under `/data`.
- Run Go tests and the StartOS TypeScript check after relevant changes.
