// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:1',
  releaseNotes: {
    en_US:
      'Adds a private local Mempool link for inspecting addresses that are not found in saved s/watcher watches.',
    de_DE:
      'Ergänzt einen privaten lokalen Mempool-Link zur Prüfung von Adressen, die nicht in gespeicherten s/watcher-Watches gefunden wurden.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
