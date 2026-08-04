// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:16',
  releaseNotes: {
    en_US:
      'Displays human-readable OP_RETURN messages and improves the service description.',
    de_DE:
      'Zeigt lesbare OP_RETURN-Nachrichten an und verbessert die Dienstbeschreibung.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
