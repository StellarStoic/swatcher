import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:11',
  releaseNotes: {
    en_US:
      'Moves one-shot notification tests into Notifications and fixes watch editing visibility.',
    de_DE:
      'Verschiebt einmalige Tests in die Benachrichtigungseinstellungen und korrigiert die Sichtbarkeit der Bearbeitung.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
