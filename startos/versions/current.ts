// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:2',
  releaseNotes: {
    en_US:
      'Shows the exact amount consumed from or sent to each input and output address in transaction history.',
    de_DE:
      'Zeigt im Transaktionsverlauf für jede Ein- und Ausgangsadresse den genauen verbrauchten oder gesendeten Betrag.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
