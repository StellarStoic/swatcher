// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:4',
  releaseNotes: {
    en_US:
      'Makes every visible Bitcoin address in transaction history copyable with one click.',
    de_DE:
      'Macht jede sichtbare Bitcoin-Adresse im Transaktionsverlauf mit einem Klick kopierbar.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
