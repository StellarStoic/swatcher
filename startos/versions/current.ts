// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:19',
  releaseNotes: {
    en_US:
      'Adds per-wallet notification direction, amount, and confirmation rules.',
    de_DE:
      'Fügt wallet-spezifische Regeln für Richtung, Betrag und Bestätigungen von Benachrichtigungen hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
