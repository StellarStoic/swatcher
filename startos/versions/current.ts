// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:21',
  releaseNotes: {
    en_US:
      'Adds configurable informational badges for address reuse, small deposits, and combined watched wallets.',
    de_DE:
      'Fügt konfigurierbare Hinweise für Adresswiederverwendung, kleine Eingänge und kombinierte beobachtete Wallets hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
