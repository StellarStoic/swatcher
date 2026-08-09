// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { setupI18n } from '@start9labs/start-sdk'
import defaultDict, { DEFAULT_LANG } from './dictionaries/default'
import translations from './dictionaries/translations'

export const i18n = setupI18n(defaultDict, translations, DEFAULT_LANG)
