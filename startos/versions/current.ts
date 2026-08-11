// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:6',
  releaseNotes: {
    en_US:
      'Adds privacy-aware, branded transaction exports for copying readable text or saving a PNG image.',
    de_DE:
      'Fügt datenschutzgerechte, gebrandete Transaktionsexporte zum Kopieren als lesbaren Text oder Speichern als PNG-Bild hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
