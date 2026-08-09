// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:3',
  releaseNotes: {
    en_US:
      'Fixes transaction address amounts remaining stuck on Refreshing locally after an upgrade.',
    de_DE:
      'Behebt, dass Transaktionsadressbeträge nach einem Upgrade dauerhaft als lokal aktualisiert angezeigt wurden.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
