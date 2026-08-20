# Updating the upstream version

s/watcher's application source lives in this repository, under `cmd/` and `internal/`, and the image is built from the root `Dockerfile`. "Upstream" here is the author's repository, [StellarStoic/swatcher](https://github.com/StellarStoic/swatcher) — there is no third-party project or published image to track.

## Determining the upstream version

- Fetch the latest release tag from the author's repository:

  ```sh
  gh release view -R StellarStoic/swatcher --json tagName -q .tagName
  ```

  StartOS tags are the package version with the colon replaced by an underscore, so `0.1.1:6` is tagged `v0.1.1_6`. The upstream part is everything before the colon.

## Applying the bump

- Pull the application changes into this fork, then bump `version` in `startos/versions/current.ts` to `<upstream version>:0`. A change to the packaging alone bumps only the revision after the colon.
- Write `releaseNotes` for every locale in `startos/i18n/dictionaries/translations.ts`.
- The Go toolchain version is pinned in `go.mod` and the `Dockerfile`'s builder stage; bump both together.
