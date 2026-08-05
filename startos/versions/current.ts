// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:23',
  releaseNotes: {
    en_US:
      'Adds private per-wallet notes and five persistent Web Interface themes with color previews.',
    de_DE:
      'Fügt private Wallet-Notizen und fünf dauerhafte Oberflächen-Themes mit Farbvorschau hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
