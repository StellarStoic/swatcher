import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:10',
  releaseNotes: {
    en_US:
      'Adds colored latest-activity signals to watched addresses and wallets.',
    de_DE:
      'Fügt farbige Aktivitätssignale für überwachte Adressen und Wallets hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
