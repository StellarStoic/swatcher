import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:1',
  releaseNotes: {
    en_US: 'Initial address-monitoring release.',
    de_DE: 'Erste Version zur Adressüberwachung.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
