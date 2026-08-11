# Contributing to s/watcher

Thank you for helping improve s/watcher. Changes should preserve its core
properties: watch-only Bitcoin operation, local blockchain queries through the
StartOS Electrs dependency, and explicit handling of sensitive public wallet
data.

## Development prerequisites

- Git
- Docker with Buildx
- Go (matching `go.mod`)
- Node.js 22 or newer and npm
- `make`, `jq`, and `mksquashfs`
- `start-cli`

Use the current [StartOS packaging environment guide](https://docs.start9.com/packaging/0.4.0.x/environment-setup.html)
for installation details.

## Local checks

Install JavaScript dependencies:

```sh
npm ci
```

Run the application tests and static checks:

```sh
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
npm run check
node node_modules/@start9labs/start-sdk/lint.mjs
npm run build
```

Verify both declared container architectures:

```sh
docker buildx build --platform linux/amd64,linux/arm64 .
```

Build the StartOS package from a configured packaging workspace:

```sh
start-cli s9pk pack --icon icon.svg
```

Before submitting a release, install the resulting package on StartOS and
complete the smoke test in [COMMUNITY_SUBMISSION.md](COMMUNITY_SUBMISSION.md).

## Pull requests

1. Create a focused branch.
2. Add tests for behavior changes.
3. Update `README.md` for packaging/runtime changes and `instructions.md` for
   user-visible changes.
4. Bump the downstream revision in `startos/versions/current.ts` and write
   user-facing release notes when the packaged output changes.
5. Run every relevant check above.
6. Explain the user impact, privacy impact, and validation in the pull request.

Do not commit `.s9pk` files, build output, local state, credentials, private
keys, seed phrases, Telegram tokens, or generated Nostr sender keys.

## Release tags

StartOS uses the package version as a git tag with the colon replaced by an
underscore:

```text
package version: X.Y.Z:N
git tag:         vX.Y.Z_N
```

Push only the intended release tag, not every local tag.

## Security reports

Do not disclose a suspected vulnerability in a public issue. Follow
[SECURITY.md](SECURITY.md).
