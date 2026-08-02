import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:6',
  releaseNotes: {
    en_US: 'Adds StartOS-based forgotten-password recovery guidance.',
    de_DE: 'Fügt Hinweise zur Passwortwiederherstellung über StartOS hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
