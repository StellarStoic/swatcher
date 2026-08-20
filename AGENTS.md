# AGENTS.md

This is a StartOS service-package repository — it builds a `.s9pk` for StartOS.

Develop it inside a StartOS packaging workspace created by `start-cli s9pk init-workspace`,
which provides the packaging guide and agent context one level up. If you're reading this in a
bare clone with no workspace, the full guide is at <https://docs.start9.com/packaging>.

**Start every task at the recipe index** — `../start-technologies/projects/start-sdk/docs/src/recipes.md`
(or <https://docs.start9.com/packaging/recipes.html>). It maps an intent ("prompt the user to create
admin credentials", "expose a web UI") to the constructs, the reference pages, and a named production
package to copy. Find the recipe before you read this package's neighbours: a package you reach by
grepping may be non-conformant, and the recipe outranks it.

Freshly scaffolded? Work the
[New Package Checklist](../start-technologies/projects/start-sdk/docs/src/new-package-checklist.md)
(or <https://docs.start9.com/packaging/new-package-checklist.html>) from top to bottom. It is a
guide page, not a file in this repo — read it, don't copy it in.

Keep `README.md` (technical reference for an AI support or administering agent) and
`instructions.md` (end-user docs) in sync with your changes.

**Bugs and feature requests are GitHub issues on this repo** — file them as you find them.
Don't record work in the repo instead: no `TODO.md`, no `NOTES.md`, no `PLAN.md`. What you
verified, tried, and decided belongs in the commit message and the PR body.

## This repo

**This repo holds the application as well as its package.** The Go source under `cmd/` and `internal/` is s/watcher itself, built by the root `Dockerfile`; `startos/` packages it. So `upstreamRepo` and `packageRepo` name different repositories on purpose — upstream is the author's, the package is the fork CI builds and releases from.

Invariants a change must not break:

- **s/watcher is watch-only.** Never accept, request, log, or persist a Bitcoin private key, WIF, extended private key, or seed phrase. Public extended keys and descriptors are accepted and are privacy-sensitive, not spending secrets.
- **Blockchain data comes from the Electrs dependency.** Never fall back to a public explorer API — the whole point of the package is that no address is disclosed to a third party.
- **`/data` holds two application secrets** — the generated Nostr sender `nsec` in `notifications.json` and the Telegram bot token. Neither may reach the logs or an unauthenticated web response.
- **The daemon starts as root only to `chown` the volume,** then drops to the unprivileged `swatcher` user with `su-exec`. Anything writing into `/data` from the service container or a temporary subcontainer runs as root and must `chown swatcher:swatcher` what it wrote, or the app cannot rewrite it.

Run the Go gate alongside the TypeScript one:

```sh
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
npm run check
```
