// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { stateConfig } from '../fileModels/state.json'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const inputSpec = InputSpec.of({
  privacyMode: Value.toggle({
    name: 'Enable privacy mode',
    description:
      'Masks balances, activity amounts, wallet identifiers, transaction IDs, and the displayed Nostr sender npub in the Web Interface.',
    default: false,
  }),
  password: Value.text({
    name: 'Web password when disabling',
    description:
      'Required only when turning privacy mode off. The password is verified without being stored.',
    required: false,
    default: null,
    masked: true,
    minLength: 5,
    maxLength: 256,
  }),
})
type PrivacyModeInput = typeof inputSpec._TYPE

export const privacyMode = sdk.Action.withInput(
  'privacy-mode',
  {
    name: 'Privacy Mode',
    description: 'Control masking of sensitive values in the Web Interface',
    warning: null,
    allowedStatuses: 'any',
    group: 'General',
    visibility: 'enabled',
  },
  inputSpec,
  async ({ effects }) => {
    const state = await stateConfig.read().const(effects)
    return {
      privacyMode: state?.privacyMode ?? false,
      password: null,
    } satisfies PrivacyModeInput
  },
  async ({ effects, input }) => {
    const state = await stateConfig.read().once()
    const wasEnabled = state?.privacyMode ?? false
    if (input.privacyMode === wasEnabled) return
    if (!input.privacyMode && !input.password) {
      throw new Error('Enter the Web Password to disable Privacy Mode.')
    }
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
      'set-privacy-mode',
      async (sub) => {
        await sub.execFail(
          [
            'sh',
            '-c',
            `s-watcher set-privacy-mode ${input.privacyMode ? 'enabled' : 'disabled'} && chown swatcher:swatcher /data/state.json`,
          ],
          {
            input: input.password ?? '',
            user: 'root',
            env: { SWATCHER_DATA: '/data' },
          },
        )
      },
    )
    await effects.restart()
  },
)
