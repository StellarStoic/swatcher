// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:20',
  releaseNotes: {
    en_US:
      'Adds persistent quiet hours and optional daily notification digests.',
    de_DE:
      'Fügt dauerhafte Ruhezeiten und optionale tägliche Benachrichtigungszusammenfassungen hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
