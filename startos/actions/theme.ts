// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { stateConfig } from '../fileModels/state.json'
import { i18n } from '../i18n'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const inputSpec = InputSpec.of({
  theme: Value.select({
    name: i18n('Web Interface theme'),
    description: i18n(
      'The color swatches beside each name preview its palette.',
    ),
    default: 'bitcoin-night',
    values: {
      'bitcoin-night': i18n('🟧 ⬛ Bitcoin Night'),
      cypherpunk: i18n('🟪 🩷 Cypherpunk Neon'),
      arctic: i18n('🟦 🩵 Arctic Node'),
      forest: i18n('🟩 🟨 Forest Ledger'),
      paper: i18n('⬜ 🟧 Paper Ledger'),
    },
  }),
})
type ThemeInput = typeof inputSpec._TYPE

export const theme = sdk.Action.withInput(
  'theme',
  {
    name: i18n('Theme'),
    description: i18n(
      'Choose the persistent appearance of the s/watcher Web Interface',
    ),
    warning: null,
    allowedStatuses: 'any',
    group: 'General',
    visibility: 'enabled',
  },
  inputSpec,
  async ({ effects }) => {
    const state = await stateConfig.read().const(effects)
    return { theme: state?.theme ?? 'bitcoin-night' } satisfies ThemeInput
  },
  async ({ effects, input }) => {
    const mounts = sdk.Mounts.of().mountVolume({
      volumeId: 'main',
      subpath: null,
      mountpoint: '/data',
      readonly: false,
    })
    await sdk.SubContainer.withTemp(
      effects,
      { imageId: 's-watcher' },
      mounts,
      'set-theme',
      async (sub) => {
        await sub.execFail(
          [
            'sh',
            '-c',
            `s-watcher set-theme ${input.theme} && chown swatcher:swatcher /data/state.json`,
          ],
          { user: 'root', env: { SWATCHER_DATA: '/data' } },
        )
      },
    )
    await effects.restart()
    return {
      version: '1',
      title: i18n('Theme saved'),
      message: i18n(
        'The selected theme now applies to the s/watcher Web Interface.',
      ),
      result: null,
    }
  },
)
