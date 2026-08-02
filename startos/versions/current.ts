import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:5',
  releaseNotes: {
    en_US:
      'Adds mandatory web-password authentication and display privacy mode.',
    de_DE:
      'Fügt eine verpflichtende Web-Passwort-Authentifizierung und einen Anzeigedatenschutzmodus hinzu.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
