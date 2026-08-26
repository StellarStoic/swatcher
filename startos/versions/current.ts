// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Improves long transaction lists and RBF warnings, confirms watch removal, adds a favicon, and lets bulk imports identify and strip watched addresses.',
    es_ES:
      'Mejora las listas largas y los avisos RBF, confirma eliminaciones, añade un favicon y permite identificar y excluir direcciones ya vigiladas al importar.',
    de_DE:
      'Verbessert lange Listen und RBF-Warnungen, bestätigt Löschungen, ergänzt ein Favicon und kann bereits beobachtete Adressen beim Massenimport erkennen und entfernen.',
    pl_PL:
      'Ulepsza długie listy i ostrzeżenia RBF, potwierdza usuwanie, dodaje faviconę oraz wykrywa i pomija obserwowane adresy podczas importu zbiorczego.',
    fr_FR:
      'Améliore les longues listes et alertes RBF, confirme les suppressions, ajoute un favicon et permet d’identifier et d’exclure les adresses déjà surveillées lors d’un import.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
