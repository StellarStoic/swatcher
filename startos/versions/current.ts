// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:25',
  releaseNotes: {
    en_US:
      'Adds bulk address imports and manual consolidation of existing watches into fixed groups, with explicit multi-address warnings.',
    de_DE:
      'Fügt Adress-Massenimporte und die manuelle Zusammenführung bestehender Überwachungen in feste Gruppen mit ausdrücklichen Warnungen für Gruppen mit mehreren Adressen hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
