// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:17',
  releaseNotes: {
    en_US:
      'Adds configurable smart address discovery for xpubs and ranged descriptors.',
    de_DE:
      'Fügt eine konfigurierbare intelligente Adresserkennung für xpubs und Deskriptoren hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
