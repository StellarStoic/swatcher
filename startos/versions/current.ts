import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:8',
  releaseNotes: {
    en_US: 'Shows notification test actions only for enabled channels.',
    de_DE: 'Zeigt Benachrichtigungstests nur für aktivierte Kanäle an.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
