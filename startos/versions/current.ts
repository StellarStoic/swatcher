// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:22',
  releaseNotes: {
    en_US:
      'Detects runestones and Ordinals inscription envelopes locally without decoding or linking their content.',
    de_DE:
      'Erkennt Runestones und Ordinals-Inskriptionshüllen lokal, ohne deren Inhalt zu dekodieren oder zu verlinken.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
