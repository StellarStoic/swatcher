// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:5',
  releaseNotes: {
    en_US:
      'Prepares s/watcher for community distribution with complete StartOS package documentation, validation guidance, and repository policies.',
    de_DE:
      'Bereitet s/watcher mit vollständiger StartOS-Paketdokumentation, Prüfanleitung und Repository-Richtlinien auf die Community-Veröffentlichung vor.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
