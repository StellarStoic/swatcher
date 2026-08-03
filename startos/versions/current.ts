// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:12',
  releaseNotes: {
    en_US:
      'Adds flexible watch names and groups, five-character web passwords, and AGPL v3-or-later licensing.',
    de_DE:
      'Erlaubt flexiblere Namen und Gruppen sowie Web-Passwörter ab fünf Zeichen und deklariert AGPL v3 oder neuer.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
