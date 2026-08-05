// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:24',
  releaseNotes: {
    en_US:
      'Adds editable bare-xpub address types, linked type descriptions, and each watch’s latest historical transaction with relative time.',
    de_DE:
      'Fügt bearbeitbare Adresstypen für reine xpubs, verlinkte Typbeschreibungen und die letzte historische Transaktion jeder Wallet mit relativer Zeit hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
