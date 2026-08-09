// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:39',
  releaseNotes: {
    en_US:
      'Adds a Cancel control to combine-selection mode that clears selected watches and restores the normal interface without refreshing or changing scroll position.',
    de_DE:
      'Ergänzt einen Abbrechen-Schalter im Kombinieren-Auswahlmodus, der die Auswahl löscht und die normale Ansicht ohne Neuladen oder Positionsverlust wiederherstellt.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
