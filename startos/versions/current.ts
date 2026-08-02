import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:7',
  releaseNotes: {
    en_US:
      'Improves watch editing, group suggestions, validation, and sorting.',
    de_DE:
      'Verbessert Bearbeitung, Gruppenvorschläge, Validierung und Sortierung.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
