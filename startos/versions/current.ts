import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:9',
  releaseNotes: {
    en_US: 'Clarifies generated sender keys and rejects recipient nsec secrets.',
    de_DE: 'Erklärt generierte Senderschlüssel und lehnt geheime nsec-Empfängerschlüssel ab.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
