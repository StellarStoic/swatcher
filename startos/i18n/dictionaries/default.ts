// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

export const DEFAULT_LANG = 'en_US'

const dict = {
  'Starting s-watcher': 0,
  'Web Interface': 1,
  'The web interface is ready': 2,
  'The web interface is not ready': 3,
  'Web UI': 4,
  'Monitor Bitcoin address activity': 5,
} as const

export type I18nKey = keyof typeof dict
export type LangDict = Record<(typeof dict)[I18nKey], string>
export default dict
