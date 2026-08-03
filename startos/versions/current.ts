// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:14',
  releaseNotes: {
    en_US:
      'Adds automatic, readable, and consistent colors to wallet-name and group tags.',
    de_DE:
      'Fügt Wallet-Namen und Gruppen-Tags automatische, gut lesbare und einheitliche Farben hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
