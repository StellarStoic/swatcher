// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:27',
  releaseNotes: {
    en_US:
      'Fixes single-address and bulk watch submissions, improves consolidation selection, and adds optional local Mempool transaction links.',
    de_DE:
      'Behebt das Hinzufügen einzelner Adressen und Adresslisten, verbessert die Auswahl zum Zusammenführen und ergänzt optionale lokale Mempool-Transaktionslinks.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
