// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:40',
  releaseNotes: {
    en_US:
      'Shows locally resolved input addresses as transaction sources in watch history and refreshes existing history after upgrade.',
    de_DE:
      'Zeigt lokal aufgelöste Eingabeadressen als Transaktionsquellen im Verlauf und aktualisiert bestehende Verlaufsdaten nach dem Upgrade.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
